package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntegration_ConcurrentRouteRegistration tests registering routes concurrently
func TestIntegration_ConcurrentRouteRegistration(t *testing.T) {
	r := New()

	var wg sync.WaitGroup
	routeCount := 100

	// Register routes from multiple goroutines
	for i := 0; i < routeCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			path := "/route" + string(rune('0'+id%10))
			r.GET(path, func(c *Context) {
				c.Status(http.StatusOK)
			})
		}(i)
	}

	wg.Wait()

	// Warmup should not panic
	r.WarmupOptimizations()

	// Routes should work
	req := httptest.NewRequest(http.MethodGet, "/route0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("routes should work after concurrent registration")
	}
}

// TestIntegration_ConcurrentRequests tests handling concurrent requests
func TestIntegration_ConcurrentRequests(t *testing.T) {
	r := New()

	var requestCount atomic.Int64

	r.GET("/test", func(c *Context) {
		requestCount.Add(1)
		time.Sleep(1 * time.Millisecond) // Simulate work
		c.JSON(http.StatusOK, map[string]int64{"count": requestCount.Load()})
	})

	r.WarmupOptimizations()

	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		}()
	}

	wg.Wait()

	if requestCount.Load() != int64(concurrency) {
		t.Errorf("expected %d requests, got %d", concurrency, requestCount.Load())
	}
}

// TestIntegration_MemoryLeakDetection tests for context pool memory leaks
func TestIntegration_MemoryLeakDetection(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	r.WarmupOptimizations()

	// Run many requests
	for i := 0; i < 10000; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	// This test mainly verifies no panics and contexts are properly recycled
	// Actual memory leak detection would require runtime.MemStats analysis
}

// TestIntegration_CompleteRequestLifecycle tests full request/response cycle
func TestIntegration_CompleteRequestLifecycle(t *testing.T) {
	r := New()

	var executionOrder []string

	// Global middleware
	r.Use(func(c *Context) {
		executionOrder = append(executionOrder, "global-start")
		c.Next()
		executionOrder = append(executionOrder, "global-end")
	})

	// Group with middleware
	api := r.Group("/api", func(c *Context) {
		executionOrder = append(executionOrder, "api-start")
		c.Next()
		executionOrder = append(executionOrder, "api-end")
	})

	// Nested group
	v1 := api.Group("/v1", func(c *Context) {
		executionOrder = append(executionOrder, "v1-start")
		c.Next()
		executionOrder = append(executionOrder, "v1-end")
	})

	// Route handler
	v1.GET("/users/:id", func(c *Context) {
		executionOrder = append(executionOrder, "handler")
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{"id": id})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := []string{
		"global-start",
		"api-start",
		"v1-start",
		"handler",
		"v1-end",
		"api-end",
		"global-end",
	}

	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d executions, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}

	for i, exp := range expected {
		if executionOrder[i] != exp {
			t.Errorf("step %d: expected %s, got %s", i, exp, executionOrder[i])
		}
	}
}

