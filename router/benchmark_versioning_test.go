package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkVersionedRouting benchmarks version-specific routing performance
func BenchmarkVersionedRouting(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("API-Version"),
		WithValidVersions("v1", "v2"),
	))

	// Register v1 routes
	v1 := r.Version("v1")
	v1.GET("/users/:id", func(c *Context) {
		c.String(200, "v1: %s", c.Param("id"))
	})

	// Register v2 routes
	v2 := r.Version("v2")
	v2.GET("/users/:id", func(c *Context) {
		c.String(200, "v2: %s", c.Param("id"))
	})

	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/users/123", nil)
	req.Header.Set("API-Version", "v1")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkVersionedRoutingHeaderDetection benchmarks header-based version detection
func BenchmarkVersionedRoutingHeaderDetection(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("API-Version"),
		WithDefaultVersion("v1"),
	))

	v1 := r.Version("v1")
	v1.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	v2 := r.Version("v2")
	v2.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	r.WarmupOptimizations()

	b.Run("v1", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		req.Header.Set("API-Version", "v1")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})

	b.Run("v2", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		req.Header.Set("API-Version", "v2")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})

	b.Run("default", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		// No version header, should use default

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkVersionedRoutingQueryDetection benchmarks query-based version detection
func BenchmarkVersionedRoutingQueryDetection(b *testing.B) {
	r := New(WithVersioning(
		WithQueryVersioning("version"),
		WithDefaultVersion("v1"),
	))

	v1 := r.Version("v1")
	v1.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	v2 := r.Version("v2")
	v2.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	r.WarmupOptimizations()

	b.Run("v1_query", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/users/123?version=v1", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})

	b.Run("v2_query", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/users/123?version=v2", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkVersionedRoutingCustomDetector benchmarks custom version detector
func BenchmarkVersionedRoutingCustomDetector(b *testing.B) {
	r := New(WithVersioning(
		WithCustomVersionDetector(func(req *http.Request) string {
			// Extract from path prefix (e.g., /v1/users, /v2/users)
			path := req.URL.Path
			if len(path) >= 3 && path[0] == '/' && path[1] == 'v' {
				return path[1:3] // "v1" or "v2"
			}
			return "v1"
		}),
	))

	v1 := r.Version("v1")
	v1.GET("/v1/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	v2 := r.Version("v2")
	v2.GET("/v2/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	r.WarmupOptimizations()

	b.Run("v1_path", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/v1/users/123", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})

	b.Run("v2_path", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/v2/users/123", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkVersionedVsStandardRouting compares versioned vs standard routing
func BenchmarkVersionedVsStandardRouting(b *testing.B) {
	b.Run("standard", func(b *testing.B) {
		r := New()
		r.GET("/users/:id", func(c *Context) {
			c.String(200, "%s", c.Param("id"))
		})
		r.WarmupOptimizations()

		req := httptest.NewRequest("GET", "/users/123", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})

	b.Run("versioned", func(b *testing.B) {
		r := New(WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithDefaultVersion("v1"),
		))

		v1 := r.Version("v1")
		v1.GET("/users/:id", func(c *Context) {
			c.String(200, "%s", c.Param("id"))
		})
		r.WarmupOptimizations()

		req := httptest.NewRequest("GET", "/users/123", nil)
		req.Header.Set("API-Version", "v1")

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkVersionedRoutingWithGroups benchmarks version groups
func BenchmarkVersionedRoutingWithGroups(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("API-Version"),
	))

	v1 := r.Version("v1")
	api := v1.Group("/api")
	api.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	})

	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	req.Header.Set("API-Version", "v1")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
