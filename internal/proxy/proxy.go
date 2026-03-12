package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// Proxy is a reverse HTTP proxy that injects authentication headers.
type Proxy struct {
	targetURL  *url.URL
	authMode   string // "gcp" or "api-key"
	apiKey     string // for api-key mode
	tokenSrc   oauth2.TokenSource // for gcp mode, injectable for testing
	transport  http.RoundTripper   // custom transport (for testing TLS)
	listener   net.Listener
	server     *http.Server
}

// Option is a functional option for Proxy.
type Option func(*Proxy)

// WithTokenSource injects a custom OAuth2 token source (for testing).
func WithTokenSource(ts oauth2.TokenSource) Option {
	return func(p *Proxy) {
		p.tokenSrc = ts
	}
}

// WithTransport injects a custom HTTP transport (for testing).
func WithTransport(t http.RoundTripper) Option {
	return func(p *Proxy) {
		p.transport = t
	}
}

// New creates a new Proxy. The target must be a full URL (https://...).
// For authMode "gcp", uses Application Default Credentials.
// For authMode "api-key", apiKey must be provided.
func New(target, authMode, apiKey string, opts ...Option) (*Proxy, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	if targetURL.Scheme != "https" {
		return nil, fmt.Errorf("target must use https, got %s", targetURL.Scheme)
	}

	p := &Proxy{
		targetURL: targetURL,
		authMode:  authMode,
		apiKey:    apiKey,
	}

	for _, opt := range opts {
		opt(p)
	}

	// Initialize GCP token source if needed and not injected
	if authMode == "gcp" && p.tokenSrc == nil {
		ts, err := google.DefaultTokenSource(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("initializing GCP credentials: %w", err)
		}
		p.tokenSrc = ts
	}

	return p, nil
}

// Start starts the proxy server on localhost:0 and returns the local port.
func (p *Proxy) Start(ctx context.Context) (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listening on localhost: %w", err)
	}
	p.listener = listener

	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Rewrite request to target, preserving any path prefix
			req.URL.Scheme = p.targetURL.Scheme
			req.URL.Host = p.targetURL.Host
			req.Host = p.targetURL.Host
			if p.targetURL.Path != "" {
				req.URL.Path = singleJoiningSlash(p.targetURL.Path, req.URL.Path)
			}

			// Inject authentication header
			if p.authMode == "gcp" && p.tokenSrc != nil {
				token, err := p.tokenSrc.Token()
				if err == nil {
					req.Header.Set("Authorization", "Bearer "+token.AccessToken)
				}
			} else if p.authMode == "api-key" {
				req.Header.Set("x-api-key", p.apiKey)
			}
		},
	}

	// Use custom transport if provided (for testing)
	if p.transport != nil {
		proxy.Transport = p.transport
	}

	p.server = &http.Server{
		Handler: proxy,
	}

	go func() {
		_ = p.server.Serve(p.listener)
	}()

	port := p.listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

// Stop gracefully stops the proxy server.
func (p *Proxy) Stop(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}