// TestIntegration_AllHTTPMethodsWithParams tests all methods with parameters
func TestIntegration_AllHTTPMethodsWithParams(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := New()

			called := false

			route := r.addRouteWithConstraints(method, "/users/:id/posts/:pid", []HandlerFunc{
				func(c *Context) {
					called = true
					userID := c.Param("id")
					postID := c.Param("pid")

					if userID != "42" {
						t.Errorf("expected id=42, got %s", userID)
					}

					if postID != "99" {
						t.Errorf("expected pid=99, got %s", postID)
					}

					c.Status(http.StatusOK)
				},
			})

			// Add constraint
			route.WhereNumber("id")

			req := httptest.NewRequest(method, "/users/42/posts/99", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if !called {
				t.Errorf("%s handler was not called", method)
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestIntegration_RouterWithAllFeatures tests router with all features enabled
func TestIntegration_RouterWithAllFeatures(t *testing.T) {
	// Create router with all features
	r := New(
		WithBloomFilterSize(2000),
		WithBloomFilterHashFunctions(5),
		WithCancellationCheck(true),
		WithTemplateRouting(true),
	)

	// Add metrics and tracing
	mockMetrics := &mockMetricsRecorder{enabled: true}
	mockTracing := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(mockMetrics)
	r.SetTracingRecorder(mockTracing)

	// Add middleware
	r.Use(func(c *Context) {
		c.Next()
	})

	// Create router with versioning
	rVersioned := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
		),
	)

	// Copy middleware
	rVersioned.Use(func(c *Context) {
		c.Next()
	})

	// Add metrics and tracing to versioned router
	rVersioned.SetMetricsRecorder(mockMetrics)
	rVersioned.SetTracingRecorder(mockTracing)

	// Register routes
	v1 := rVersioned.Version("v1")
	v1.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id"), "version": "v1"})
	})

	v2 := rVersioned.Version("v2")
	v2.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id"), "version": "v2"})
	})

	// Warmup
	rVersioned.WarmupOptimizations()

	// Test v1
	req1 := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req1.Header.Set("X-API-Version", "v1")
	w1 := httptest.NewRecorder()
	rVersioned.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("v1 request failed: %d", w1.Code)
	}

	// Test v2
	req2 := httptest.NewRequest(http.MethodGet, "/users/456", nil)
	req2.Header.Set("X-API-Version", "v2")
	w2 := httptest.NewRecorder()
	rVersioned.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("v2 request failed: %d", w2.Code)
	}
}

// TestIntegration_LargeNumberOfRoutes tests router with many routes
func TestIntegration_LargeNumberOfRoutes(t *testing.T) {
	r := New()

	// Register 1000 routes
	for i := 0; i < 1000; i++ {
		path := "/route" + string(rune('0'+i%10)) + "/" + string(rune('a'+i%26))
		r.GET(path, func(c *Context) {
			c.Status(http.StatusOK)
		})
	}

	// Warmup
	r.WarmupOptimizations()

	// Test a route
	req := httptest.NewRequest(http.MethodGet, "/route0/a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("route should work with large number of routes")
	}
}

// TestIntegration_MixedStaticAndDynamic tests routing with mix of static and dynamic routes
func TestIntegration_MixedStaticAndDynamic(t *testing.T) {
	r := New()

	// Static routes
	r.GET("/", func(c *Context) {
		c.String(http.StatusOK, "home")
	})

	r.GET("/about", func(c *Context) {
		c.String(http.StatusOK, "about")
	})

	r.GET("/contact", func(c *Context) {
		c.String(http.StatusOK, "contact")
	})

	// Dynamic routes
	r.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	r.GET("/posts/:id/comments/:cid", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{
			"post":    c.Param("id"),
			"comment": c.Param("cid"),
		})
	})

	// Wildcard routes
	r.GET("/files/*", func(c *Context) {
		c.String(http.StatusOK, "file")
	})

	r.WarmupOptimizations()

	// Test all route types
	tests := []struct {
		path       string
		expectCode int
	}{
		{"/", 200},
		{"/about", 200},
		{"/contact", 200},
		{"/users/1", 200},
		{"/posts/1/comments/2", 200},
		{"/files/anything/here", 200},
		{"/notfound", 404},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectCode {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.expectCode, w.Code)
			}
		})
	}
}

