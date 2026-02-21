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

//go:build !integration

package ratelimit

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

//nolint:paralleltest // Tests rate limiting behavior
func TestRateLimit_Basic(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	// Allow 5 requests per second with burst of 5
	r.Use(New(
		WithRequestsPerSecond(5),
		WithBurst(5),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// First 5 requests should succeed (burst capacity)
	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

//nolint:paralleltest // Time-sensitive test
func TestRateLimit_TokenRefill(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	// Allow 10 requests per second
	r.Use(New(
		WithRequestsPerSecond(10),
		WithBurst(2),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// Use up the burst
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// Next request should fail
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for token refill (100ms should give us 1 token at 10 req/s)
	time.Sleep(150 * time.Millisecond)

	// Now request should succeed
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Request should succeed after token refill")
}

//nolint:paralleltest // Tests rate limiting behavior
func TestRateLimit_CustomKeyFunc(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	// Rate limit by custom header
	r.Use(New(
		WithRequestsPerSecond(5),
		WithBurst(2),
		WithKeyFunc(func(c *router.Context) string {
			return c.Request.Header.Get("X-User-Id")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// User 1: use up burst
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "user1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "User1 request %d should succeed", i+1)
	}

	// User 1: should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-Id", "user1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "User1 should be rate limited")

	// User 2: should still have tokens
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-Id", "user2")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "User2 should succeed")
}

//nolint:paralleltest // Tests rate limiting behavior
func TestRateLimit_CustomLimitHandler(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	customHandlerCalled := false

	r.Use(New(
		WithRequestsPerSecond(1),
		WithBurst(1),
		WithHandler(func(c *router.Context) {
			customHandlerCalled = true
			//nolint:errcheck // Test handler
			c.String(http.StatusTooManyRequests, "custom rate limit message")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// First request succeeds
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Second request should trigger custom handler
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, customHandlerCalled, "Custom limit handler should be called")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "custom rate limit message", w.Body.String())
}

//nolint:paralleltest // Tests concurrent behavior
func TestRateLimit_Concurrent(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	r.Use(New(
		WithRequestsPerSecond(100),
		WithBurst(50),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// Send concurrent requests
	var wg sync.WaitGroup
	successCount := 0
	rateLimitedCount := 0
	var mu sync.Mutex

	for range 100 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
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
	assert.InDelta(t, 50, successCount, 10, "Expected ~50 successful requests")
	assert.InDelta(t, 50, rateLimitedCount, 10, "Expected ~50 rate limited requests")
	t.Logf("Concurrent test: %d succeeded, %d rate limited", successCount, rateLimitedCount)
}

//nolint:paralleltest // Tests rate limiting behavior
func TestRateLimit_EmptyKey(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	// Key function that returns empty string
	r.Use(New(
		WithKeyFunc(func(_ *router.Context) string {
			return "" // Empty key
		}),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// Request should be allowed (empty key = no rate limiting)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Empty key should allow request")
}

//nolint:paralleltest // Tests rate limiting behavior
func TestRateLimit_BurstBehavior(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	// Allow 10 req/s with burst of 3
	r.Use(New(
		WithRequestsPerSecond(10),
		WithBurst(3),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// Should allow burst of 3 requests immediately
	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Burst request %d should succeed", i+1)
	}

	// 4th request should fail
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request after burst should be rate limited")
}

//nolint:paralleltest // Tests sliding window rate limiting
func TestRateLimit_WithSlidingWindow(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	sw := SlidingWindow{
		Window: 10 * time.Second,
		Limit:  2,
		Store:  NewInMemoryStore(),
	}
	opts := CommonOptions{Headers: true, Enforce: true}
	r.Use(WithSlidingWindow(sw, opts))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	// First 2 requests should succeed
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		assert.NotEmpty(t, w.Header().Get("RateLimit-Remaining"))
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "0", w.Header().Get("RateLimit-Remaining"))
}

//nolint:paralleltest // Tests PerRoute wraps middleware for per-route application
func TestRateLimit_PerRoute(t *testing.T) {
	r, err := router.New()
	require.NoError(t, err)

	sw := SlidingWindow{
		Window: 10 * time.Second,
		Limit:  1,
		Store:  NewInMemoryStore(),
	}
	opts := CommonOptions{Headers: true, Enforce: true}
	limitedMiddleware := PerRoute(WithSlidingWindow(sw, opts))

	handler := func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	}
	r.GET("/limited", limitedMiddleware, handler)
	r.GET("/unlimited", handler)

	// First request to /limited succeeds
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request to /limited is rate limited
	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// /unlimited is not rate limited
	req = httptest.NewRequest(http.MethodGet, "/unlimited", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

//nolint:paralleltest // Tests options WithCleanupInterval, WithLimiterTTL, WithLogger
func TestRateLimit_Options(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"WithCleanupInterval", []Option{WithCleanupInterval(time.Minute), WithRequestsPerSecond(10), WithBurst(5)}},
		{"WithLimiterTTL", []Option{WithLimiterTTL(5 * time.Minute), WithRequestsPerSecond(10), WithBurst(5)}},
		{"WithLogger", []Option{WithLogger(slog.Default()), WithRequestsPerSecond(10), WithBurst(5)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := router.New()
			require.NoError(t, err)
			r.Use(New(tt.opts...))
			r.GET("/test", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
