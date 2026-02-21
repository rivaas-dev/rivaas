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

// Package ratelimit provides middleware for rate limiting HTTP requests
// using configurable stores (in-memory, Redis, etc.) and token bucket algorithm.
package ratelimit

import (
	"log/slog"
	"time"

	"rivaas.dev/router"
)

// Option defines functional options for rate limit middleware configuration.
type Option func(*config)

// config holds the configuration for the rate limit middleware.
type config struct {
	logger            *slog.Logger
	requestsPerSecond int
	burst             int
	keyFunc           func(*router.Context) string
	onLimitExceeded   func(*router.Context)
	cleanupInterval   time.Duration
	limiterTTL        time.Duration
}

// WithRequestsPerSecond sets the number of requests allowed per second.
// Default: 100 requests/second
//
// Example:
//
//	ratelimit.New(ratelimit.WithRequestsPerSecond(50))
func WithRequestsPerSecond(rps int) Option {
	return func(cfg *config) {
		if rps > 0 {
			cfg.requestsPerSecond = rps
		}
	}
}

// WithBurst sets the maximum burst size.
// Burst allows clients to make multiple requests instantly up to this limit.
// Default: 20 requests
//
// Example:
//
//	ratelimit.New(ratelimit.WithBurst(10))
func WithBurst(burst int) Option {
	return func(cfg *config) {
		if burst > 0 {
			cfg.burst = burst
		}
	}
}

// WithKeyFunc sets a custom function to extract the rate limit key from requests.
// Common use cases:
//   - Per-IP limiting: Use client IP (default)
//   - Per-user limiting: Use user ID from authentication
//   - Per-API key limiting: Use API key from header
//
// Example:
//
//	ratelimit.New(
//	    ratelimit.WithKeyFunc(func(c *router.Context) string {
//	        // Rate limit by user ID from auth token
//	        return c.Request.Header.Get("X-User-ID")
//	    }),
//	)
func WithKeyFunc(fn func(*router.Context) string) Option {
	return func(cfg *config) {
		cfg.keyFunc = fn
	}
}

// WithHandler sets a custom handler for when rate limit is exceeded.
// Default: Returns 429 Too Many Requests with JSON error
//
// Example:
//
//	ratelimit.New(
//	    ratelimit.WithHandler(func(c *router.Context) {
//	        c.String(http.StatusTooManyRequests, "Slow down! Try again in a minute.")
//	    }),
//	)
func WithHandler(fn func(*router.Context)) Option {
	return func(cfg *config) {
		cfg.onLimitExceeded = fn
	}
}

// WithCleanupInterval sets how often to clean up expired limiters.
// Default: 1 minute
func WithCleanupInterval(interval time.Duration) Option {
	return func(cfg *config) {
		if interval > 0 {
			cfg.cleanupInterval = interval
		}
	}
}

// WithLimiterTTL sets how long to keep inactive limiters before cleanup.
// Default: 5 minutes
func WithLimiterTTL(ttl time.Duration) Option {
	return func(cfg *config) {
		if ttl > 0 {
			cfg.limiterTTL = ttl
		}
	}
}

// WithLogger sets the slog.Logger for error logging.
// If not provided, errors will be silently ignored.
//
// Uses the standard library's log/slog package for structured logging:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	ratelimit.New(ratelimit.WithLogger(logger))
//
// Example:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r.Use(ratelimit.New(ratelimit.WithLogger(logger)))
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}
