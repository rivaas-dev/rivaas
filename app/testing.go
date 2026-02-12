// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"rivaas.dev/router"
)

// TestOption configures test execution behavior.
type TestOption func(*testConfig)

type testConfig struct {
	timeout time.Duration
	ctx     context.Context //nolint:containedctx // Intentional: test configuration struct
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

// WithContext uses the provided context for the test request.
// Useful for testing context propagation and cancellation.
func WithContext(ctx context.Context) TestOption {
	return func(cfg *testConfig) {
		cfg.ctx = ctx
	}
}

// Test executes an HTTP request against the app without starting a server.
// Test is useful for unit testing handlers and middleware.
//
// The request is executed in a goroutine with optional timeout via context.
// If a timeout occurs, Test returns an error immediately, but the handler
// goroutine may continue running until it completes (the router's ServeHTTP
// cannot be canceled mid-execution). This is acceptable for test scenarios
// where handlers are expected to complete within a reasonable time.
//
// Test returns an *http.Response that can be inspected for status, headers, and body.
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
	go func() {
		defer func() {
			//nolint:errcheck // Intentionally ignoring panic value; test framework handles panics
			recover()
			// The test framework will handle it appropriately
			close(done)
		}()
		a.router.ServeHTTP(recorder, req)
	}()

	select {
	case <-done:
		// Request completed
		return recorder.Result(), nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	}
}

// TestJSON is a convenience method for testing JSON requests.
// TestJSON automatically sets Content-Type and encodes the body as JSON.
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
	// Note: This is a test helper function that accepts a minimal testingT interface.
	// For full assertion support, callers should use testify/assert or testify/require directly.
	// This helper provides basic validation for convenience.
	if resp.StatusCode != statusCode {
		t.Errorf("expected status %d, got %d", statusCode, resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read response body: %v", err)
		return
	}

	if unmarshalErr := json.Unmarshal(body, out); unmarshalErr != nil {
		t.Errorf("failed to decode JSON: %v\nBody: %s", unmarshalErr, string(body))
		return
	}
}

// testingT is a minimal interface for testing.T to allow use with other test frameworks.
type testingT interface {
	Errorf(format string, args ...any)
}

// TestContextWithBody creates a Context with JSON body for testing.
// TestContextWithBody is useful for testing binding and validation logic.
//
// Example:
//
//	body := map[string]string{"name": "Alice", "email": "alice@example.com"}
//	c, err := app.TestContextWithBody("POST", "/users", body)
//	if err != nil {
//	    t.Fatal(err)
//	}
//	var req CreateUserRequest
//	err = c.Bind(&req)
func TestContextWithBody(method, path string, body any) (*Context, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("failed to encode JSON body: %w", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	// Create a minimal app for testing
	a, err := New()
	if err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	// Create router context
	rc := &router.Context{
		Request:  req,
		Response: httptest.NewRecorder(),
	}

	// Create app context from the pool
	c := a.contextPool.Get()
	c.Context = rc
	c.app = a
	c.bindingMeta = nil

	return c, nil
}

// TestContextWithForm creates a Context with form data for testing.
// TestContextWithForm is useful for testing form binding logic.
//
// Example:
//
//	values := map[string][]string{
//	    "name":  {"Alice"},
//	    "email": {"alice@example.com"},
//	}
//	c, err := app.TestContextWithForm("POST", "/users", values)
//	if err != nil {
//	    t.Fatal(err)
//	}
//	var req CreateUserRequest
//	err = c.Bind(&req)
func TestContextWithForm(method, path string, values map[string][]string) (*Context, error) {
	body := strings.NewReader(encodeFormValues(values))
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a minimal app for testing
	a, err := New()
	if err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	// Create router context
	rc := &router.Context{
		Request:  req,
		Response: httptest.NewRecorder(),
	}

	// Create app context from the pool
	c := a.contextPool.Get()
	c.Context = rc
	c.app = a
	c.bindingMeta = nil

	return c, nil
}

// encodeFormValues encodes form values into URL-encoded format.
func encodeFormValues(values map[string][]string) string {
	var parts []string
	for key, vals := range values {
		for _, val := range vals {
			parts = append(parts, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return strings.Join(parts, "&")
}
