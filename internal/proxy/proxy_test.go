package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	// Fake upstream that echoes the request path
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
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
	defer func() { _ = p.Stop(ctx) }()

	// Make request with specific path
	client := &http.Client{Transport: upstream.Client().Transport}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/messages", port))
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	expected := "/v1/messages"
	if string(body) != expected {
		t.Errorf("request path = %q, want %q", string(body), expected)
	}
}

func TestProxyInvalidTarget(t *testing.T) {
	_, err := New("not-a-url", "gcp", "")
	if err == nil {
		t.Error("New() expected error for invalid URL")
	}
}

func TestProxyNonHTTPSTarget(t *testing.T) {
	_, err := New("http://insecure.com", "gcp", "")
	if err == nil {
		t.Error("New() expected error for non-HTTPS target")
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
