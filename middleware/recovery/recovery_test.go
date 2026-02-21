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

package recovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_BasicPanic(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	r.GET("/panic", func(_ *router.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Internal server error", response["error"])
	assert.Equal(t, "INTERNAL_ERROR", response["code"])
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_NoPanic(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	r.GET("/safe", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_CustomHandler(t *testing.T) {
	r := router.MustNew()

	customHandlerCalled := false
	r.Use(New(
		WithHandler(func(c *router.Context, err any) {
			customHandlerCalled = true
			//nolint:errcheck // Test handler
			c.JSON(http.StatusInternalServerError, map[string]any{
				"custom_error": "Custom recovery",
				"panic_value":  err,
			})
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("custom panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, customHandlerCalled, "Custom handler should be called")
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Custom recovery", response["custom_error"])
	assert.Equal(t, "custom panic", response["panic_value"])
}

//nolint:paralleltest // Tests panic recovery behavior with shared state
func TestRecovery_CustomLogger(t *testing.T) {
	r := router.MustNew()

	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r.Use(New(
		WithLogger(logger),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("logger test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "logger test panic")
	assert.Contains(t, logOutput, "stack")
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_DisableStackTrace(t *testing.T) {
	r := router.MustNew()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r.Use(New(
		WithStackTrace(false),
		WithLogger(logger),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("no stack trace")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "no stack trace")
	// Stack trace should not be present when disabled
	assert.NotContains(t, logOutput, "Stack trace:")
	assert.NotContains(t, logOutput, "goroutine")
}

//nolint:paralleltest // Tests panic recovery behavior with shared state
func TestRecovery_CustomStackSize(t *testing.T) {
	r := router.MustNew()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r.Use(New(
		WithStackSize(1024), // 1KB
		WithLogger(logger),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("stack size test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "stack")
	// Stack should be captured but within size limit (1KB)
	assert.LessOrEqual(t, len(logOutput), 8192)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_MultipleMiddleware(t *testing.T) {
	r := router.MustNew()

	middlewareCalled := false
	r.Use(func(c *router.Context) {
		middlewareCalled = true
		c.Next()
	})

	r.Use(New())

	r.GET("/panic", func(_ *router.Context) {
		panic("middleware test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, middlewareCalled, "Middleware before Recovery should be called")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_PanicInMiddleware(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	r.Use(func(_ *router.Context) {
		panic("panic in middleware")
	})

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Subtests share router state
func TestRecovery_DifferentPanicTypes(t *testing.T) {
	tests := []struct {
		name       string
		panicValue any
	}{
		{"string panic", "string error"},
		{"int panic", 42},
		{"error panic", http.ErrBodyNotAllowed},
		{"struct panic", struct{ Message string }{"structured error"}},
		{"nil panic", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.MustNew()

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			r.Use(New(
				WithLogger(logger),
			))

			r.GET("/panic", func(_ *router.Context) {
				panic(tt.panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			logOutput := buf.String()
			assert.Contains(t, logOutput, "panic recovered")

			// Verify panic value is logged
			if tt.panicValue != nil {
				assert.Contains(t, logOutput, fmt.Sprintf("%v", tt.panicValue))
			}

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_WithoutLogging(t *testing.T) {
	r := router.MustNew()

	r.Use(New(
		WithoutLogging(),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("custom logger test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// With WithoutLogging(), no logs should be produced
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_StackTraceContent(t *testing.T) {
	r := router.MustNew()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r.Use(New(
		WithLogger(logger),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("stack content test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()

	// Verify stack trace contains expected information
	assert.Contains(t, logOutput, "panic")
	assert.Contains(t, logOutput, "recovery_test.go")
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_RouteGroups(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	api := r.Group("/api")
	api.GET("/panic", func(_ *router.Context) {
		panic("group panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_MultipleOptions(t *testing.T) {
	r := router.MustNew()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handlerCalled := false

	r.Use(New(
		WithStackTrace(true),
		WithStackSize(2048),
		WithLogger(logger),
		WithHandler(func(c *router.Context, _ any) {
			handlerCalled = true
			//nolint:errcheck // Test handler
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "recovered"})
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("multiple options test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.True(t, handlerCalled, "Handler should be called")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Tests WithPrettyStack option
func TestRecovery_WithPrettyStack(t *testing.T) {
	tests := []struct {
		name          string
		prettyStack   *bool
		wantCompact   bool // true = log contains "stack trace" and "frames"; false = colorized to stderr (we only verify 500)
		wantStatus500 bool
	}{
		{
			name:          "WithPrettyStack true triggers colorized stack path",
			prettyStack:   ptrBool(true),
			wantCompact:   false,
			wantStatus500: true,
		},
		{
			name:          "WithPrettyStack false triggers compact log",
			prettyStack:   ptrBool(false),
			wantCompact:   true,
			wantStatus500: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			opts := []Option{WithLogger(logger), WithStackTrace(true)}
			if tt.prettyStack != nil {
				opts = append(opts, WithPrettyStack(*tt.prettyStack))
			}

			r := router.MustNew()
			r.Use(New(opts...))
			r.GET("/panic", func(_ *router.Context) {
				panic("pretty stack test")
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)

			logOutput := logBuf.String()
			if tt.wantCompact {
				assert.Contains(t, logOutput, "stack trace", "compact path should log 'stack trace'")
				assert.Contains(t, logOutput, "frames", "compact path should log 'frames'")
			} else {
				assert.Contains(t, logOutput, "panic recovered", "colorized path still logs panic")
			}
		})
	}
}

func ptrBool(b bool) *bool { return &b }

//nolint:paralleltest // Triggers colorized stack path for coverage
func TestRecovery_PrettyPrintedStackPath(t *testing.T) {
	// WithPrettyStack(true) + WithLogger triggers printColorizedStack (writes to stderr),
	// covering parseStackFrames, cleanFunctionName, simplifyAnonFunc, isIdentChar,
	// parseFileLine, shortenPath, isStandardLibrary.
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	r := router.MustNew()
	r.Use(New(
		WithPrettyStack(true),
		WithLogger(logger),
		WithStackTrace(true),
	))
	r.GET("/panic", func(_ *router.Context) {
		panic("colorized stack test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, logBuf.String(), "panic recovered")
}

//nolint:paralleltest // Verifies compact stack path
func TestRecovery_CompactStackPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := router.MustNew()
	r.Use(New(
		WithPrettyStack(false),
		WithLogger(logger),
		WithStackTrace(true),
	))
	r.GET("/panic", func(_ *router.Context) {
		panic("compact stack test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "stack trace")
	assert.Contains(t, logOutput, "frames")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
