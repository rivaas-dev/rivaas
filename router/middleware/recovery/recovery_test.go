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

package recovery

import (
	"encoding/json"
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

	var loggedError any
	var loggedStack []byte
	loggerCalled := false

	r.Use(New(
		WithLogger(func(_ *router.Context, err any, stack []byte) {
			loggerCalled = true
			loggedError = err
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("logger test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, loggerCalled, "Custom logger should be called")
	assert.Equal(t, "logger test panic", loggedError)
	assert.NotEmpty(t, loggedStack, "Expected stack trace to be captured")
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_DisableStackTrace(t *testing.T) {
	r := router.MustNew()

	var loggedStack []byte
	r.Use(New(
		WithStackTrace(false),
		WithLogger(func(_ *router.Context, _ any, stack []byte) {
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("no stack trace")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Empty(t, loggedStack, "Stack trace should not be captured when disabled")
}

//nolint:paralleltest // Tests panic recovery behavior with shared state
func TestRecovery_CustomStackSize(t *testing.T) {
	r := router.MustNew()

	var loggedStack []byte
	r.Use(New(
		WithStackSize(1024), // 1KB
		WithLogger(func(_ *router.Context, _ any, stack []byte) {
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("stack size test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Stack should be captured but within size limit
	assert.NotEmpty(t, loggedStack, "Stack trace should be captured")

	// Note: Stack size might be less than buffer size depending on actual stack depth
	assert.LessOrEqual(t, len(loggedStack), 8192)
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

			var capturedPanic any
			r.Use(New(
				WithLogger(func(_ *router.Context, err any, _ []byte) {
					capturedPanic = err
				}),
			))

			r.GET("/panic", func(_ *router.Context) {
				panic(tt.panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// When panic(nil) is called, Go converts it to a runtime.PanicNilError
			// We can't compare nil panics directly, so just check that something was captured
			if tt.panicValue == nil {
				assert.NotNil(t, capturedPanic, "Expected to capture a panic")
			} else {
				assert.Equal(t, tt.panicValue, capturedPanic)
			}

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_CustomLoggerDisablesPrint(t *testing.T) {
	r := router.MustNew()

	loggerCalled := false
	r.Use(New(
		WithLogger(func(_ *router.Context, _ any, _ []byte) {
			loggerCalled = true
			// Custom logger - doesn't print to stderr
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("custom logger test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, loggerCalled, "Custom logger should be called")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

//nolint:paralleltest // Tests panic recovery behavior
func TestRecovery_StackTraceContent(t *testing.T) {
	r := router.MustNew()

	var stackTrace []byte
	r.Use(New(
		WithLogger(func(_ *router.Context, _ any, stack []byte) {
			stackTrace = stack
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("stack content test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	stackStr := string(stackTrace)

	// Verify stack trace contains expected information
	assert.Contains(t, stackStr, "panic")
	assert.Contains(t, stackStr, "recovery_test.go")
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

	loggerCalled := false
	handlerCalled := false

	r.Use(New(
		WithStackTrace(true),
		WithStackSize(2048),
		WithLogger(func(_ *router.Context, _ any, _ []byte) {
			loggerCalled = true
		}),
		WithHandler(func(c *router.Context, _ any) {
			handlerCalled = true
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "recovered"})
		}),
	))

	r.GET("/panic", func(_ *router.Context) {
		panic("multiple options test")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, loggerCalled, "Logger should be called")
	assert.True(t, handlerCalled, "Handler should be called")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
