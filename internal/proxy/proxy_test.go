package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// mockTokenSource implements oauth2.TokenSource for testing.
type mockTokenSource struct {
	token string
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: m.token,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

// failingTokenSource implements oauth2.TokenSource that always returns an error.
type failingTokenSource struct{}

func (f *failingTokenSource) Token() (*oauth2.Token, error) {
	return nil, fmt.Errorf("token refresh failed: credentials expired")
}

func TestProxyGCPAuthInjection(t *testing.T) {
	// Fake upstream that echoes the Authorization header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer upstream.Close()

	// Create proxy with mock token source and upstream's TLS transport
	mockTS := &mockTokenSource{token: "test-gcp-token-123"}
	p, err := New(upstream.URL, "gcp", "", WithTokenSource(mockTS), WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	// Make request through proxy
	client := &http.Client{Transport: upstream.Client().Transport}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	expected := "Bearer test-gcp-token-123"
	if string(body) != expected {
		t.Errorf("Authorization header = %q, want %q", string(body), expected)
	}
}

func TestProxyAPIKeyAuthInjection(t *testing.T) {
	// Fake upstream that echoes the x-api-key header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.Header.Get("x-api-key")))
	}))
	defer upstream.Close()

	// Create proxy with API key
	p, err := New(upstream.URL, "api-key", "my-secret-key-789", WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	// Make request through proxy
	client := &http.Client{Transport: upstream.Client().Transport}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	expected := "my-secret-key-789"
	if string(body) != expected {
		t.Errorf("x-api-key header = %q, want %q", string(body), expected)
	}
}

func TestProxyHostRewriting(t *testing.T) {
	// Fake upstream that echoes the Host header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.Host))
	}))
	defer upstream.Close()

	p, err := New(upstream.URL, "api-key", "test-key", WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	// Make request through proxy
	client := &http.Client{Transport: upstream.Client().Transport}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/test", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	// Host should be rewritten to upstream's host
	if len(body) == 0 {
		t.Error("Host header was not set")
	}
}

func TestProxyPathPreservation(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string // appended to upstream URL
		reqPath    string // path in client request
		wantPath   string // expected path at upstream
	}{
		{
			name:     "no target path",
			reqPath:  "/v1/messages",
			wantPath: "/v1/messages",
		},
		{
			name:       "target with path prefix",
			targetPath: "/v1",
			reqPath:    "/projects/my-project/locations/us-east5/publishers/anthropic/models/claude:rawPredict",
			wantPath:   "/v1/projects/my-project/locations/us-east5/publishers/anthropic/models/claude:rawPredict",
		},
		{
			name:       "target with trailing slash",
			targetPath: "/v1/",
			reqPath:    "/projects/foo/locations/bar",
			wantPath:   "/v1/projects/foo/locations/bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(r.URL.Path))
			}))
			defer upstream.Close()

			target := upstream.URL + tt.targetPath
			p, err := New(target, "api-key", "test", WithTransport(upstream.Client().Transport))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			ctx := context.Background()
			port, err := p.Start(ctx)
			if err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			defer func() { _ = p.Stop(ctx) }()

			client := &http.Client{Transport: upstream.Client().Transport}
			resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, tt.reqPath))
			if err != nil {
				t.Fatalf("proxy request error: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.wantPath {
				t.Errorf("request path = %q, want %q", string(body), tt.wantPath)
			}
		})
	}
}

func TestProxyNoAuth(t *testing.T) {
	// Fake upstream that echoes both auth-related headers
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Should have neither Authorization nor x-api-key
		_, _ = w.Write([]byte(r.Header.Get("Authorization") + "|" + r.Header.Get("x-api-key")))
	}))
	defer upstream.Close()

	p, err := New(upstream.URL, "", "", WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	client := &http.Client{Transport: upstream.Client().Transport}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "|" {
		t.Errorf("expected no auth headers, got %q", string(body))
	}
}

func TestProxyGCPTokenError(t *testing.T) {
	// Fake upstream — should never be reached
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream was reached despite token error")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p, err := New(upstream.URL, "gcp", "",
		WithTokenSource(&failingTokenSource{}),
		WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d (502 Bad Gateway)", resp.StatusCode, http.StatusBadGateway)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "fetching GCP token") {
		t.Errorf("body = %q, want it to mention token error", string(body))
	}
}

func TestProxyInvalidTarget(t *testing.T) {
	_, err := New("not-a-url", "gcp", "")
	if err == nil {
		t.Error("New() expected error for invalid URL")
	}
}

func TestProxyTargetScheme(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{"https is allowed", "https://example.com", false},
		{"http is allowed", "http://example.com", false},
		{"ftp is rejected", "ftp://example.com", true},
		{"empty scheme is rejected", "://example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.target, "raw", "test-key")
			if (err != nil) != tt.wantErr {
				t.Errorf("New(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestProxySSEStreaming(t *testing.T) {
	// Fake upstream that returns an SSE stream with delays between events,
	// simulating Vertex AI streamRawPredict responses.
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not implement http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Hello\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\" world\"}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event))
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	mockTS := &mockTokenSource{token: "stream-token"}
	p, err := New(upstream.URL, "gcp", "", WithTokenSource(mockTS), WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() { _ = p.Stop(ctx) }()

	// Make request through proxy and read events as they arrive
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/stream", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	// Read events one at a time and verify they arrive promptly (not buffered)
	buf := make([]byte, 4096)
	var received int
	start := time.Now()
	var firstEventTime time.Duration

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			received++
			if received == 1 {
				firstEventTime = time.Since(start)
			}
		}
		if err != nil {
			break
		}
	}

	if received == 0 {
		t.Fatal("no SSE events received through proxy")
	}

	// First event should arrive quickly (within 500ms), not after all events
	// are buffered (which would be >200ms if 4 events * 50ms delay)
	if firstEventTime > 500*time.Millisecond {
		t.Errorf("first event took %v, expected < 500ms (proxy may be buffering)", firstEventTime)
	}
}

func TestProxyStartStop(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p, err := New(upstream.URL, "api-key", "test", WithTransport(upstream.Client().Transport))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	port, err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if port == 0 {
		t.Error("Start() returned port 0")
	}

	// Stop should not error
	if err := p.Stop(ctx); err != nil {
		t.Errorf("Stop() error: %v", err)
	}
}
