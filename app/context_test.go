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

//go:build !integration

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/tracing"
	"rivaas.dev/validation"
)

func TestContext_Bind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "valid request",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			body: map[string]any{
				"name": "Alice",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			body: map[string]any{
				"name":  "Alice",
				"email": "not-an-email",
				"age":   30,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.Bind(&req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				body, ok := tt.body.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, body["name"], req.Name)
			}
		})
	}
}

func TestContext_Bind_WithOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		opts    []BindOption
		wantErr bool
	}{
		{
			name: "strict mode rejects unknown fields",
			body: map[string]any{
				"name":    "Alice",
				"email":   "alice@example.com",
				"age":     30,
				"unknown": "field",
			},
			opts:    []BindOption{WithStrict()},
			wantErr: true,
		},
		{
			name: "partial mode allows missing required fields",
			body: map[string]any{
				"name": "Alice",
			},
			opts:    []BindOption{WithPartial()},
			wantErr: false,
		},
		{
			name: "without validation skips validation",
			body: map[string]any{
				"name":  "A",
				"email": "not-an-email",
			},
			opts:    []BindOption{WithoutValidation()},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.Bind(&req, tt.opts...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContext_MustBind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   any
		wantOK bool
	}{
		{
			name: "valid request returns true",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantOK: true,
		},
		{
			name: "invalid request returns false",
			body: map[string]any{
				"name": "Alice",
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			ok := c.MustBind(&req)

			assert.Equal(t, tt.wantOK, ok)
			if ok {
				body, bodyOK := tt.body.(map[string]any)
				require.True(t, bodyOK)
				assert.Equal(t, body["name"], req.Name)
			}
		})
	}
}

func TestContext_BindOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "binds without validation",
			body: map[string]any{
				"name":  "A", // Too short but validation skipped
				"email": "not-an-email",
				"age":   -1,
			},
			wantErr: false,
		},
		{
			name:    "malformed JSON fails",
			body:    "not-json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.BindOnly(&req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContext_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     testBindRequest
		wantErr bool
	}{
		{
			name: "valid struct passes validation",
			req: testBindRequest{
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   30,
			},
			wantErr: false,
		},
		{
			name: "invalid email fails validation",
			req: testBindRequest{
				Name:  "Alice",
				Email: "not-an-email",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "age below minimum fails",
			req: testBindRequest{
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", nil)
			require.NoError(t, err)

			err = c.Validate(&tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				var verr *validation.Error
				assert.ErrorAs(t, err, &verr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContext_Validate_WithOptions(t *testing.T) {
	t.Parallel()

	// WithValidatePartial and WithValidateOptions are accepted and applied
	c, err := TestContextWithBody("POST", "/test", nil)
	require.NoError(t, err)

	req := testBindRequest{Name: "Alice", Email: "alice@example.com", Age: 30}
	err = c.Validate(&req, WithValidatePartial())
	assert.NoError(t, err)

	// WithValidateOptions pass-through
	err = c.Validate(&req, WithValidateOptions(validation.WithMaxErrors(10)))
	assert.NoError(t, err)
}

func TestWithValidationEngine(t *testing.T) {
	t.Parallel()

	// App with custom engine that limits to 1 error (truncated)
	engine := validation.MustNew(validation.WithMaxErrors(1))
	a, err := New(
		WithServiceName("test"),
		WithValidationEngine(engine),
	)
	require.NoError(t, err)

	// Bind request with multiple missing required fields; custom engine should return only 1 error
	c, err := TestContextWithBodyAndApp(a, "POST", "/test", map[string]any{})
	require.NoError(t, err)

	var req testBindRequest
	bindErr := c.Bind(&req)
	require.Error(t, bindErr)

	var verr *validation.Error
	require.ErrorAs(t, bindErr, &verr)
	assert.Len(t, verr.Fields, 1, "custom engine WithMaxErrors(1) should return at most 1 error")
	assert.True(t, verr.Truncated, "error list should be truncated")
}

func TestContext_Presence(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	require.NoError(t, err)

	// Bind to trigger presence tracking
	var req testBindRequest
	err = c.Bind(&req)
	require.NoError(t, err)

	pm := c.Presence()
	require.NotNil(t, pm)
	assert.True(t, pm.Has("name"))
	assert.True(t, pm.Has("email"))
	assert.False(t, pm.Has("age"))
}

func TestContext_ResetBinding(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	// First bind
	var req1 testBindRequest
	err = c.Bind(&req1)
	require.NoError(t, err)

	// Reset binding metadata
	c.ResetBinding()

	// Presence should be nil after reset
	pm := c.Presence()
	assert.Nil(t, pm)
}

func TestContext_Fail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantAbort  bool
	}{
		{
			name:       "formats error and aborts",
			err:        fmt.Errorf("test error"),
			wantStatus: 500,
			wantAbort:  true,
		},
		{
			name:       "nil error does nothing",
			err:        nil,
			wantStatus: 0,
			wantAbort:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("GET", "/test", nil)
			require.NoError(t, err)

			c.Fail(tt.err)

			if tt.wantAbort {
				assert.True(t, c.IsAborted())
			} else {
				assert.False(t, c.IsAborted())
			}
		})
	}
}

func TestContext_FailStatus(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	testErr := fmt.Errorf("test error")
	c.FailStatus(404, testErr)

	assert.True(t, c.IsAborted())
}

func TestContext_Fail_withNilApp_doesNotPanic(t *testing.T) {
	t.Parallel()

	a, err := New()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rc := &router.Context{Request: req, Response: rec}

	c := a.contextPool.Get()
	c.Context = rc
	// Intentionally leave c.app nil to simulate pool reuse or manual construction.

	testErr := fmt.Errorf("test error")
	c.Fail(testErr)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Greater(t, rec.Body.Len(), 0)
}

func TestContext_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantAbort bool
	}{
		{
			name:      "with error",
			err:       fmt.Errorf("user not found"),
			wantAbort: true,
		},
		{
			name:      "with nil for generic message",
			err:       nil,
			wantAbort: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("GET", "/test", nil)
			require.NoError(t, err)

			c.NotFound(tt.err)

			assert.Equal(t, tt.wantAbort, c.IsAborted())
		})
	}
}

