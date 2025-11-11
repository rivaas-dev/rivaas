// Package app provides the main application implementation for Rivaas.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

// TestOption configures test execution behavior.
type TestOption func(*testConfig)

type testConfig struct {
	timeout time.Duration
	trace   bool
	ctx     context.Context
}

// WithTimeout sets the test request timeout.
// Use -1 for no timeout.
//
// Example:
//
//	resp, err := app.Test(req, WithTimeout(5*time.Second))
func WithTimeout(d time.Duration) TestOption {
	return func(cfg *testConfig) {
		cfg.timeout = d
	}
}

// WithTrace enables tracing capture for the test request.
// This allows assertions on trace spans and metrics.
func WithTrace() TestOption {
	return func(cfg *testConfig) {
		cfg.trace = true
	}
}

// WithContext uses the provided context for the test request.
// Useful for testing context propagation and cancellation.
func WithContext(ctx context.Context) TestOption {
	return func(cfg *testConfig) {
		cfg.ctx = ctx
	}
}

// Test executes an HTTP request against the app without starting a server.
// This is useful for unit testing handlers and middleware.
//
// The request is executed synchronously with optional timeout via context.
// Returns an *http.Response that can be inspected for status, headers, and body.
//
// Example:
//
//	req := httptest.NewRequest("GET", "/users/123", nil)
//	resp, err := app.Test(req)
//	if err != nil {
//	    t.Fatal(err)
//	}
//	assert.Equal(t, 200, resp.StatusCode)
func (a *App) Test(req *http.Request, opts ...TestOption) (*http.Response, error) {
	cfg := &testConfig{
		timeout: 1 * time.Second, // default timeout
		ctx:     context.Background(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Apply timeout via context if specified
	ctx := cfg.ctx
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	} else if cfg.timeout < 0 {
		// -1 means no timeout
		ctx = context.Background()
	}

	req = req.WithContext(ctx)

	// Create response recorder
	recorder := httptest.NewRecorder()

	// Execute handler (respects context cancellation)
	done := make(chan struct{})
	var handlerErr error
	go func() {
		a.router.ServeHTTP(recorder, req)
		close(done)
	}()

	select {
	case <-done:
		// Request completed
		return recorder.Result(), handlerErr
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	}
}

// TestJSON is a convenience method for testing JSON requests.
// It automatically sets Content-Type and encodes the body as JSON.
//
// Example:
//
//	body := map[string]string{"name": "Alice"}
//	resp, err := app.TestJSON("POST", "/users", body)
func (a *App) TestJSON(method, path string, body any, opts ...TestOption) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("failed to encode JSON body: %w", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	return a.Test(req, opts...)
}

// ExpectJSON is a test helper that asserts a response has the expected status code
// and JSON body. It decodes the JSON into the provided output value.
//
// Example:
//
//	var user User
//	ExpectJSON(t, resp, 200, &user)
//	assert.Equal(t, "Alice", user.Name)
func ExpectJSON(t testingT, resp *http.Response, statusCode int, out any) {
	if resp.StatusCode != statusCode {
		t.Errorf("expected status %d, got %d", statusCode, resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read response body: %v", err)
		return
	}

	if err := json.Unmarshal(body, out); err != nil {
		t.Errorf("failed to decode JSON: %v\nBody: %s", err, string(body))
		return
	}
}

// testingT is a minimal interface for testing.T to allow use with other test frameworks.
type testingT interface {
	Errorf(format string, args ...interface{})
}
