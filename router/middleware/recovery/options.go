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

import "rivaas.dev/router"

// WithStackTrace enables or disables stack trace printing.
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

// WithStackSize sets the maximum size of the stack trace buffer in bytes.
// Default: 4KB (4 << 10)
//
// Example:
//
//	recovery.New(recovery.WithStackSize(8 << 10)) // 8KB
func WithStackSize(size int) Option {
	return func(cfg *config) {
		cfg.stackSize = size
	}
}

// WithLogger sets a custom logger function for panic messages.
// The logger receives the context, error, and stack trace.
//
// Example:
//
//	recovery.New(recovery.WithLogger(func(c *router.Context, err any, stack []byte) {
//	    myLogger.Error("panic recovered", "error", err, "stack", string(stack))
//	}))
func WithLogger(logger func(c *router.Context, err any, stack []byte)) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}

// WithHandler sets a custom recovery handler function.
// The handler receives the context and error, and is responsible for sending the response.
//
// Example:
//
//	recovery.New(recovery.WithHandler(func(c *router.Context, err any) {
//	    c.JSON(http.StatusInternalServerError, map[string]string{"error": "Something went wrong"})
//	}))
func WithHandler(handler func(c *router.Context, err any)) Option {
	return func(cfg *config) {
		cfg.handler = handler
	}
}

// WithDisableStackAll disables capturing full stack trace from all goroutines.
// When enabled, only the current goroutine's stack is captured.
// Default: true
//
// Example:
//
//	recovery.New(recovery.WithDisableStackAll(false)) // Capture all goroutines
func WithDisableStackAll(disabled bool) Option {
	return func(cfg *config) {
		cfg.disableStackAll = disabled
	}
}
