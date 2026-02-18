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

package tracing

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// TestWithExcludePaths tests the WithExcludePaths middleware option.
func TestWithExcludePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		excludePaths   []string
		requestPath    string
		expectedStatus int
	}{
		{
			name:           "single excluded path matches",
			excludePaths:   []string{"/health"},
			requestPath:    "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "excluded path does not match other paths",
			excludePaths:   []string{"/health"},
			requestPath:    "/api/users",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "multiple excluded paths",
			excludePaths:   []string{"/health", "/metrics", "/ready"},
			requestPath:    "/metrics",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "empty excluded paths",
			excludePaths:   []string{},
			requestPath:    "/api/users",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := TestingTracer(t)
			middleware := Middleware(tracer, WithExcludePaths(tt.excludePaths...))

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestWithExcludePaths_RespectsMaxExcludedPaths covers the cap at MaxExcludedPaths (1000).
func TestWithExcludePaths_RespectsMaxExcludedPaths(t *testing.T) {
	t.Parallel()

	paths := make([]string, 0, MaxExcludedPaths+1)
	for i := 0; i <= MaxExcludedPaths; i++ {
		paths = append(paths, fmt.Sprintf("/path%d", i))
	}

	tracer := TestingTracer(t)
	middleware := Middleware(tracer, WithExcludePaths(paths...))

	var spanCreated bool
	tracerWithHook := TestingTracer(t, WithSpanStartHook(func(_ context.Context, _ trace.Span, _ *http.Request) {
		spanCreated = true
	}))
	middlewareWithHook := Middleware(tracerWithHook, WithExcludePaths(paths...))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handlerWithHook := middlewareWithHook(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request to paths[999] is excluded (within cap)
	req0 := httptest.NewRequest(http.MethodGet, paths[999], nil)
	w0 := httptest.NewRecorder()
	handler.ServeHTTP(w0, req0)
	assert.Equal(t, http.StatusOK, w0.Code)

	// Request to paths[1000] (1001st) is NOT excluded because loop broke at i >= 1000
	spanCreated = false
	req1 := httptest.NewRequest(http.MethodGet, paths[MaxExcludedPaths], nil)
	w1 := httptest.NewRecorder()
	handlerWithHook.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.True(t, spanCreated, "path at index MaxExcludedPaths should not be excluded and should create span")
}

// TestWithExcludePrefixes tests the WithExcludePrefixes middleware option.
func TestWithExcludePrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		excludePrefixes []string
		requestPath     string
		expectedStatus  int
	}{
		{
			name:            "prefix matches path",
			excludePrefixes: []string{"/debug/"},
			requestPath:     "/debug/pprof",
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "prefix does not match without trailing content",
			excludePrefixes: []string{"/debug/"},
			requestPath:     "/debug",
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "multiple prefixes",
			excludePrefixes: []string{"/debug/", "/internal/", "/admin/"},
			requestPath:     "/internal/status",
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "prefix does not match different path",
			excludePrefixes: []string{"/debug/"},
			requestPath:     "/api/users",
			expectedStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := TestingTracer(t)
			middleware := Middleware(tracer, WithExcludePrefixes(tt.excludePrefixes...))

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestWithExcludePatterns tests the WithExcludePatterns middleware option.
func TestWithExcludePatterns(t *testing.T) {
	t.Parallel()

	t.Run("valid patterns", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		middleware := Middleware(tracer,
			WithExcludePatterns(`^/api/v\d+/health$`, `^/internal/.*`),
		)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test pattern matching
		testPaths := []string{"/api/v1/health", "/api/v2/health", "/internal/debug", "/api/users"}
		for _, path := range testPaths {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Middleware panics on invalid pattern", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		assert.Panics(t, func() {
			Middleware(tracer, WithExcludePatterns("[invalid"))
		})
	})

	t.Run("invalid regex with valid pattern panics with excludePatterns message", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		var panicVal any
		func() {
			defer func() { panicVal = recover() }()
			Middleware(tracer, WithExcludePatterns("[", "^/health"))
		}()
		require.NotNil(t, panicVal, "expected panic")
		assert.Contains(t, fmt.Sprint(panicVal), "excludePatterns")
	})
}

// TestMiddleware_InvalidOptionPanics covers panic when cfg.validate() returns error.
func TestMiddleware_InvalidOptionPanics(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	assert.Panics(t, func() {
		Middleware(tracer, WithExcludePatterns("["))
	})
}

// TestMiddleware_NilTracerPanics covers panic when tracer is nil.
func TestMiddleware_NilTracerPanics(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		Middleware(nil)
	})
}

