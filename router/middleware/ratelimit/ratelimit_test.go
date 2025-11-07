package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"rivaas.dev/router"
)

func TestRateLimit_Basic(t *testing.T) {
	r := router.MustNew()

	// Allow 5 requests per second with burst of 5
	r.Use(New(
		WithRequestsPerSecond(5),
		WithBurst(5),
	))

	r.GET("/test", func(c *router.Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// First 5 requests should succeed (burst capacity)
	for i := range 5 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

func TestRateLimit_TokenRefill(t *testing.T) {
	r := router.MustNew()

	// Allow 10 requests per second
	r.Use(New(
		WithRequestsPerSecond(10),
		WithBurst(2),
	))

	r.GET("/test", func(c *router.Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// Use up the burst
	for i := range 2 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Next request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}

	// Wait for token refill (100ms should give us 1 token at 10 req/s)
	time.Sleep(150 * time.Millisecond)

	// Now request should succeed
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("After token refill: expected 200, got %d", w.Code)
	}
}

func TestRateLimit_CustomKeyFunc(t *testing.T) {
	r := router.MustNew()

	// Rate limit by custom header
	r.Use(New(
		WithRequestsPerSecond(5),
		WithBurst(2),
		WithKeyFunc(func(c *router.Context) string {
			return c.Request.Header.Get("X-User-ID")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// User 1: use up burst
	for i := range 2 {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", "user1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("User1 Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// User 1: should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("User1 rate limited: expected 429, got %d", w.Code)
	}

	// User 2: should still have tokens
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user2")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("User2: expected 200, got %d", w.Code)
	}
}

func TestRateLimit_CustomLimitHandler(t *testing.T) {
	r := router.MustNew()

	customHandlerCalled := false

	r.Use(New(
		WithRequestsPerSecond(1),
		WithBurst(1),
		WithHandler(func(c *router.Context) {
			customHandlerCalled = true
			c.String(http.StatusTooManyRequests, "custom rate limit message")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First request succeeds
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request: expected 200, got %d", w.Code)
	}

	// Second request should trigger custom handler
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !customHandlerCalled {
		t.Error("Custom limit handler was not called")
	}

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}

	if w.Body.String() != "custom rate limit message" {
		t.Errorf("Expected custom message, got %q", w.Body.String())
	}
}

func TestRateLimit_Concurrent(t *testing.T) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(100),
		WithBurst(50),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Send concurrent requests
	var wg sync.WaitGroup
	successCount := 0
	rateLimitedCount := 0
	var mu sync.Mutex

	for range 100 {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			mu.Lock()
			defer mu.Unlock()

			switch w.Code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				rateLimitedCount++
			}
		})
	}

	wg.Wait()

	// With burst of 50, we should have ~50 successful and ~50 rate limited
	if successCount < 40 || successCount > 60 {
		t.Errorf("Expected ~50 successful requests, got %d", successCount)
	}

	if rateLimitedCount < 40 || rateLimitedCount > 60 {
		t.Errorf("Expected ~50 rate limited requests, got %d", rateLimitedCount)
	}

	t.Logf("Concurrent test: %d succeeded, %d rate limited", successCount, rateLimitedCount)
}

func TestRateLimit_EmptyKey(t *testing.T) {
	r := router.MustNew()

	// Key function that returns empty string
	r.Use(New(
		WithKeyFunc(func(_ *router.Context) string {
			return "" // Empty key
		}),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Request should be allowed (empty key = no rate limiting)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for empty key, got %d", w.Code)
	}
}

func TestRateLimit_BurstBehavior(t *testing.T) {
	r := router.MustNew()

	// Allow 10 req/s with burst of 3
	r.Use(New(
		WithRequestsPerSecond(10),
		WithBurst(3),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Should allow burst of 3 requests immediately
	for i := range 3 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Burst request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 after burst, got %d", w.Code)
	}
}

func BenchmarkRateLimit(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000), // Very high limit to avoid rate limiting in benchmark
		WithBurst(1000000),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRateLimit_ParallelSameKey(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000),
		WithBurst(1000000),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test", nil)

		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRateLimit_ParallelDifferentKeys(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000),
		WithBurst(1000000),
		WithKeyFunc(func(c *router.Context) string {
			return c.Request.Header.Get("X-User-ID")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		userID := 0

		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-User-ID", string(rune(userID)))
			userID++

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}
