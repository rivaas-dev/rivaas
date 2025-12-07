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

package benchmarks

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"
	"github.com/labstack/echo/v4"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	fiberadaptor "github.com/gofiber/fiber/v2/middleware/adaptor"
	router "rivaas.dev/router"
)

// Router Comparison Benchmarks
//
// This file contains comparative benchmarks between rivaas/router and other
// popular Go web frameworks. These benchmarks are isolated in a separate
// module to avoid polluting the main module's dependencies.
//
// To run these benchmarks:
//   cd benchmarks
//   go test -bench=.

// BenchmarkRivaasRouter benchmarks the Rivaas router with formatted string responses.
// Allocations: 1 per request
//  1. Stringf variadic slice - from formatted response
func BenchmarkRivaasRouter(b *testing.B) {
	r := router.MustNew()
	r.GET("/", func(c *router.Context) {
		c.String(http.StatusOK, "Hello")
	})
	r.GET("/users/:id", func(c *router.Context) {
		c.Stringf(http.StatusOK, "User: %s", c.Param("id"))
	})
	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		c.Stringf(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	// Warm up all optimizations for performance
	r.Warmup()

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRivaasRouterPlainString benchmarks the Rivaas router with plain string concatenation.
// This avoids the Stringf variadic allocation but string concatenation and conversion
// to []byte still causes allocations.
// Allocations: 1 per request
//  1. String concatenation + []byte(value) conversion in WriteString
func BenchmarkRivaasRouterPlainString(b *testing.B) {
	r := router.MustNew()
	r.GET("/users/:id", func(c *router.Context) {
		// Manual concatenation - avoids Stringf variadic but still allocates
		c.String(http.StatusOK, "User: "+c.Param("id"))
	})

	// Warm up all optimizations for performance
	r.Warmup()

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRivaasRouterZeroAlloc benchmarks the Rivaas router with a truly static response.
// This demonstrates the router's efficiency when the handler doesn't allocate.
// Allocations: 0 per request
//
// Note: This uses a pre-allocated byte slice and writes directly to the response
// without setting headers (headers would allocate). Real applications with dynamic
// responses or headers will always have allocations.
func BenchmarkRivaasRouterZeroAlloc(b *testing.B) {
	r := router.MustNew()

	// Pre-allocate response to avoid allocations in handler
	staticResponse := []byte("Hello, World!")

	r.GET("/hello", func(c *router.Context) {
		// Don't set headers - Header.Set() allocates
		// c.Response.Header().Set("Content-Type", "text/plain")
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write(staticResponse) // No allocation - reuses existing slice
	})

	// Warm up all optimizations for performance
	r.Warmup()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		r.ServeHTTP(w, req)
	}
}

// BenchmarkStandardMux benchmarks Go's standard library mux
func BenchmarkStandardMux(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})
	mux.HandleFunc("/users/123", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123"))
	})
	mux.HandleFunc("/users/123/posts/456", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123, Post: 456"))
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		mux.ServeHTTP(w, req)
	}
}

// BenchmarkSimpleRouter benchmarks a simple map-based router
func BenchmarkSimpleRouter(b *testing.B) {
	routes := map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello"))
		},
		"/users/123": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User: 123"))
		},
		"/users/123/posts/456": func(w http.ResponseWriter, _ *http.Request) {
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

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
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

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
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

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		e.ServeHTTP(w, req)
	}
}

// BenchmarkFasthttpRouter benchmarks fasthttp with basic routing
// Note: This uses fasthttp's native RequestCtx which is more efficient than net/http
// but makes direct comparison harder due to different APIs
func BenchmarkFasthttpRouter(b *testing.B) {
	// Create a simple fasthttp handler with basic path matching
	handler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		switch path {
		case "/":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("Hello")
		case "/users/123":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("User: 123")
		case "/users/123/posts/456":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("User: 123, Post: 456")
		default:
			// For fair comparison, we handle dynamic routes
			// This is a simplified version - real fasthttp apps would use a router
			if len(path) > 7 && path[:7] == "/users/" {
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetBodyString("User: 123")
			} else {
				ctx.SetStatusCode(fasthttp.StatusNotFound)
			}
		}
	}

	// Create fasthttp request context
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/users/123")

	b.ResetTimer()
	for b.Loop() {
		handler(&ctx)
		ctx.Response.Reset()
	}
}

// BenchmarkFasthttpRouterViaAdaptor benchmarks fasthttp via net/http adaptor
// This provides a more apples-to-apples comparison with other frameworks
func BenchmarkFasthttpRouterViaAdaptor(b *testing.B) {
	// Create a simple net/http handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})
	mux.HandleFunc("/users/123", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123"))
	})
	mux.HandleFunc("/users/123/posts/456", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: 123, Post: 456"))
	})

	// Wrap with fasthttp adaptor
	fasthttpHandler := fasthttpadaptor.NewFastHTTPHandlerFunc(mux.ServeHTTP)

	// Create fasthttp request context
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/users/123")

	b.ResetTimer()
	for b.Loop() {
		fasthttpHandler(&ctx)
		ctx.Response.Reset()
	}
}

// BenchmarkChiRouter benchmarks Chi router
func BenchmarkChiRouter(b *testing.B) {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: " + id))
	})
	r.Get("/users/{id}/posts/{post_id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		postID := chi.URLParam(r, "post_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User: " + id + ", Post: " + postID))
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		r.ServeHTTP(w, req)
	}
}

// BenchmarkFiberRouter benchmarks Fiber router
func BenchmarkFiberRouter(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello")
	})
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return c.SendString("User: " + c.Params("id"))
	})
	app.Get("/users/:id/posts/:post_id", func(c *fiber.Ctx) error {
		return c.SendString("User: " + c.Params("id") + ", Post: " + c.Params("post_id"))
	})

	// Convert Fiber app to http.HandlerFunc for httptest compatibility
	handler := fiberadaptor.FiberApp(app)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		handler(w, req)
	}
}
