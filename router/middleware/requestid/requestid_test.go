package requestid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rivaas.dev/router"
)

func TestRequestID_GeneratesID(t *testing.T) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header to be set")
	}

	// Default generator produces 32 character hex string (16 bytes * 2)
	if len(requestID) != 32 {
		t.Errorf("Expected request ID length 32, got %d", len(requestID))
	}
}

func TestRequestID_AllowClientID(t *testing.T) {
	r := router.New()
	r.Use(New(WithAllowClientID(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	clientID := "client-provided-id-123"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID != clientID {
		t.Errorf("Expected request ID %s, got %s", clientID, requestID)
	}
}

func TestRequestID_DisallowClientID(t *testing.T) {
	r := router.New()
	r.Use(New(WithAllowClientID(false)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	clientID := "client-provided-id-123"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == clientID {
		t.Error("Should not use client-provided ID when disabled")
	}

	if requestID == "" {
		t.Error("Should generate new ID when client ID is disallowed")
	}
}

func TestRequestID_CustomHeader(t *testing.T) {
	r := router.New()
	r.Use(New(WithHeader("X-Correlation-ID")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Correlation-ID")
	if requestID == "" {
		t.Error("Expected X-Correlation-ID header to be set")
	}

	// Default header should not be set
	if w.Header().Get("X-Request-ID") != "" {
		t.Error("Default X-Request-ID header should not be set")
	}
}

func TestRequestID_CustomGenerator(t *testing.T) {
	counter := 0
	r := router.New()
	r.Use(New(WithGenerator(func() string {
		counter++
		return "custom-id-" + string(rune('0'+counter))
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// First request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	requestID1 := w.Header().Get("X-Request-ID")
	if !strings.HasPrefix(requestID1, "custom-id-") {
		t.Errorf("Expected custom ID format, got %s", requestID1)
	}

	// Second request
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	requestID2 := w.Header().Get("X-Request-ID")
	if requestID1 == requestID2 {
		t.Error("Each request should get unique ID")
	}
}

func TestRequestID_MultipleRequests(t *testing.T) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Request ID should be generated")
		}

		if ids[requestID] {
			t.Errorf("Duplicate request ID: %s", requestID)
		}
		ids[requestID] = true
	}

	if len(ids) != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", len(ids))
	}
}

func TestRequestID_CombinedOptions(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithHeader("X-Trace-ID"),
		WithAllowClientID(false),
		WithGenerator(func() string {
			return "generated-123"
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Try to provide client ID (should be ignored)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-ID", "client-id")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Trace-ID")
	if requestID != "generated-123" {
		t.Errorf("Expected generated-123, got %s", requestID)
	}
}

// Benchmark tests
func BenchmarkRequestID_Generate(b *testing.B) {
	r := router.New()
	r.Use(New(WithAllowClientID(false)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRequestID_UseClientID(b *testing.B) {
	r := router.New()
	r.Use(New(WithAllowClientID(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "client-provided-id")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