func TestContext_BadRequest(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.BadRequest(fmt.Errorf("invalid input"))

	assert.True(t, c.IsAborted())
}

func TestContext_Unauthorized(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.Unauthorized(fmt.Errorf("invalid token"))

	assert.True(t, c.IsAborted())
}

func TestContext_Forbidden(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.Forbidden(fmt.Errorf("insufficient permissions"))

	assert.True(t, c.IsAborted())
}

func TestContext_Conflict(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.Conflict(fmt.Errorf("user already exists"))

	assert.True(t, c.IsAborted())
}

func TestContext_Gone(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.Gone(fmt.Errorf("resource deleted"))

	assert.True(t, c.IsAborted())
}

func TestContext_UnprocessableEntity(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.UnprocessableEntity(fmt.Errorf("validation failed"))

	assert.True(t, c.IsAborted())
}

func TestContext_TooManyRequests(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.TooManyRequests(fmt.Errorf("rate limit exceeded"))

	assert.True(t, c.IsAborted())
}

func TestContext_InternalError(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.InternalError(fmt.Errorf("internal error"))

	assert.True(t, c.IsAborted())
}

func TestContext_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	c.ServiceUnavailable(fmt.Errorf("maintenance mode"))

	assert.True(t, c.IsAborted())
}

// TestContext_ObservabilityMethods_NoPanic verifies that observability methods
// do not panic when metrics/tracing are not configured (app created with New() has nil metrics/tracing).
func TestContext_ObservabilityMethods_NoPanic(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("GET", "/test", nil)
	require.NoError(t, err)

	// Tracing methods — should return empty or no-op when no span in context
	assert.Empty(t, c.TraceID())
	assert.Empty(t, c.SpanID())
	assert.NotNil(t, c.TraceContext())
	c.SetSpanAttribute("key", "value")
	c.AddSpanEvent("event")
	span := c.Span()
	assert.False(t, span.SpanContext().IsValid(), "Span should be non-recording when tracing not enabled")

	// Metrics methods — should no-op when metrics not configured
	c.RecordHistogram("test_histogram", 1.0)
	c.IncrementCounter("test_counter")
	c.SetGauge("test_gauge", 42)
	c.AddCounter("test_add_counter", 10)

	// Tracer, StartSpan, FinishSpan — should no-op or return nil when tracing not configured
	assert.Nil(t, c.Tracer())
	ctx, childSpan := c.StartSpan("child-op")
	require.NotNil(t, ctx)
	require.NotNil(t, childSpan)
	assert.False(t, childSpan.IsRecording(), "StartSpan should return non-recording span when tracing disabled")
	c.FinishSpan(childSpan, 0)
	c.FinishSpan(nil, 0) // no panic with nil span
}

