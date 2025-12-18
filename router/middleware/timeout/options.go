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

// Package timeout provides middleware for setting request timeouts.
package timeout

import (
	"log/slog"
	"time"

	"rivaas.dev/router"
)

// WithDuration sets the timeout duration.
// Default: 30 seconds
//
// Example:
//
//	timeout.New(timeout.WithDuration(5 * time.Second))
func WithDuration(d time.Duration) Option {
	return func(cfg *config) {
		cfg.duration = d
	}
}

// WithoutLogging disables timeout logging.
// By default, timeouts are logged using slog.Default().
//
// Example:
//
//	timeout.New(timeout.WithoutLogging())
func WithoutLogging() Option {
	return func(cfg *config) {
		cfg.logger = nil
	}
}

// WithLogger sets a custom slog.Logger for timeout logging.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	timeout.New(timeout.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}

// WithHandler sets a custom handler for timeout errors.
// This handler is called when a request exceeds the timeout duration.
// The handler receives the configured timeout duration.
//
// Example:
//
//	timeout.New(
//	    timeout.WithHandler(func(c *router.Context, timeout time.Duration) {
//	        c.JSON(http.StatusRequestTimeout, map[string]any{
//	            "error":      "Request took too long",
//	            "timeout":    timeout.String(),
//	            "request_id": c.Response.Header().Get("X-Request-ID"),
//	        })
//	    }),
//	)
func WithHandler(handler func(c *router.Context, timeout time.Duration)) Option {
	return func(cfg *config) {
		cfg.handler = handler
	}
}

// WithSkipPaths sets exact paths that should not have timeout applied.
// Useful for long-running endpoints like streaming or webhooks.
//
// Example:
//
//	timeout.New(timeout.WithSkipPaths("/stream", "/webhook"))
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}

// WithSkipPrefix skips paths that start with any of the given prefixes.
// Useful for skipping entire route groups.
//
// Example:
//
//	timeout.New(timeout.WithSkipPrefix("/admin", "/internal"))
func WithSkipPrefix(prefixes ...string) Option {
	return func(cfg *config) {
		cfg.skipPrefixes = append(cfg.skipPrefixes, prefixes...)
	}
}

// WithSkipSuffix skips paths that end with any of the given suffixes.
// Useful for skipping specific endpoint types.
//
// Example:
//
//	timeout.New(timeout.WithSkipSuffix("/stream", "/events"))
func WithSkipSuffix(suffixes ...string) Option {
	return func(cfg *config) {
		cfg.skipSuffixes = append(cfg.skipSuffixes, suffixes...)
	}
}

// WithSkip sets a custom function to determine if timeout should be skipped.
// Return true to skip timeout for the request.
//
// Example:
//
//	timeout.New(
//	    timeout.WithSkip(func(c *router.Context) bool {
//	        // Skip OPTIONS requests
//	        if c.Request.Method == "OPTIONS" {
//	            return true
//	        }
//	        // Skip if header present
//	        return c.Request.Header.Get("X-No-Timeout") != ""
//	    }),
//	)
func WithSkip(fn func(c *router.Context) bool) Option {
	return func(cfg *config) {
		cfg.skipFunc = fn
	}
}
