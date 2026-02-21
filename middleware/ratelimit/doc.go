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

// Package ratelimit provides middleware for token bucket rate limiting per client.
//
// This middleware implements rate limiting using a token bucket algorithm to
// control the rate of requests from clients. It supports per-IP, per-user, or
// custom key-based rate limiting with configurable rate and burst limits.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/ratelimit"
//
//	r := router.MustNew()
//	r.Use(ratelimit.New(
//	    ratelimit.WithRequestsPerSecond(100),
//	    ratelimit.WithBurst(20),
//	))
//
// # Rate Limiting Strategies
//
// The middleware supports different rate limiting strategies:
//
//   - Per IP: Limit requests per client IP address (default)
//   - Per User: Limit requests per authenticated user
//   - Custom Key: Use a custom KeyFunc to determine the rate limit key
//
// # Configuration Options
//
//   - RequestsPerSecond: Average rate of requests allowed (required)
//   - Burst: Maximum burst size (default: same as RequestsPerSecond)
//   - KeyFunc: Function to determine rate limit key (default: per IP)
//   - SkipPaths: Paths to exclude from rate limiting (e.g., /health)
//   - Logger: Optional logger for rate limit events
//   - OnLimitExceeded: Custom handler when rate limit is exceeded
//
// # Custom Key Function
//
//	import "rivaas.dev/middleware/ratelimit"
//
//	r.Use(ratelimit.New(
//	    ratelimit.WithRequestsPerSecond(100),
//	    ratelimit.WithKeyFunc(func(c *router.Context) string {
//	        // Rate limit per user ID
//	        return c.Param("user_id")
//	    }),
//	))
//
// # Rate Limit Headers
//
// The middleware sets standard rate limit headers in responses:
//
//   - X-RateLimit-Limit: Maximum requests allowed per window
//   - X-RateLimit-Remaining: Remaining requests in current window
//   - X-RateLimit-Reset: Unix timestamp when the rate limit resets
//
// The token bucket algorithm supports concurrent access.
package ratelimit