// TestMustMiddleware_ValidOptionsSucceeds covers MustMiddleware.
func TestMustMiddleware_ValidOptionsSucceeds(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	handler := MustMiddleware(tracer, WithExcludePaths("/health"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMiddleware_ProbabilisticSamplingSkipsSomeSpans covers the probabilistic skip branch in startMiddlewareSpan.
func TestMiddleware_ProbabilisticSamplingSkipsSomeSpans(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t, WithSampleRate(0.0001))
	middleware := Middleware(tracer)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Many requests so some hit the skip path (hash > threshold).
	for range 1000 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

// TestResponseWriter_StatusCode_ReturnsOKWhenWriteOnly covers StatusCode() when only Write was called (no WriteHeader).
func TestResponseWriter_StatusCode_ReturnsOKWhenWriteOnly(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := Middleware(tracer)

	var capturedRW *responseWriter
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if rw, ok := w.(*responseWriter); ok {
			capturedRW = rw
		}
		// Only Write, no WriteHeader
		_, _ = w.Write([]byte("ok")) //nolint:errcheck // Test handler
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.NotNil(t, capturedRW, "handler should receive *responseWriter")
	assert.Equal(t, http.StatusOK, capturedRW.StatusCode())
}

// TestWithHeaders tests the WithHeaders middleware option.
func TestWithHeaders(t *testing.T) {
	t.Parallel()

	t.Run("records specified headers", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		middleware := Middleware(tracer, WithHeaders("X-Request-ID", "X-Correlation-ID"))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		req.Header.Set("X-Correlation-ID", "corr-456")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("filters sensitive headers", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		// Authorization and Cookie should be filtered out
		middleware := Middleware(tracer, WithHeaders("X-Request-ID", "Authorization", "Cookie"))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("Cookie", "session=abc")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("only sensitive headers results in empty record list", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			headers []string
		}{
			{
				name:    "Authorization only",
				headers: []string{"Authorization"},
			},
			{
				name:    "Cookie only",
				headers: []string{"Cookie"},
			},
			{
				name:    "mixed sensitive and non-sensitive",
				headers: []string{"X-Request-ID", "Authorization"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tracer := TestingTracer(t)
				middleware := Middleware(tracer, WithHeaders(tt.headers...))

				handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))

				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
			})
		}
	})
}

// TestWithRecordParams tests the WithRecordParams middleware option.
func TestWithRecordParams(t *testing.T) {
	t.Parallel()

	t.Run("records whitelisted params", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		middleware := Middleware(tracer, WithRecordParams("user_id", "page"))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?user_id=123&page=5&secret=password", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("empty whitelist records no params", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		middleware := Middleware(tracer, WithRecordParams())

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?user_id=123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestWithExcludeParams tests the WithExcludeParams middleware option.
func TestWithExcludeParams(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := Middleware(tracer, WithExcludeParams("password", "token", "api_key"))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?user_id=123&password=secret&token=abc", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestWithoutParams tests the WithoutParams middleware option.
func TestWithoutParams(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := Middleware(tracer, WithoutParams())

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?secret=password&token=abc123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// Middleware Functionality Tests
// =============================================================================

// TestMiddleware_BasicFunctionality tests basic middleware functionality.
func TestMiddleware_BasicFunctionality(t *testing.T) {
	t.Parallel()

	t.Run("passes through to handler", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		middleware := Middleware(tracer)

		handlerCalled := false
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})

	t.Run("disabled tracer passes through", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t, WithSampleRate(0.0))
		middleware := Middleware(tracer)

		handlerCalled := false
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// NOTE: startMiddlewareSpan's spanNamePool.Get() !ok branch (allocating a new
	// strings.Builder when the pool returns a non-*strings.Builder) is defensive
	// and not exercised by tests; the pool's New function always returns *strings.Builder.

	t.Run("enabled tracer with 0% sample rate does not create span", func(t *testing.T) {
		t.Parallel()

		// Tracer is enabled (default) but samples 0% of requests; startMiddlewareSpan
		// is still called and returns early without creating a span.
		tracer := TestingTracer(t, WithSampleRate(0))
		middleware := Middleware(tracer)

		handlerCalled := false
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestMiddleware_StatusCodes tests middleware with various HTTP status codes.
func TestMiddleware_StatusCodes(t *testing.T) {
	t.Parallel()

	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			t.Parallel()

			tracer := TestingTracer(t)
			middleware := Middleware(tracer)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, code, w.Code)
		})
	}
}

// TestMiddleware_HTTPMethods tests middleware with various HTTP methods.
func TestMiddleware_HTTPMethods(t *testing.T) {
	t.Parallel()

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			tracer := TestingTracer(t)
			middleware := Middleware(tracer)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, method, r.Method)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// =============================================================================
// Response Writer Tests
// =============================================================================

// TestResponseWriter tests the responseWriter wrapper.
func TestResponseWriter(t *testing.T) {
	t.Parallel()

	t.Run("captures status code", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		rw.WriteHeader(http.StatusCreated)

		assert.Equal(t, http.StatusCreated, rw.StatusCode())
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("default status code is 200", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		// Write without calling WriteHeader
		_, err := rw.Write([]byte("OK"))
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, rw.StatusCode())
	})

	t.Run("captures response size", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		data := []byte("Hello, World!")
		n, err := rw.Write(data)
		require.NoError(t, err)

		assert.Equal(t, len(data), n)
		assert.Equal(t, len(data), rw.Size())
	})

	t.Run("multiple writes accumulate size", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		//nolint:errcheck // Test handler
		rw.Write([]byte("Hello"))
		//nolint:errcheck // Test handler
		rw.Write([]byte(", "))
		//nolint:errcheck // Test handler
		rw.Write([]byte("World!"))

		assert.Equal(t, 13, rw.Size()) // "Hello, World!" = 13 bytes
	})

	t.Run("WriteHeader only called once", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		rw.WriteHeader(http.StatusCreated)
		rw.WriteHeader(http.StatusBadRequest) // Should be ignored

		assert.Equal(t, http.StatusCreated, rw.StatusCode())
	})

	t.Run("implements Flush", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		// Should not panic
		rw.Flush()
	})

	t.Run("Flush is no-op when underlying does not implement Flusher", func(t *testing.T) {
		t.Parallel()

		w := &minimalResponseWriter{header: make(http.Header)}
		rw := newResponseWriter(w)

		// Should not panic when underlying has no Flusher
		rw.Flush()
	})

	t.Run("Push returns ErrNotSupported when underlying does not implement Pusher", func(t *testing.T) {
		t.Parallel()

		w := &minimalResponseWriter{header: make(http.Header)}
		rw := newResponseWriter(w)

		err := rw.Push("/static.js", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, http.ErrNotSupported)
	})

	t.Run("implements Unwrap", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		unwrapped := rw.Unwrap()
		assert.Equal(t, w, unwrapped)
	})
}

