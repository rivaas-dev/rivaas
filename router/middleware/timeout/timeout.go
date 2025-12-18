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
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"rivaas.dev/router"
)

// Option defines functional options for timeout middleware configuration.
type Option func(*config)

// config holds the configuration for the timeout middleware.
type config struct {
	// duration is the timeout duration after which requests are canceled
	duration time.Duration

	// logger is used to log timeout events
	logger *slog.Logger

	// handler is called when a timeout occurs
	handler func(c *router.Context, timeout time.Duration)

	// skipPaths are exact paths that should not have timeout applied
	skipPaths map[string]bool

	// skipPrefixes are path prefixes that should not have timeout applied
	skipPrefixes []string

	// skipSuffixes are path suffixes that should not have timeout applied
	skipSuffixes []string

	// skipFunc is a custom function to determine if timeout should be skipped
	skipFunc func(c *router.Context) bool
}

// defaultConfig returns the default configuration for timeout middleware.
func defaultConfig() *config {
	return &config{
		duration:     30 * time.Second, // Sensible default
		logger:       slog.Default(),   // Logging enabled by default
		handler:      defaultHandler,
		skipPaths:    make(map[string]bool),
		skipPrefixes: nil,
		skipSuffixes: nil,
		skipFunc:     nil,
	}
}

// defaultHandler is the default timeout error handler.
func defaultHandler(c *router.Context, timeout time.Duration) {
	c.JSON(http.StatusRequestTimeout, map[string]any{
		"error":   "Request timeout",
		"code":    "TIMEOUT",
		"timeout": timeout.String(),
		"path":    c.Request.URL.Path,
	})
}

// shouldSkip determines if timeout should be skipped for the given request.
func shouldSkip(cfg *config, c *router.Context) bool {
	path := c.Request.URL.Path

	// Check exact paths
	if cfg.skipPaths[path] {
		return true
	}

	// Check prefixes
	for _, prefix := range cfg.skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Check suffixes
	for _, suffix := range cfg.skipSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	// Check custom function
	if cfg.skipFunc != nil && cfg.skipFunc(c) {
		return true
	}

	return false
}

// New returns a middleware that adds a timeout to requests.
// If a request takes longer than the specified duration, it will be canceled
// and an error response will be sent.
//
// The middleware creates a new context with timeout and passes it to the handler.
// Handlers should respect context cancellation to properly handle timeouts.
//
// Basic usage (uses 30s default):
//
//	r := router.MustNew()
//	r.Use(timeout.New())
//
// With custom duration:
//
//	r.Use(timeout.New(timeout.WithDuration(5 * time.Second)))
//
// With custom error handler:
//
//	r.Use(timeout.New(
//	    timeout.WithDuration(30 * time.Second),
//	    timeout.WithHandler(func(c *router.Context, timeout time.Duration) {
//	        c.JSON(http.StatusRequestTimeout, map[string]any{
//	            "error":   "Operation timed out",
//	            "timeout": timeout.String(),
//	        })
//	    }),
//	))
//
// Skip certain paths:
//
//	r.Use(timeout.New(
//	    timeout.WithSkipPaths("/stream", "/events"),
//	    timeout.WithSkipPrefix("/admin"),
//	    timeout.WithSkipSuffix("/ws"),
//	))
//
// Skip based on custom logic:
//
//	r.Use(timeout.New(
//	    timeout.WithSkip(func(c *router.Context) bool {
//	        return c.Request.Method == "OPTIONS"
//	    }),
//	))
//
// Disable logging:
//
//	r.Use(timeout.New(timeout.WithoutLogging()))
//
// Respecting timeouts in handlers:
//
//	r.GET("/slow", func(c *router.Context) {
//	    select {
//	    case <-time.After(2 * time.Second):
//	        c.JSON(http.StatusOK, map[string]string{"message": "Done"})
//	    case <-c.Request.Context().Done():
//	        // Request was canceled or timed out
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
//	If a timeout occurs, the context is canceled but the goroutine continues
//	until it naturally completes or checks c.Request.Context().Done().
//	This is expected behavior and handlers must be designed to respect context
//	cancellation to avoid goroutine leaks and unnecessary work.
//
// Panic handling:
//
//	Panics that occur within the handler goroutine are caught and re-thrown
//	in the main goroutine. This ensures the recovery middleware (which runs
//	in the main goroutine) can properly catch and handle panics.
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		// Check if timeout should be skipped
		if shouldSkip(cfg, c) {
			c.Next()
			return
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.duration)
		defer cancel()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Create channels for completion and panic propagation
		done := make(chan struct{})
		panicChan := make(chan any, 1)
		timedOut := false

		// Run the handler in a goroutine
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicChan <- r
				}
				close(done)
			}()
			c.Next()
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Check if there was a panic in the goroutine
			select {
			case p := <-panicChan:
				// Re-panic in main goroutine so recovery middleware can catch it
				panic(p)
			default:
				// Request completed normally
			}
		case <-ctx.Done():
			// Request timed out or was canceled
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				timedOut = true

				// Log timeout event
				if cfg.logger != nil {
					cfg.logger.Warn("request timeout",
						"method", c.Request.Method,
						"path", c.Request.URL.Path,
						"timeout", cfg.duration.String(),
					)
				}

				// Call timeout handler
				cfg.handler(c, cfg.duration)
			}
		}

		// CRITICAL: Wait for goroutine to fully complete to prevent race conditions
		// If timeout occurred, the goroutine might still be running and accessing c.Request
		// We must wait for it to finish before allowing the context to be returned to pool
		if timedOut {
			<-done
			// Check if handler panicked after timeout
			select {
			case p := <-panicChan:
				// Re-panic so recovery middleware can handle it
				panic(p)
			default:
				// No panic
			}
		}
	}
}
