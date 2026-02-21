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
//	import "rivaas.dev/middleware/requestid"
//
//	r := router.MustNew()
//	r.Use(requestid.New())
//
// # Request ID Generation
//
// By default, UUID v7 is used for request ID generation. UUID v7 is time-ordered
// and lexicographically sortable (RFC 9562), making it ideal for debugging and
// log correlation. Generated IDs are 36-character UUID strings.
//
// The middleware generates request IDs using:
//
//   - X-Request-ID header: Uses existing header if present (for request tracing)
//   - UUID v7 generation: Time-ordered UUID if no header present (default)
//   - ULID generation: Compact 26-character alternative via [WithULID]
//
// # ID Format Comparison
//
//   - UUID v7 (default): 018f3e9a-1b2c-7def-8000-abcdef123456 (36 chars)
//   - ULID: 01ARZ3NDEKTSV4RRFFQ69G5FAV (26 chars)
//
// # Configuration Options
//
//   - [WithHeader]: Custom header name for request ID (default: X-Request-ID)
//   - [WithULID]: Use ULID instead of UUID v7 for shorter IDs
//   - [WithGenerator]: Custom function for generating request IDs
//   - [WithAllowClientID]: Control whether to accept client-provided IDs
//
// # Using ULID
//
// For shorter request IDs, use ULID:
//
//	r.Use(requestid.New(requestid.WithULID()))
//
// # Accessing Request ID
//
// The request ID is stored in the request context and can be retrieved:
//
//	import "rivaas.dev/middleware/requestid"
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
