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

package timeout

import (
	"context"
	"net/http"
	"time"

	"rivaas.dev/router"
)

// Option defines functional options for timeout middleware configuration.
type Option func(*config)

// config holds the configuration for the timeout middleware.
type config struct {
	// timeout is the duration after which requests are cancelled
	timeout time.Duration

	// errorHandler is called when a timeout occurs
	errorHandler func(c *router.Context)

	// skipPaths are paths that should not have timeout applied
	skipPaths map[string]bool
}

// defaultConfig returns the default configuration for timeout middleware.
func defaultConfig(timeout time.Duration) *config {
	return &config{
		timeout:      timeout,
		errorHandler: defaultErrorHandler,
		skipPaths:    make(map[string]bool),
	}
}

// defaultErrorHandler is the default timeout error handler.
func defaultErrorHandler(c *router.Context) {
	c.Status(http.StatusRequestTimeout)
	c.JSON(http.StatusRequestTimeout, map[string]any{
		"error": "Request timeout",
		"code":  "TIMEOUT",
	})
}

// New returns a middleware that adds a timeout to requests.
// If a request takes longer than the specified duration, it will be cancelled
// and an error response will be sent.
//
// The middleware creates a new context with timeout and passes it to the handler.
// Handlers should respect context cancellation to properly handle timeouts.
//
// Basic usage:
//
//	r := router.MustNew()
//	r.Use(timeout.New(30 * time.Second))
//
// With custom error handler:
//
//	r.Use(timeout.New(30*time.Second,
//	    timeout.WithHandler(func(c *router.Context) {
//	        c.JSON(http.StatusRequestTimeout, map[string]any{
//	            "error": "Operation timed out",
//	            "timeout": "30s",
//	        })
//	    }),
//	))
//
// Skip certain paths:
//
//	r.Use(timeout.New(30*time.Second,
//	    timeout.WithSkipPaths("/stream", "/events"),
//	))
//
// Respecting timeouts in handlers:
//
//	r.GET("/slow", func(c *router.Context) {
//	    select {
//	    case <-time.After(2 * time.Second):
//	        c.JSON(http.StatusOK, map[string]string{"message": "Done"})
//	    case <-c.Request.Context().Done():
//	        // Request was cancelled or timed out
//	        return
//	    }
//	})
//
// Important notes:
//   - Handlers MUST check c.Request.Context().Done() for long operations
//   - Database queries should use context: db.QueryContext(c.Request.Context(), ...)
//   - HTTP calls should use context: req.WithContext(c.Request.Context())
//   - Timeouts don't interrupt running code, they cancel the context
//   - Goroutines spawned by handlers may continue after timeout until they complete
//     or check context cancellation - this is a limitation of Go's timeout mechanism
//
// Timeout checking is handled by Go's context package.
//
// Goroutine behavior:
//
//	The timeout middleware spawns a goroutine to execute the handler chain.
//	If a timeout occurs, the context is cancelled but the goroutine continues
//	until it naturally completes or checks c.Request.Context().Done().
//	This is expected behavior and handlers must be designed to respect context
//	cancellation to avoid goroutine leaks and unnecessary work.
func New(timeout time.Duration, opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig(timeout)
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		// Check if path should skip timeout
		if cfg.skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.timeout)
		defer cancel()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Create a channel to signal completion
		done := make(chan struct{})
		timedOut := false

		// Run the handler in a goroutine
		go func() {
			c.Next()
			close(done)
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Request completed normally
		case <-ctx.Done():
			// Request timed out or was cancelled
			if ctx.Err() == context.DeadlineExceeded {
				timedOut = true
				cfg.errorHandler(c)
			}
		}

		// CRITICAL: Wait for goroutine to fully complete to prevent race conditions
		// If timeout occurred, the goroutine might still be running and accessing c.Request
		// We must wait for it to finish before allowing the context to be returned to pool
		if timedOut {
			<-done
		}
	}
}
