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
//	import (
//	    "time"
//	    "rivaas.dev/router/middleware/timeout"
//	)
//
//	r := router.MustNew()
//	r.Use(timeout.New(
//	    timeout.WithTimeout(30 * time.Second),
//	))
//
// # Configuration Options
//
//   - Timeout: Maximum duration for request processing (required)
//   - ErrorHandler: Custom handler for timeout errors
//   - SkipPaths: Paths to exclude from timeout (e.g., long-running endpoints)
//
// # Timeout Behavior
//
// When a timeout occurs:
//
//   - The request context is canceled
//   - Handlers should check ctx.Done() and return early
//   - A 504 Gateway Timeout response is sent
//   - The error handler can customize the response
//
// # Custom Error Handler
//
//	import "rivaas.dev/router/middleware/timeout"
//
//	r.Use(timeout.New(
//	    timeout.WithTimeout(30 * time.Second),
//	    timeout.WithErrorHandler(func(c *router.Context) {
//	        c.JSON(http.StatusGatewayTimeout, map[string]string{
//	            "error": "Request timeout",
//	            "timeout": "30s",
//	        })
//	    }),
//	))
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
