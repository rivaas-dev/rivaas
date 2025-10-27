package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rivaas-dev/rivaas/router"
)

func TestCORS_NoCORSRequest(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// No Origin header means no CORS headers should be set
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers for non-CORS request")
	}
}

func TestCORS_AllowAllOrigins(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_AllowedOrigins(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowedOrigins([]string{"https://example.com", "https://app.example.com"})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectedOrigin string
	}{
		{
			name:           "allowed origin 1",
			origin:         "https://example.com",
			expectedOrigin: "https://example.com",
		},
		{
			name:           "allowed origin 2",
			origin:         "https://app.example.com",
			expectedOrigin: "https://app.example.com",
		},
		{
			name:           "disallowed origin",
			origin:         "https://evil.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.expectedOrigin {
				t.Errorf("Expected Access-Control-Allow-Origin: %s, got %s", tt.expectedOrigin, got)
			}
		})
	}
}

func TestCORS_AllowOriginFunc(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowOriginFunc(func(origin string) bool {
		return strings.HasSuffix(origin, ".example.com")
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectedOrigin string
	}{
		{
			name:           "subdomain allowed",
			origin:         "https://app.example.com",
			expectedOrigin: "https://app.example.com",
		},
		{
			name:           "another subdomain allowed",
			origin:         "https://api.example.com",
			expectedOrigin: "https://api.example.com",
		},
		{
			name:           "different domain disallowed",
			origin:         "https://evil.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.expectedOrigin {
				t.Errorf("Expected Access-Control-Allow-Origin: %s, got %s", tt.expectedOrigin, got)
			}
		})
	}
}

func TestCORS_Preflight(t *testing.T) {
	r := router.New()
	r.Use(CORS(
		WithAllowedOrigins([]string{"https://example.com"}),
		WithAllowedMethods([]string{"GET", "POST", "PUT"}),
		WithAllowedHeaders([]string{"Content-Type", "Authorization"}),
		WithMaxAge(7200),
	))
	r.POST("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})
	// Register OPTIONS handler for preflight
	r.OPTIONS("/test", func(c *router.Context) {
		// CORS middleware will handle the response
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin: https://example.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT" {
		t.Errorf("Expected Access-Control-Allow-Methods: GET, POST, PUT, got %s", w.Header().Get("Access-Control-Allow-Methods"))
	}

	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Errorf("Expected Access-Control-Allow-Headers: Content-Type, Authorization, got %s", w.Header().Get("Access-Control-Allow-Headers"))
	}

	if w.Header().Get("Access-Control-Max-Age") != "7200" {
		t.Errorf("Expected Access-Control-Max-Age: 7200, got %s", w.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORS_PreflightDisallowedOrigin(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowedOrigins([]string{"https://example.com"})))
	r.POST("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})
	// Register OPTIONS handler for preflight
	r.OPTIONS("/test", func(c *router.Context) {
		// CORS middleware will handle the response
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should not set CORS headers for disallowed origin
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers for disallowed origin")
	}
}

func TestCORS_Credentials(t *testing.T) {
	r := router.New()
	r.Use(CORS(
		WithAllowedOrigins([]string{"https://example.com"}),
		WithAllowCredentials(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin: https://example.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected Access-Control-Allow-Credentials: true")
	}
}

func TestCORS_CredentialsWithAllOrigins(t *testing.T) {
	r := router.New()
	r.Use(CORS(
		WithAllowAllOrigins(true),
		WithAllowCredentials(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// When credentials are enabled, should return specific origin instead of *
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin: https://example.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected Access-Control-Allow-Credentials: true")
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	r := router.New()
	r.Use(CORS(
		WithAllowedOrigins([]string{"https://example.com"}),
		WithExposedHeaders([]string{"X-Request-ID", "X-Rate-Limit"}),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Expose-Headers") != "X-Request-ID, X-Rate-Limit" {
		t.Errorf("Expected Access-Control-Expose-Headers: X-Request-ID, X-Rate-Limit, got %s", w.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestCORS_DefaultConfig(t *testing.T) {
	r := router.New()
	r.Use(CORS()) // Use default config
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Default config has no allowed origins, so no CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers with default restrictive config")
	}
}

func TestCORS_ActualRequest(t *testing.T) {
	r := router.New()
	r.Use(CORS(WithAllowedOrigins([]string{"https://example.com"})))
	r.POST("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"data":"test"}`))
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin: https://example.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// Should not have preflight headers on actual request
	if w.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Error("Should not have Access-Control-Allow-Methods on actual request")
	}
}

// Benchmark tests
func BenchmarkCORS_SimpleRequest(b *testing.B) {
	r := router.New()
	r.Use(CORS(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_Preflight(b *testing.B) {
	r := router.New()
	r.Use(CORS(WithAllowedOrigins([]string{"https://example.com"})))
	r.POST("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_OriginValidation(b *testing.B) {
	r := router.New()
	r.Use(CORS(WithAllowedOrigins([]string{
		"https://example.com",
		"https://app.example.com",
		"https://api.example.com",
		"https://admin.example.com",
		"https://dashboard.example.com",
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_OriginFunc(b *testing.B) {
	r := router.New()
	r.Use(CORS(WithAllowOriginFunc(func(origin string) bool {
		return strings.HasSuffix(origin, ".example.com")
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
