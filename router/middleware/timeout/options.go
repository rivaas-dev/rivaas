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

import "rivaas.dev/router"

// WithHandler sets a custom handler for timeout errors.
// This handler is called when a request exceeds the timeout duration.
//
// Example:
//
//	timeout.New(30*time.Second,
//	    timeout.WithHandler(func(c *router.Context) {
//	        c.JSON(http.StatusRequestTimeout, map[string]string{
//	            "error": "Request took too long",
//	            "request_id": c.Response.Header().Get("X-Request-ID"),
//	        })
//	    }),
//	)
func WithHandler(handler func(c *router.Context)) Option {
	return func(cfg *config) {
		cfg.errorHandler = handler
	}
}

// WithSkipPaths sets paths that should not have timeout applied.
// Useful for long-running endpoints like streaming or webhooks.
//
// Example:
//
//	timeout.New(30*time.Second,
//	    timeout.WithSkipPaths("/stream", "/webhook"),
//	)
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}
