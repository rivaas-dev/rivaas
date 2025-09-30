package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
)

// BenchmarkRivaasRouter benchmarks our optimized router
func BenchmarkRivaasRouter(b *testing.B) {
	r := New() // Now uses the optimized implementation
	r.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello")
	})
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})
	r.GET("/users/:id/posts/:post_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkStandardMux benchmarks Go's standard library mux
func BenchmarkStandardMux(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})
	mux.HandleFunc("/users/123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123"))
	})
	mux.HandleFunc("/users/123/posts/456", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123, Post: 456"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		mux.ServeHTTP(w, req)
	}
}

// BenchmarkSimpleRouter benchmarks a simple map-based router
func BenchmarkSimpleRouter(b *testing.B) {
	routes := map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello"))
		},
		"/users/123": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User: 123"))
		},
		"/users/123/posts/456": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User: 123, Post: 456"))
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if route, exists := routes[r.URL.Path]; exists {
			route(w, r)
		} else {
			http.NotFound(w, r)
		}
	}

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		handler(w, req)
	}
}

// BenchmarkGinRouter benchmarks Gin router
func BenchmarkGinRouter(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello")
	})
	r.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})
	r.GET("/users/:id/posts/:post_id", func(c *gin.Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkEchoRouter benchmarks Echo router
func BenchmarkEchoRouter(b *testing.B) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello")
	})
	e.GET("/users/:id", func(c echo.Context) error {
		return c.String(http.StatusOK, "User: "+c.Param("id"))
	})
	e.GET("/users/:id/posts/:post_id", func(c echo.Context) error {
		return c.String(http.StatusOK, "User: "+c.Param("id")+", Post: "+c.Param("post_id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		e.ServeHTTP(w, req)
	}
}