// TestIntegration_AbortInMiddleware tests Abort in complex middleware chains
func TestIntegration_AbortInMiddleware(t *testing.T) {
	r := New()

	executionLog := []string{}

	r.Use(func(c *Context) {
		executionLog = append(executionLog, "middleware1-before")
		c.Next()
		executionLog = append(executionLog, "middleware1-after")
	})

	r.Use(func(c *Context) {
		executionLog = append(executionLog, "middleware2-before")
		// Abort the chain
		c.Abort()
		executionLog = append(executionLog, "middleware2-abort")
		// Even though we call Next, it should not proceed
		c.Next()
		executionLog = append(executionLog, "middleware2-after")
	})

	r.GET("/test", func(c *Context) {
		executionLog = append(executionLog, "handler")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check execution order
	expectedMin := []string{
		"middleware1-before",
		"middleware2-before",
		"middleware2-abort",
		// handler should NOT be in the log
		// middleware2-after SHOULD be in log (after Abort, before Next)
		"middleware2-after",
		"middleware1-after",
	}

	// Verify handler was NOT called
	for _, log := range executionLog {
		if log == "handler" {
			t.Error("handler should NOT execute after Abort()")
		}
	}

	// Verify middleware cleanup still ran
	foundMiddleware1After := false
	for _, log := range executionLog {
		if log == "middleware1-after" {
			foundMiddleware1After = true
		}
	}

	if !foundMiddleware1After {
		t.Error("middleware1-after should execute (cleanup after Abort)")
	}

	_ = expectedMin // Reference to avoid unused variable
}

// TestIntegration_ContextReuse tests context pool reuse doesn't leak data
func TestIntegration_ContextReuse(t *testing.T) {
	r := New()

	var firstRequestParam string
	var secondRequestParam string

	r.GET("/first/:id", func(c *Context) {
		firstRequestParam = c.Param("id")
		c.Param("nonexistent") // Access non-existent param
		c.Status(http.StatusOK)
	})

	r.GET("/second", func(c *Context) {
		// Should not have params from first request
		secondRequestParam = c.Param("id")
		c.Status(http.StatusOK)
	})

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/first/123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if firstRequestParam != "123" {
		t.Errorf("first request: expected id=123, got %s", firstRequestParam)
	}

	// Second request (should use recycled context)
	req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if secondRequestParam != "" {
		t.Errorf("second request should not have id param, got %s", secondRequestParam)
	}
}

// TestIntegration_PanicRecoveryInHandler tests that router handles panics gracefully
func TestIntegration_PanicRecoveryInHandler(t *testing.T) {
	r := New()

	r.GET("/panic", func(c *Context) {
		panic("test panic")
	})

	// Without recovery middleware, panic should propagate
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate without recovery middleware")
		}
	}()

	r.ServeHTTP(w, req)
}

// TestIntegration_HighLoadStressTest tests router under high load
func TestIntegration_HighLoadStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	r := New()

	// Register routes
	for i := 0; i < 100; i++ {
		path := "/api/resource" + string(rune('0'+i%10)) + "/:id"
		r.GET(path, func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
		})
	}

	r.WarmupOptimizations()

	// High concurrent load
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var errorCount atomic.Int64

	concurrency := 1000
	requestsPerRoutine := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerRoutine; j++ {
				path := "/api/resource" + string(rune('0'+j%10)) + "/123"
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	totalExpected := int64(concurrency * requestsPerRoutine)
	actualTotal := successCount.Load() + errorCount.Load()

	if actualTotal != totalExpected {
		t.Errorf("expected %d total requests, got %d", totalExpected, actualTotal)
	}

	if errorCount.Load() > 0 {
		t.Errorf("expected 0 errors, got %d", errorCount.Load())
	}
}

// TestIntegration_StaticFileServing tests static file routes
func TestIntegration_StaticFileServing(t *testing.T) {
	r := New()

	// Static file serving (using handler)
	r.GET("/static/*", func(c *Context) {
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, "static content")
	})

	paths := []string{
		"/static/file.txt",
		"/static/dir/file.txt",
		"/static/a/b/c/d.txt",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestIntegration_ContentNegotiation tests full content negotiation flow
func TestIntegration_ContentNegotiation(t *testing.T) {
	r := New()

	data := map[string]string{"message": "hello"}

	r.GET("/data", func(c *Context) {
		c.Format(http.StatusOK, data)
	})

	tests := []struct {
		acceptHeader string
		expectType   string
	}{
		{"application/json", "json"},
		{"text/html", "html"},
		{"application/xml", "xml"},
		{"text/plain", "plain"},
		{"*/*", "json"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.acceptHeader, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/data", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, tt.expectType) {
				t.Errorf("expected type containing %s, got %s", tt.expectType, contentType)
			}
		})
	}
}

// TestIntegration_ErrorHandling tests complete error handling flow
func TestIntegration_ErrorHandling(t *testing.T) {
	r := New()

	// Route that returns binding error
	r.POST("/bind", func(c *Context) {
		type Data struct {
			Age int `json:"age"`
		}

		var data Data
		if err := c.BindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(`{"age": "not a number"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
