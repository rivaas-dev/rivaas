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

package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"rivaas.dev/router"
)

// KeyFunc determines the rate limit key for a request (e.g., per IP, per user, per route).
type KeyFunc func(*router.Context) string

// Meta contains rate limit metadata for callbacks and logging.
type Meta struct {
	Limit        int           // Rate limit (requests per window)
	Remaining    int           // Remaining requests in current window
	ResetSeconds int           // Seconds until window reset
	Window       time.Duration // Window duration
	Key          string        // Rate limit key (e.g., "ip:192.168.1.1")
	Route        string        // Matched route pattern
	Method       string        // HTTP method
	ClientIP     string        // Client IP address
}

// CommonOptions contains shared configuration for all rate limiters.
type CommonOptions struct {
	Key        KeyFunc                     // Function to derive rate limit key
	Headers    bool                        // Emit RateLimit-* headers (IETF draft)
	Enforce    bool                        // true = block on exceed (429), false = report-only
	OnExceeded func(*router.Context, Meta) // Callback when limit exceeded
	logger     *slog.Logger                // Optional slog logger for error logging
}

// TokenBucket implements token bucket rate limiting.
// Allows bursts up to Burst size, refills at Rate tokens per second.
type TokenBucket struct {
	Rate  int              // Tokens per second
	Burst int              // Maximum tokens (burst capacity)
	Store TokenBucketStore // Optional custom store (defaults to in-memory)
}

// TokenBucketStore provides storage for token bucket rate limiting.
// This allows custom implementations (e.g., Redis-backed) for distributed systems.
type TokenBucketStore interface {
	// Allow checks if a request is allowed for the given key.
	// Returns (allowed, remaining tokens, reset time in seconds).
	Allow(key string, now time.Time) (allowed bool, remaining int, resetSeconds int)
}

// SlidingWindow implements sliding window rate limiting.
// Uses two fixed windows (current + previous) for accurate counting.
type SlidingWindow struct {
	Window time.Duration // Fixed window duration (e.g., 1 minute)
	Limit  int           // Requests per window
	Store  WindowStore   // Storage backend (in-memory, Redis, etc.)
}

// WindowStore provides storage for sliding window rate limiting.
type WindowStore interface {
	// GetCounts returns (current count, previous count, window start unix time, error).
	GetCounts(ctx context.Context, key string, window time.Duration) (int, int, int64, error)
	// Incr increments the current window count.
	Incr(ctx context.Context, key string, window time.Duration) error
}

