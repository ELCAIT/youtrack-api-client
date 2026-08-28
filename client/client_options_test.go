package youtrack

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientValidatesHostAndToken(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		token   string
		wantErr bool
	}{
		{name: "valid https host", host: "https://youtrack.example.com", token: "perm:abc"},
		{name: "valid http host", host: "http://localhost:8080", token: "perm:abc"},
		{name: "trailing slash accepted", host: "https://youtrack.example.com/", token: "perm:abc"},
		{name: "empty host rejected", host: "", token: "perm:abc", wantErr: true},
		{name: "blank host rejected", host: "   ", token: "perm:abc", wantErr: true},
		{name: "relative host rejected", host: "youtrack.example.com", token: "perm:abc", wantErr: true},
		{name: "non-http scheme rejected", host: "ftp://youtrack.example.com", token: "perm:abc", wantErr: true},
		{name: "scheme without hostname rejected", host: "https://", token: "perm:abc", wantErr: true},
		{name: "empty token rejected", host: "https://youtrack.example.com", token: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.host, tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for host %q token %q", tt.host, tt.token)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c == nil {
				t.Fatal("expected a client")
			}
		})
	}
}

func TestNewClientNormalizesTrailingSlash(t *testing.T) {
	c, err := NewClient("https://youtrack.example.com///", "perm:abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.HostURL != "https://youtrack.example.com" {
		t.Fatalf("host not normalized, got %q", c.HostURL)
	}
}

func TestClientSendsAuthAndUserAgent(t *testing.T) {
	var gotAuth, gotAgent, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(headerAuthorization)
		gotAgent = r.Header.Get(headerUserAgent)
		gotContentType = r.Header.Get(headerContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "perm:abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), httpMethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := c.doRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer perm:abc" {
		t.Errorf("authorization header = %q", gotAuth)
	}
	if gotAgent != defaultUserAgent {
		t.Errorf("user-agent = %q, want %q", gotAgent, defaultUserAgent)
	}
	if gotContentType != contentTypeJSON {
		t.Errorf("content-type = %q", gotContentType)
	}
}

func TestWithUserAgentOverrides(t *testing.T) {
	var gotAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAgent = r.Header.Get(headerUserAgent)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "perm:abc", WithUserAgent("my-operator/2.1.0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), httpMethodGet, srv.URL, nil)
	if _, err := c.doRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAgent != "my-operator/2.1.0" {
		t.Errorf("user-agent = %q", gotAgent)
	}
}

func TestWithHTTPClientAndTimeout(t *testing.T) {
	custom := &http.Client{Timeout: 3 * time.Second}

	c, err := NewClient("https://youtrack.example.com", "perm:abc", WithHTTPClient(custom))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HTTPClient != custom {
		t.Fatal("WithHTTPClient did not replace the HTTP client")
	}

	c2, err := NewClient("https://youtrack.example.com", "perm:abc", WithTimeout(42*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c2.HTTPClient.Timeout != 42*time.Second {
		t.Errorf("timeout = %v", c2.HTTPClient.Timeout)
	}
}

func TestNilOptionsAreIgnored(t *testing.T) {
	c, err := NewClient("https://youtrack.example.com", "perm:abc",
		WithHTTPClient(nil), WithUserAgent(""), WithLogger(nil), WithTimeout(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTP client was cleared by a nil option")
	}
	if c.userAgent != defaultUserAgent {
		t.Errorf("user agent was cleared, got %q", c.userAgent)
	}
	if c.HTTPClient.Timeout != defaultHTTPTimeout {
		t.Errorf("timeout was cleared, got %v", c.HTTPClient.Timeout)
	}
}

func TestWithLoggerDoesNotLeakToken(t *testing.T) {
	var buf safeBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"secret":"top-secret-body"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "perm:super-secret-token", WithLogger(logger))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), httpMethodGet, srv.URL, nil)
	if _, err := c.doRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logged := buf.String()
	if logged == "" {
		t.Fatal("expected the logger to record the request")
	}
	if contains(logged, "super-secret-token") {
		t.Error("the token leaked into the log output")
	}
	if contains(logged, "top-secret-body") {
		t.Error("the response body leaked into the log output")
	}
}

func TestDoRequestReturnsHTTPErrorWithParsedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(headerRetryAfter, "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate_limited","error_description":"slow down"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "perm:abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), httpMethodGet, srv.URL, nil)
	_, err = c.doRequest(req)
	if err == nil {
		t.Fatal("expected an error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected an *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d", httpErr.StatusCode)
	}
	if httpErr.ErrorCode != "rate_limited" {
		t.Errorf("error code = %q", httpErr.ErrorCode)
	}
	if httpErr.ErrorDescription != "slow down" {
		t.Errorf("error description = %q", httpErr.ErrorDescription)
	}
	if httpErr.RetryAfter != 7*time.Second {
		t.Errorf("retry-after = %v", httpErr.RetryAfter)
	}
	if got, ok := RetryAfter(err); !ok || got != 7*time.Second {
		t.Errorf("RetryAfter() = %v, %v", got, ok)
	}
	if !IsRateLimited(err) || !IsRetryable(err) {
		t.Error("a 429 should be both rate-limited and retryable")
	}
}

func TestNonJSONErrorBodyStillCarriesRawBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<html>gateway down</html>`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "perm:abc")
	req, _ := http.NewRequestWithContext(context.Background(), httpMethodGet, srv.URL, nil)
	_, err := c.doRequest(req)

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected an *HTTPError, got %T", err)
	}
	if string(httpErr.Body) != `<html>gateway down</html>` {
		t.Errorf("raw body lost: %q", httpErr.Body)
	}
	if httpErr.ErrorCode != "" {
		t.Errorf("expected no parsed error code, got %q", httpErr.ErrorCode)
	}
	if !IsRetryable(err) {
		t.Error("502 should be retryable")
	}
}
