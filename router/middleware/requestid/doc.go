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

// Package requestid provides middleware for generating and tracking request IDs
// for distributed tracing and correlation.
//
// This middleware generates unique request IDs for each HTTP request and stores
// them in the request context. The ID is also included in response headers,
// allowing clients to correlate requests across services in distributed systems.
//
// # Basic Usage
//
//	import "rivaas.dev/router/middleware/requestid"
//
//	r := router.MustNew()
//	r.Use(requestid.New())
//
// # Request ID Generation
//
// The middleware generates request IDs using:
//
//   - X-Request-ID header: Uses existing header if present (for request tracing)
//   - Random generation: Cryptographically secure random ID if no header present
//
// Generated IDs are 16-byte values encoded as hexadecimal strings (32 characters).
//
// # Configuration Options
//
//   - HeaderName: Custom header name for request ID (default: X-Request-ID)
//   - Generator: Custom function for generating request IDs
//   - SkipPaths: Paths to exclude from request ID generation (e.g., /health)
//
// # Accessing Request ID
//
// The request ID is stored in the request context and can be retrieved:
//
//	import "rivaas.dev/router/middleware/requestid"
//
//	func handler(c *router.Context) {
//	    id := requestid.Get(c)
//	    // Use id for logging, tracing, etc.
//	}
//
// # Response Headers
//
// The middleware automatically includes the request ID in response headers,
// allowing clients to track requests across services.
//
// # Integration with Logging
//
// Request IDs are commonly used with structured logging for request correlation:
//
//	logger.Info("processing request",
//	    "request_id", requestid.Get(c),
//	    "method", c.Request.Method,
//	    "path", c.Request.URL.Path,
//	)
package requestid
