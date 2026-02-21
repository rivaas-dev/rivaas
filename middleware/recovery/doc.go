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

// Package recovery provides middleware for recovering from panics in request handlers.
//
// This middleware catches panics that occur during request handling, logs them
// with stack traces, and returns a graceful error response instead of crashing
// the server. It integrates with OpenTelemetry to mark spans with exception
// information for observability.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/recovery"
//
//	r := router.MustNew()
//	r.Use(recovery.New())
//
// This middleware should typically be registered first (or early) in the middleware
// chain to catch panics from all subsequent handlers.
//
// # Configuration Options
//
//   - WithStackTrace: Enable/disable stack trace logging (default: true)
//   - WithStackSize: Maximum stack trace size in bytes (default: 4KB)
//   - WithLogger: Custom logger function for panic messages
//   - WithHandler: Custom recovery handler for error responses
//   - WithDisableStackAll: Disable full stack trace from all goroutines
//
// # Custom Recovery Handler
//
//	import "rivaas.dev/middleware/recovery"
//
//	r.Use(recovery.New(
//	    recovery.WithHandler(func(c *router.Context, err any) {
//	        c.JSON(http.StatusInternalServerError, map[string]any{
//	            "error": "Internal server error",
//	            "request_id": requestid.Get(c),
//	        })
//	    }),
//	))
//
// # OpenTelemetry Integration
//
// The middleware automatically marks OpenTelemetry spans with exception information:
//
//   - exception.escaped: Set to true for panics (only place this is set)
//   - exception.type: Type of the panic value
//   - exception.message: String representation of the panic value
package recovery
