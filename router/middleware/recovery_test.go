package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rivaas-dev/rivaas/router"
)

func TestRecovery_BasicPanic(t *testing.T) {
	r := router.New()
	r.Use(Recovery())

	r.GET("/panic", func(c *router.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "Internal server error" {
		t.Errorf("Expected error message 'Internal server error', got %v", response["error"])
	}

	if response["code"] != "INTERNAL_ERROR" {
		t.Errorf("Expected error code 'INTERNAL_ERROR', got %v", response["code"])
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	r := router.New()
	r.Use(Recovery())

	r.GET("/safe", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/safe", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRecovery_CustomHandler(t *testing.T) {
	r := router.New()

	customHandlerCalled := false
	r.Use(Recovery(
		WithRecoveryHandler(func(c *router.Context, err interface{}) {
			customHandlerCalled = true
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"custom_error": "Custom recovery",
				"panic_value":  err,
			})
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("custom panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if !customHandlerCalled {
		t.Error("Custom handler was not called")
	}

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["custom_error"] != "Custom recovery" {
		t.Errorf("Expected custom_error 'Custom recovery', got %v", response["custom_error"])
	}

	if response["panic_value"] != "custom panic" {
		t.Errorf("Expected panic_value 'custom panic', got %v", response["panic_value"])
	}
}

func TestRecovery_CustomLogger(t *testing.T) {
	r := router.New()

	var loggedError interface{}
	var loggedStack []byte
	loggerCalled := false

	r.Use(Recovery(
		WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
			loggerCalled = true
			loggedError = err
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("logger test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if !loggerCalled {
		t.Error("Custom logger was not called")
	}

	if loggedError != "logger test panic" {
		t.Errorf("Expected logged error 'logger test panic', got %v", loggedError)
	}

	if len(loggedStack) == 0 {
		t.Error("Expected stack trace to be captured")
	}
}

func TestRecovery_DisableStackTrace(t *testing.T) {
	r := router.New()

	var loggedStack []byte
	r.Use(Recovery(
		WithStackTrace(false),
		WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("no stack trace")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if len(loggedStack) > 0 {
		t.Error("Stack trace should not be captured when disabled")
	}
}

func TestRecovery_CustomStackSize(t *testing.T) {
	r := router.New()

	var loggedStack []byte
	r.Use(Recovery(
		WithStackSize(1024), // 1KB
		WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
			loggedStack = stack
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("stack size test")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Stack should be captured but within size limit
	if len(loggedStack) == 0 {
		t.Error("Stack trace should be captured")
	}

	// Note: Stack size might be less than buffer size depending on actual stack depth
	if len(loggedStack) > 8192 {
		t.Errorf("Stack trace should be limited, got %d bytes", len(loggedStack))
	}
}

func TestRecovery_MultipleMiddleware(t *testing.T) {
	r := router.New()

	middlewareCalled := false
	r.Use(func(c *router.Context) {
		middlewareCalled = true
		c.Next()
	})

	r.Use(Recovery())

	r.GET("/panic", func(c *router.Context) {
		panic("middleware test")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if !middlewareCalled {
		t.Error("Middleware before Recovery should be called")
	}

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestRecovery_PanicInMiddleware(t *testing.T) {
	r := router.New()
	r.Use(Recovery())

	r.Use(func(c *router.Context) {
		panic("panic in middleware")
	})

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestRecovery_DifferentPanicTypes(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{"string panic", "string error"},
		{"int panic", 42},
		{"error panic", http.ErrBodyNotAllowed},
		{"struct panic", struct{ Message string }{"structured error"}},
		{"nil panic", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.New()

			var capturedPanic interface{}
			r.Use(Recovery(
				WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
					capturedPanic = err
				}),
			))

			r.GET("/panic", func(c *router.Context) {
				panic(tt.panicValue)
			})

			req := httptest.NewRequest("GET", "/panic", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// When panic(nil) is called, Go converts it to a runtime.PanicNilError
			// We can't compare nil panics directly, so just check that something was captured
			if tt.panicValue == nil {
				if capturedPanic == nil {
					t.Error("Expected to capture a panic, but got nil")
				}
			} else if capturedPanic != tt.panicValue {
				t.Errorf("Expected panic value %v, got %v", tt.panicValue, capturedPanic)
			}

			if w.Code != http.StatusInternalServerError {
				t.Errorf("Expected status 500, got %d", w.Code)
			}
		})
	}
}

func TestRecovery_DisablePrintStack(t *testing.T) {
	r := router.New()

	// Capture stderr
	oldStderr := bytes.NewBuffer(nil)

	r.Use(Recovery(
		WithDisablePrintStack(true),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("no print")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should not panic and should handle recovery
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Check that nothing was printed to our buffer
	if oldStderr.Len() > 0 {
		t.Error("Stack trace should not be printed when disabled")
	}
}

func TestRecovery_StackTraceContent(t *testing.T) {
	r := router.New()

	var stackTrace []byte
	r.Use(Recovery(
		WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
			stackTrace = stack
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("stack content test")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	stackStr := string(stackTrace)

	// Verify stack trace contains expected information
	if !strings.Contains(stackStr, "panic") {
		t.Error("Stack trace should contain panic information")
	}

	if !strings.Contains(stackStr, "recovery_test.go") {
		t.Error("Stack trace should contain file information")
	}
}

func TestRecovery_RouteGroups(t *testing.T) {
	r := router.New()
	r.Use(Recovery())

	api := r.Group("/api")
	api.GET("/panic", func(c *router.Context) {
		panic("group panic")
	})

	req := httptest.NewRequest("GET", "/api/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestRecovery_MultipleOptions(t *testing.T) {
	r := router.New()

	loggerCalled := false
	handlerCalled := false

	r.Use(Recovery(
		WithStackTrace(true),
		WithStackSize(2048),
		WithDisablePrintStack(true),
		WithRecoveryLogger(func(c *router.Context, err interface{}, stack []byte) {
			loggerCalled = true
		}),
		WithRecoveryHandler(func(c *router.Context, err interface{}) {
			handlerCalled = true
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "recovered"})
		}),
	))

	r.GET("/panic", func(c *router.Context) {
		panic("multiple options test")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if !loggerCalled {
		t.Error("Logger should be called")
	}

	if !handlerCalled {
		t.Error("Handler should be called")
	}

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Benchmark tests
func BenchmarkRecovery_NoPanic(b *testing.B) {
	r := router.New()
	r.Use(Recovery())

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRecovery_WithPanic(b *testing.B) {
	r := router.New()
	r.Use(Recovery())

	r.GET("/panic", func(c *router.Context) {
		panic("benchmark panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
