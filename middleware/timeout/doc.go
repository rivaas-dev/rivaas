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

// Package timeout provides middleware for enforcing request timeouts to prevent
// long-running requests from consuming server resources.
//
// This middleware sets a deadline on the request context, causing handlers
// to be canceled if they exceed the configured timeout duration. This prevents
// slow or stuck handlers from consuming resources indefinitely.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/timeout"
//
//	r := router.MustNew()
//	r.Use(timeout.New())  // Uses 30s default timeout
//
// # With Custom Duration
//
//	r.Use(timeout.New(timeout.WithDuration(5 * time.Second)))
//
// # Configuration Options
//
//   - Duration: Maximum duration for request processing (default: 30s)
//   - Logger: Custom slog.Logger for timeout events (default: slog.Default())
//   - Handler: Custom handler for timeout errors
//   - SkipPaths: Exact paths to exclude from timeout
//   - SkipPrefix: Path prefixes to exclude from timeout
//   - SkipSuffix: Path suffixes to exclude from timeout
//   - Skip: Custom function to determine if timeout should be skipped
//
// # Timeout Behavior
//
// When a timeout occurs:
//
//   - The request context is canceled
//   - A warning is logged (unless disabled with WithoutLogging())
//   - A 408 Request Timeout response is sent
//   - Handlers should check ctx.Done() and return early
//
// # Skip Paths
//
//	// Skip exact paths
//	r.Use(timeout.New(
//	    timeout.WithSkipPaths("/stream", "/webhook"),
//	))
//
//	// Skip by prefix (all /admin/* routes)
//	r.Use(timeout.New(
//	    timeout.WithSkipPrefix("/admin", "/internal"),
//	))
//
//	// Skip by suffix (all streaming endpoints)
//	r.Use(timeout.New(
//	    timeout.WithSkipSuffix("/stream", "/events"),
//	))
//
//	// Skip with custom logic
//	r.Use(timeout.New(
//	    timeout.WithSkip(func(c *router.Context) bool {
//	        return c.Request.Method == "OPTIONS"
//	    }),
//	))
//
// # Custom Error Handler
//
//	r.Use(timeout.New(
//	    timeout.WithDuration(30 * time.Second),
//	    timeout.WithHandler(func(c *router.Context, timeout time.Duration) {
//	        c.JSON(http.StatusRequestTimeout, map[string]any{
//	            "error":   "Request timeout",
//	            "timeout": timeout.String(),
//	        })
//	    }),
//	))
//
// # Disable Logging
//
//	r.Use(timeout.New(timeout.WithoutLogging()))
//
// # Handler Implementation
//
// Handlers should respect context cancellation:
//
//	func handler(c *router.Context) {
//	    ctx := c.Request.Context()
//	    select {
//	    case <-ctx.Done():
//	        return // Timeout occurred
//	    case result := <-longRunningOperation(ctx):
//	        c.JSON(http.StatusOK, result)
//	    }
//	}
//
// Timeout enforcement uses context.WithTimeout, which is standard Go practice.
package timeout
