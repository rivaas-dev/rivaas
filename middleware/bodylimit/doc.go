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

// Package bodylimit provides middleware for limiting the size of HTTP request bodies,
// preventing abuse and protecting against large payload attacks.
//
// This middleware enforces a maximum request body size limit to prevent DoS attacks
// and protect server resources. Requests exceeding the limit are rejected before
// the body is fully read, saving bandwidth and memory.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/bodylimit"
//
//	r := router.MustNew()
//	r.Use(bodylimit.New(
//	    bodylimit.WithMaxSize(10 << 20), // 10MB
//	))
//
// # Configuration Options
//
//   - MaxSize: Maximum request body size in bytes (required)
//   - SkipPaths: Paths to exclude from body limiting (e.g., file upload endpoints)
//   - ErrorHandler: Custom handler for body limit exceeded errors
//
// # Error Handling
//
// When a request body exceeds the limit, the middleware returns a 413 Payload
// Too Large response. The error can be customized:
//
//	import "rivaas.dev/middleware/bodylimit"
//
//	r.Use(bodylimit.New(
//	    bodylimit.WithMaxSize(10 << 20),
//	    bodylimit.WithErrorHandler(func(c *router.Context, err error) {
//	        c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
//	            "error": "Request body too large",
//	            "max_size": "10MB",
//	        })
//	    }),
//	))
//
// # Security Considerations
//
// Body limiting is essential for preventing:
//
//   - DoS attacks via large payloads
//   - Memory exhaustion from oversized requests
//   - Bandwidth consumption attacks
//
// Set appropriate limits based on your API's needs. Consider different limits
// for different routes (e.g., file upload endpoints may need higher limits).
package bodylimit