// minimalResponseWriter implements only http.ResponseWriter (no Flusher, Pusher, or Hijacker).
// Used to test responseWriter behavior when the underlying writer does not support optional interfaces.
type minimalResponseWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (m *minimalResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *minimalResponseWriter) Write(b []byte) (int, error) {
	m.body = append(m.body, b...)
	if m.code == 0 {
		m.code = http.StatusOK
	}
	return len(b), nil
}

func (m *minimalResponseWriter) WriteHeader(code int) {
	m.code = code
}

// mockHijacker implements http.Hijacker for testing.
type mockHijacker struct {
	http.ResponseWriter
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

// TestResponseWriter_Hijack tests the Hijack implementation.
func TestResponseWriter_Hijack(t *testing.T) {
	t.Parallel()

	t.Run("supports hijack when underlying supports it", func(t *testing.T) {
		t.Parallel()

		hijacker := &mockHijacker{ResponseWriter: httptest.NewRecorder()}
		rw := newResponseWriter(hijacker)

		conn, buf, err := rw.Hijack()
		require.NoError(t, err)
		assert.Nil(t, conn)
		assert.Nil(t, buf)
	})

	t.Run("returns error when underlying does not support hijack", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		rw := newResponseWriter(w)

		_, _, err := rw.Hijack()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't support Hijack")
	})
}

