package proxy

import (
	"context"
	"fmt"
	"log"
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

// Proxy is a reverse HTTP proxy that optionally injects authentication headers.
type Proxy struct {
	targetURL *url.URL
	authMode  string             // "gcp", "api-key", or "" (no auth)
	apiKey    string             // for api-key mode
	tokenSrc  oauth2.TokenSource // for gcp mode, injectable for testing
	transport http.RoundTripper  // custom base transport (for testing TLS)
	listener  net.Listener
	server    *http.Server
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
// authMode can be "gcp", "api-key", or "" for no authentication.
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

	// Build the transport chain: auth injection wraps the base transport.
	// This surfaces token errors as 502s instead of silently forwarding
	// unauthenticated requests.
	base := p.transport
	if base == nil {
		base = http.DefaultTransport
	}
	transport := &authTransport{
		base:     base,
		authMode: p.authMode,
		apiKey:   p.apiKey,
		tokenSrc: p.tokenSrc,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = p.targetURL.Scheme
			req.URL.Host = p.targetURL.Host
			req.Host = p.targetURL.Host
			if p.targetURL.Path != "" {
				req.URL.Path = singleJoiningSlash(p.targetURL.Path, req.URL.Path)
			}
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %v", err)
			http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		},
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

// authTransport injects authentication headers into outgoing requests.
// Token errors are returned as transport errors, causing the reverse proxy
// to respond with 502 Bad Gateway.
type authTransport struct {
	base     http.RoundTripper
	authMode string
	apiKey   string
	tokenSrc oauth2.TokenSource
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch t.authMode {
	case "gcp":
		token, err := t.tokenSrc.Token()
		if err != nil {
			return nil, fmt.Errorf("fetching GCP token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	case "api-key":
		req.Header.Set("x-api-key", t.apiKey)
	}
	return t.base.RoundTrip(req)
}
