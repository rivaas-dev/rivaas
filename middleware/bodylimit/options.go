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

package bodylimit

import "rivaas.dev/router"

// WithLimit sets the maximum allowed body size in bytes.
// Default: 2MB (2 * 1024 * 1024 bytes)
//
// Example:
//
//	// Limit to 1MB
//	bodylimit.New(bodylimit.WithLimit(1024 * 1024))
//
//	// Limit to 10MB
//	bodylimit.New(bodylimit.WithLimit(10 * 1024 * 1024))
func WithLimit(size int64) Option {
	return func(cfg *config) {
		if size <= 0 {
			panic("body limit must be positive")
		}
		cfg.limit = size
	}
}

// WithErrorHandler sets a custom handler for when body limit is exceeded.
// The handler receives both the context and the configured limit for flexibility.
// Default: Returns 413 Request Entity Too Large with JSON error
//
// Example:
//
//	bodylimit.New(
//	    bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
//	        c.Stringf(http.StatusRequestEntityTooLarge, "Request body exceeds maximum allowed size of %d bytes", limit)
//	    }),
//	)
func WithErrorHandler(handler func(c *router.Context, limit int64)) Option {
	return func(cfg *config) {
		cfg.errorHandler = handler
	}
}

// WithSkipPaths sets paths that should not have body limit applied.
// Useful for endpoints that need to accept large uploads.
//
// Example:
//
//	bodylimit.New(
//	    bodylimit.WithSkipPaths("/upload", "/files"),
//	)
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}
