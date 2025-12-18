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

// Package recovery provides middleware for recovering from panics in HTTP handlers,
// preventing server crashes and returning proper error responses.
package recovery

import (
	"log/slog"

	"rivaas.dev/router"
)

// WithoutLogging disables panic logging.
// Useful for tests to avoid noisy output and race conditions.
//
// Example:
//
//	recovery.New(recovery.WithoutLogging())
func WithoutLogging() Option {
	return func(cfg *config) {
		cfg.logger = nil
	}
}

// WithLogger sets a custom slog.Logger for panic logging.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	recovery.New(recovery.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}

// WithHandler sets a custom recovery handler for sending error responses.
//
// Example:
//
//	recovery.New(recovery.WithHandler(func(c *router.Context, err any) {
//	    c.JSON(http.StatusInternalServerError, map[string]any{
//	        "error":      "Something went wrong",
//	        "request_id": c.Header("X-Request-ID"),
//	    })
//	}))
func WithHandler(handler func(c *router.Context, err any)) Option {
	return func(cfg *config) {
		cfg.handler = handler
	}
}

// WithStackTrace enables or disables stack trace capture.
// Default: true
//
// Example:
//
//	recovery.New(recovery.WithStackTrace(false))
func WithStackTrace(enabled bool) Option {
	return func(cfg *config) {
		cfg.stackTrace = enabled
	}
}

// WithStackSize sets the maximum size of the stack trace in bytes.
// Default: 4KB
//
// Example:
//
//	recovery.New(recovery.WithStackSize(8 << 10)) // 8KB
func WithStackSize(size int) Option {
	return func(cfg *config) {
		cfg.stackSize = size
	}
}

// WithPrettyStack controls whether stack traces are pretty-printed.
// By default (nil), stack traces are auto-detected:
//   - Pretty-printed to stderr when running in a terminal (TTY)
//   - Compact JSON when piped/redirected (production logs)
//
// Example:
//
//	// Force pretty printing (useful for development)
//	recovery.New(recovery.WithPrettyStack(true))
//
//	// Force compact output (useful for CI/testing)
//	recovery.New(recovery.WithPrettyStack(false))
func WithPrettyStack(enabled bool) Option {
	return func(cfg *config) {
		cfg.prettyStack = &enabled
	}
}