// =============================================================================
// Context Tracing Helper Tests
// =============================================================================

// TestContextTracing_Helper tests the ContextTracing helper.
func TestContextTracing_Helper(t *testing.T) {
	t.Parallel()

	t.Run("with valid span", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		ctx, span := tracer.StartSpan(t.Context(), "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, tracer, span)

		assert.NotNil(t, ct.TraceContext())
		assert.NotNil(t, ct.GetSpan())
		assert.NotNil(t, ct.GetTracer())
	})

	t.Run("with nil span", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		ct := NewContextTracing(t.Context(), tracer, nil)

		assert.Empty(t, ct.TraceID())
		assert.Empty(t, ct.SpanID())

		// Should not panic
		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
	})

	t.Run("with nil context panics", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		// Nil context should panic (consistent with stdlib context functions)
		assert.Panics(t, func() {
			//nolint:staticcheck // Intentionally testing nil context handling
			NewContextTracing(nil, tracer, nil)
		})
	})

	t.Run("SetSpanAttribute and AddSpanEvent", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		ctx, span := tracer.StartSpan(t.Context(), "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, tracer, span)

		// Should not panic
		ct.SetSpanAttribute("string_key", "value")
		ct.SetSpanAttribute("int_key", 42)
		ct.SetSpanAttribute("bool_key", true)
		ct.AddSpanEvent("test_event")
	})
}

// TestMiddlewareConfig_Validation tests middleware configuration validation.
func TestMiddlewareConfig_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid config passes validation", func(t *testing.T) {
		t.Parallel()

		cfg := newMiddlewareConfig()
		err := cfg.validate()
		assert.NoError(t, err)
	})

	t.Run("invalid regex pattern panics", func(t *testing.T) {
		t.Parallel()

		tracer := TestingTracer(t)
		assert.Panics(t, func() {
			Middleware(tracer, WithExcludePatterns("[invalid", "(unclosed"))
		})
	})

	t.Run("invalid regex appends validation error", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			patterns []string
		}{
			{
				name:     "single invalid pattern",
				patterns: []string{"[invalid"},
			},
			{
				name:     "valid then invalid pattern",
				patterns: []string{`^/health$`, "(unclosed"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := newMiddlewareConfig()
				WithExcludePatterns(tt.patterns...)(cfg)
				err := cfg.validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "middleware validation errors")
			})
		}
	})
}