// New creates a token bucket rate limiter middleware using functional options.
// Defaults: 100 requests/second, burst of 20, rate limit by IP.
//
// Example:
//
//	r.Use(ratelimit.New(
//	    ratelimit.WithRequestsPerSecond(50),
//	    ratelimit.WithBurst(10),
//	))
func New(opts ...Option) router.HandlerFunc {
	cfg := &config{
		requestsPerSecond: 100,
		burst:             20,
		cleanupInterval:   time.Minute,
		limiterTTL:        5 * time.Minute,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Build CommonOptions from config
	commonOpts := CommonOptions{
		Key:     cfg.keyFunc,
		Headers: true,
		Enforce: true,
		logger:  cfg.logger,
	}

	// Convert onLimitExceeded handler if provided
	if cfg.onLimitExceeded != nil {
		commonOpts.OnExceeded = func(c *router.Context, _ Meta) {
			cfg.onLimitExceeded(c)
		}
	}

	// Create token bucket from config
	tb := TokenBucket{
		Rate:  cfg.requestsPerSecond,
		Burst: cfg.burst,
	}

	return WithTokenBucket(tb, commonOpts)
}

// WithTokenBucket creates a token bucket rate limiter middleware.
func WithTokenBucket(tb TokenBucket, opts CommonOptions) router.HandlerFunc {
	if opts.Key == nil {
		opts.Key = func(c *router.Context) string {
			return "ip:" + c.ClientIP()
		}
	}

	// Use custom store if provided, otherwise default to in-memory
	var store TokenBucketStore
	if tb.Store != nil {
		store = tb.Store
	} else {
		store = newTokenBucketStore(tb.Rate, tb.Burst)
	}

	return func(c *router.Context) {
		key := opts.Key(c)

		// Check limit
		allowed, remaining, resetSeconds := store.Allow(key, time.Now())

		// Set headers if enabled
		if opts.Headers {
			c.Header("RateLimit-Limit", strconv.Itoa(tb.Burst))
			c.Header("RateLimit-Remaining", strconv.Itoa(remaining))
			c.Header("RateLimit-Reset", strconv.Itoa(resetSeconds))
		}

		if !allowed {
			// Limit exceeded
			meta := Meta{
				Limit:        tb.Burst,
				Remaining:    0,
				ResetSeconds: resetSeconds,
				Window:       time.Second, // Token bucket uses 1-second windows
				Key:          key,
				Route:        c.RoutePattern(),
				Method:       c.Request.Method,
				ClientIP:     c.ClientIP(),
			}

			// Call callback if provided
			if opts.OnExceeded != nil {
				opts.OnExceeded(c, meta)
				// Always abort after calling custom handler to prevent route handler execution
				// The custom handler is responsible for writing the response
				c.Abort()
				return
			}

			// Enforce or just report
			if opts.Enforce {
				// Set Retry-After header
				c.Header("Retry-After", strconv.Itoa(resetSeconds))

				// Return 429 response
				c.WriteErrorResponse(http.StatusTooManyRequests, "Too Many Requests")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// WithSlidingWindow creates a sliding window rate limiter middleware.
func WithSlidingWindow(sw SlidingWindow, opts CommonOptions) router.HandlerFunc {
	if opts.Key == nil {
		opts.Key = func(c *router.Context) string {
			return "ip:" + c.ClientIP()
		}
	}

	if sw.Store == nil {
		// Default to in-memory store
		sw.Store = NewInMemoryStore()
	}

	return func(c *router.Context) {
		key := opts.Key(c)
		now := time.Now()

		// Get counts from store
		curr, prev, windowStart, err := sw.Store.GetCounts(c.Request.Context(), key, sw.Window)
		if err != nil {
			// Store error - allow request but log
			if opts.logger != nil {
				opts.logger.Warn("rate limit store error", "error", err, "key", key)
			}
			c.Next()
			return
		}

		// Calculate effective usage using sliding window algorithm
		// Effective = curr + prev * (1 - elapsed/window)
		elapsed := min(now.Sub(time.Unix(windowStart, 0)), sw.Window)
		prevWeight := max(0.0, 1.0-float64(elapsed)/float64(sw.Window))
		effectiveUsage := float64(curr) + float64(prev)*prevWeight

		// Increment current window
		_ = sw.Store.Incr(c.Request.Context(), key, sw.Window)

		// Calculate remaining and reset
		remaining := max(0, int(float64(sw.Limit)-effectiveUsage))

		// Calculate reset time (seconds until current window ends)
		windowEnd := windowStart + int64(sw.Window.Seconds())
		resetSeconds := max(0, int(windowEnd-now.Unix()))

		// Set headers if enabled
		if opts.Headers {
			// Format: RateLimit-Limit: <limit>;w=<seconds>
			c.Header("RateLimit-Limit", fmt.Sprintf("%d;w=%d", sw.Limit, int(sw.Window.Seconds())))
			c.Header("RateLimit-Remaining", strconv.Itoa(remaining))
			c.Header("RateLimit-Reset", strconv.Itoa(resetSeconds))
		}

		// Check if limit exceeded
		if int(effectiveUsage) >= sw.Limit {
			meta := Meta{
				Limit:        sw.Limit,
				Remaining:    0,
				ResetSeconds: resetSeconds,
				Window:       sw.Window,
				Key:          key,
				Route:        c.RoutePattern(),
				Method:       c.Request.Method,
				ClientIP:     c.ClientIP(),
			}

			// Call callback if provided
			if opts.OnExceeded != nil {
				opts.OnExceeded(c, meta)
				// Always abort after calling custom handler to prevent route handler execution
				// The custom handler is responsible for writing the response
				c.Abort()
				return
			}

			// Enforce or just report
			if opts.Enforce {
				// Set Retry-After header
				c.Header("Retry-After", strconv.Itoa(resetSeconds))

				// Return 429 response
				c.WriteErrorResponse(http.StatusTooManyRequests, "Too Many Requests")
				c.Abort()
				return
			}
		}

		// Check if aborted (e.g., by custom handler)
		if c.IsAborted() {
			return
		}

		c.Next()
	}
}

// PerRoute wraps a rate limiter middleware for per-route application.
// This allows different rate limits for different routes.
//
// Example:
//
//	r.GET("/expensive", handler, ratelimit.PerRoute(
//	    ratelimit.WithSlidingWindow(...),
//	))
func PerRoute(m router.HandlerFunc) router.HandlerFunc {
	return m
}