// testContextWithTracing creates a Context with tracing enabled (noop provider) for testing.
func testContextWithTracing(t *testing.T, method, path string, body any) *Context {
	t.Helper()
	a, err := New(
		WithObservability(WithTracing(tracing.WithNoop())),
	)
	require.NoError(t, err)
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rc := &router.Context{Request: req, Response: httptest.NewRecorder()}
	c := a.contextPool.Get()
	c.Context = rc
	c.app = a
	c.bindingMeta = nil
	return c
}

// testContextWithMetrics creates a Context with metrics enabled (stdout provider) for testing.
func testContextWithMetrics(t *testing.T, method, path string) *Context {
	t.Helper()
	a, err := New(
		WithObservability(WithMetrics(metrics.WithStdout())),
	)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, nil)
	rc := &router.Context{Request: req, Response: httptest.NewRecorder()}
	c := a.contextPool.Get()
	c.Context = rc
	c.app = a
	c.bindingMeta = nil
	return c
}

func TestContext_Tracer(t *testing.T) {
	t.Parallel()

	t.Run("tracing disabled returns nil", func(t *testing.T) {
		t.Parallel()
		c, err := TestContextWithBody("GET", "/test", nil)
		require.NoError(t, err)
		assert.Nil(t, c.Tracer())
	})

	t.Run("tracing enabled returns non-nil tracer", func(t *testing.T) {
		t.Parallel()
		c := testContextWithTracing(t, "GET", "/test", nil)
		tr := c.Tracer()
		require.NotNil(t, tr)
		assert.True(t, tr.IsEnabled())
	})
}

func TestContext_StartSpan(t *testing.T) {
	t.Parallel()

	t.Run("tracing disabled returns request context and non-recording span", func(t *testing.T) {
		t.Parallel()
		c, err := TestContextWithBody("GET", "/test", nil)
		require.NoError(t, err)
		ctx, span := c.StartSpan("db-query")
		require.NotNil(t, ctx)
		require.NotNil(t, span)
		assert.False(t, span.IsRecording())
	})

	t.Run("tracing enabled returns context and recording span", func(t *testing.T) {
		t.Parallel()
		c := testContextWithTracing(t, "GET", "/test", nil)
		ctx, span := c.StartSpan("db-query")
		require.NotNil(t, ctx)
		require.NotNil(t, span)
		assert.True(t, span.IsRecording())
		// End span to satisfy tracer
		c.FinishSpan(span, 0)
	})
}

func TestContext_FinishSpan(t *testing.T) {
	t.Parallel()

	t.Run("nil span does not panic", func(t *testing.T) {
		t.Parallel()
		c, err := TestContextWithBody("GET", "/test", nil)
		require.NoError(t, err)
		c.FinishSpan(nil, http.StatusOK)
	})

	t.Run("non-recording span does not panic", func(t *testing.T) {
		t.Parallel()
		c, err := TestContextWithBody("GET", "/test", nil)
		require.NoError(t, err)
		_, span := c.StartSpan("noop")
		c.FinishSpan(span, 0)
	})
}

func TestContext_AddCounter(t *testing.T) {
	t.Parallel()

	t.Run("metrics disabled is no-op", func(t *testing.T) {
		t.Parallel()
		c, err := TestContextWithBody("GET", "/test", nil)
		require.NoError(t, err)
		c.AddCounter("items_total", 42)
	})

	t.Run("metrics enabled does not panic", func(t *testing.T) {
		t.Parallel()
		c := testContextWithMetrics(t, "GET", "/test")
		c.AddCounter("items_processed", 10)
		c.AddCounter("bytes_total", 1024)
	})
}