// TestShouldRecordParam tests the shouldRecordParam helper.
func TestShouldRecordParam(t *testing.T) {
	t.Parallel()

	t.Run("excluded param returns false", func(t *testing.T) {
		t.Parallel()

		cfg := newMiddlewareConfig()
		cfg.excludeParams["password"] = true

		assert.False(t, shouldRecordParam(cfg, "password"))
		assert.True(t, shouldRecordParam(cfg, "user_id"))
	})

	t.Run("whitelist only records whitelisted params", func(t *testing.T) {
		t.Parallel()

		cfg := newMiddlewareConfig()
		cfg.recordParamsList = []string{"user_id", "page"}

		assert.True(t, shouldRecordParam(cfg, "user_id"))
		assert.True(t, shouldRecordParam(cfg, "page"))
		assert.False(t, shouldRecordParam(cfg, "secret"))
	})

	t.Run("blacklist takes precedence over whitelist", func(t *testing.T) {
		t.Parallel()

		cfg := newMiddlewareConfig()
		cfg.recordParamsList = []string{"user_id", "password"}
		cfg.excludeParams["password"] = true

		assert.True(t, shouldRecordParam(cfg, "user_id"))
		assert.False(t, shouldRecordParam(cfg, "password")) // Excluded even though whitelisted
	})
}

// TestTracingResponseWriterImplementsMarkerInterface verifies that the
// tracing middleware response writer implements the marker interface.
func TestTracingResponseWriterImplementsMarkerInterface(t *testing.T) {
	t.Parallel()

	innerWriter := httptest.NewRecorder()
	wrapped := newResponseWriter(innerWriter)

	// Verify it implements the marker interface
	marker, ok := any(wrapped).(observabilityWrappedWriter)
	require.True(t, ok, "responseWriter should implement observabilityWrappedWriter")
	assert.True(t, marker.IsObservabilityWrapped())
}

// TestTracingMiddlewareDoubleWrappingPrevention tests that the tracing middleware
// doesn't wrap an already-wrapped response writer. Covers the observabilityWrappedWriter
// branch in Middleware where an already-wrapped writer is used as-is and the span is
// finished with http.StatusOK.
func TestTracingMiddlewareDoubleWrappingPrevention(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := Middleware(tracer)

	// Create a pre-wrapped writer
	preWrappedWriter := httptest.NewRecorder()
	alreadyWrapped := &mockWrappedWriter{ResponseWriter: preWrappedWriter}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Serve with already-wrapped writer
	handler.ServeHTTP(alreadyWrapped, req)

	assert.Equal(t, http.StatusOK, alreadyWrapped.StatusCode())
	// The middleware should detect it's already wrapped and not wrap again
}

// TestTracingMiddlewareWithExcludedPathsNoWrapping tests that excluded paths skip wrapping.
func TestTracingMiddlewareWithExcludedPathsNoWrapping(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := Middleware(tracer, WithExcludePaths("/health"))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if writer is wrapped
		if _, ok := w.(observabilityWrappedWriter); ok {
			//nolint:errcheck // Test handler
			w.Write([]byte("wrapped"))
		} else {
			//nolint:errcheck // Test handler
			w.Write([]byte("not-wrapped"))
		}
	}))

	// Test excluded path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, "not-wrapped", w.Body.String(), "Excluded path should not be wrapped")

	// Test non-excluded path
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, "wrapped", w2.Body.String(), "Non-excluded path should be wrapped")
}

// mockWrappedWriter is a mock implementation of an already-wrapped response writer.
type mockWrappedWriter struct {
	http.ResponseWriter

	statusCode int
	size       int
	written    bool
}

func (m *mockWrappedWriter) WriteHeader(code int) {
	if !m.written {
		m.statusCode = code
		m.ResponseWriter.WriteHeader(code)
		m.written = true
	}
}

func (m *mockWrappedWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.written = true
		m.statusCode = http.StatusOK
	}
	n, err := m.ResponseWriter.Write(b)
	m.size += n

	return n, err
}

func (m *mockWrappedWriter) StatusCode() int {
	if m.statusCode == 0 {
		return http.StatusOK
	}

	return m.statusCode
}

func (m *mockWrappedWriter) Size() int64 {
	return int64(m.size)
}

func (m *mockWrappedWriter) IsObservabilityWrapped() bool {
	return true
}

func (m *mockWrappedWriter) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}
