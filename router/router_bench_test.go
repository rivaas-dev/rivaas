package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"
	fiberadaptor "github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/labstack/echo/v4"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func BenchmarkRouter(b *testing.B) {
	r := New()

	// Add many routes to test performance
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/users/:id/posts/:post_id/comments",
		"/users/:id/posts/:post_id/comments/:comment_id",
		"/posts",
		"/posts/:id",
		"/posts/:id/comments",
		"/posts/:id/comments/:comment_id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
		"/api/v2/users",
		"/api/v2/posts",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for performance
	r.Warmup()

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/users/123/posts/456/comments",
		"/users/123/posts/456/comments/789",
		"/posts",
		"/posts/123",
		"/posts/123/comments",
		"/posts/123/comments/456",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
		"/api/v2/users",
		"/api/v2/posts",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

func BenchmarkRouterWithMiddleware(b *testing.B) {
	r := New()

	// Add middleware
	r.Use(func(c *Context) {
		c.Next()
	})

	// Add routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts",
		"/posts/:id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts",
		"/posts/123",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

func BenchmarkRouterGroup(b *testing.B) {
	r := New()

	// Create groups
	api := r.Group("/api/v1")
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/posts", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/posts/:id", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	admin := r.Group("/admin")
	admin.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	admin.GET("/posts", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	// Test paths
	testPaths := []string{
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
		"/admin/users",
		"/admin/posts",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

func BenchmarkRadixTree(b *testing.B) {
	root := &node{}

	// Add routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts",
		"/posts/:id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
	}

	for _, route := range routes {
		root.addRouteWithConstraints(route, []HandlerFunc{func(c *Context) {}}, nil)
	}

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts",
		"/posts/123",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &Context{}
		for pb.Next() {
			for _, path := range testPaths {
				ctx.paramCount = 0 // Reset context for reuse
				root.getRoute(path, ctx)
			}
		}
	})
}

// BenchmarkStaticRoutes tests the performance of compiled static routes
func BenchmarkStaticRoutes(b *testing.B) {
	r := New()

	// Add static routes only
	staticRoutes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	for _, route := range staticRoutes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for performance
	r.Warmup()

	// Test paths
	testPaths := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// BenchmarkContextPool tests the performance of context pooling
func BenchmarkContextPool(b *testing.B) {
	r := New()

	// Add routes with different parameter counts
	routes := []string{
		"/",                                  // 0 params
		"/users",                             // 0 params
		"/users/:id",                         // 1 param
		"/users/:id/posts",                   // 1 param
		"/users/:id/posts/:post_id",          // 2 params
		"/users/:id/posts/:post_id/comments", // 2 params
		"/users/:id/posts/:post_id/comments/:comment_id",                          // 3 params
		"/api/v1/users/:id/posts/:post_id/comments/:comment_id/replies/:reply_id", // 4 params
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for performance
	r.Warmup()

	// Test paths with different parameter counts
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/users/123/posts/456/comments",
		"/users/123/posts/456/comments/789",
		"/api/v1/users/123/posts/456/comments/789/replies/101",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// BenchmarkMemoryUsage tests memory usage with optimizations
func BenchmarkMemoryUsage(b *testing.B) {
	r := New()

	// Add many routes to test memory efficiency
	routes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts/:id",
		"/posts/:id/comments",
		"/posts/:id/comments/:comment_id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for performance
	r.Warmup()

	// Test paths
	testPaths := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts/123",
		"/posts/123/comments",
		"/posts/123/comments/456",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// BenchmarkBloomFilter tests the performance of bloom filter for negative lookups
func BenchmarkBloomFilter(b *testing.B) {
	r := New()

	// Add many static routes
	routes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for performance
	r.Warmup()

	// Test paths (mix of existing and non-existing routes)
	testPaths := []string{
		"/",               // exists
		"/health",         // exists
		"/status",         // exists
		"/metrics",        // exists
		"/api",            // exists
		"/api/v1",         // exists
		"/api/v2",         // exists
		"/users",          // exists
		"/users/profile",  // exists
		"/users/settings", // exists
		"/posts",          // exists
		"/posts/latest",   // exists
		"/posts/popular",  // exists
		"/admin",          // exists
		"/admin/users",    // exists
		"/admin/posts",    // exists
		"/admin/settings", // exists
		"/auth",           // exists
		"/auth/login",     // exists
		"/auth/logout",    // exists
		"/auth/register",  // exists
		"/static",         // exists
		"/static/css",     // exists
		"/static/js",      // exists
		"/static/images",  // exists
		"/docs",           // exists
		"/docs/api",       // exists
		"/docs/guide",     // exists
		"/help",           // exists
		"/help/faq",       // exists
		"/help/contact",   // exists
		"/nonexistent",    // doesn't exist
		"/api/v3",         // doesn't exist
		"/users/123",      // doesn't exist
		"/posts/123",      // doesn't exist
		"/admin/123",      // doesn't exist
		"/auth/123",       // doesn't exist
		"/static/123",     // doesn't exist
		"/docs/123",       // doesn't exist
		"/help/123",       // doesn't exist
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// ============================================================================
// Atomic Benchmarks (merged from atomic_bench_test.go)
// ============================================================================

func BenchmarkAtomicRouteRegistration(b *testing.B) {
	r := New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := "/benchmark" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
			r.GET(path, func(c *Context) {
				c.String(http.StatusOK, "OK")
			})
			i++
		}
	})
}

// BenchmarkAtomicRouteLookup benchmarks the performance of atomic route lookup
func BenchmarkAtomicRouteLookup(b *testing.B) {
	r := New()

	// Register test routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts",
		"/posts/:id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts",
		"/posts/123",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// BenchmarkConcurrentRegistrationAndLookup benchmarks concurrent route registration and lookup
func BenchmarkConcurrentRegistrationAndLookup(b *testing.B) {
	r := New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Alternate between registration and lookup
			if i%2 == 0 {
				// Register a route
				path := "/concurrent" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
				r.GET(path, func(c *Context) {
					c.String(http.StatusOK, "OK")
				})
			} else {
				// Lookup a route
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
			i++
		}
	})
}

// ============================================================================
// Router Comparison Benchmarks (merged from comparison_bench_test.go)
// ============================================================================

func BenchmarkRivaasRouter(b *testing.B) {
	r := New()
	r.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello")
	})
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})
	r.GET("/users/:id/posts/:post_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	// Warm up all optimizations for performance
	r.Warmup()

	req := httptest.NewRequest("GET", "/users/123", nil)
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
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
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

	req := httptest.NewRequest("GET", "/users/123", nil)
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

	req := httptest.NewRequest("GET", "/users/123", nil)
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
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
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

	req := httptest.NewRequest("GET", "/users/123", nil)
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

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		handler(w, req)
	}
}
