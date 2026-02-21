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

// Package cors provides middleware for handling Cross-Origin Resource Sharing (CORS),
// allowing configurable access control for cross-origin requests.
//
// This middleware implements the CORS specification to enable secure cross-origin
// requests from web browsers. It supports preflight requests, credentials, and
// fine-grained control over allowed origins, methods, headers, and exposed headers.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/cors"
//
//	r := router.MustNew()
//	r.Use(cors.New(
//	    cors.WithAllowedOrigins("https://example.com"),
//	    cors.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
//	    cors.WithAllowedHeaders("Content-Type", "Authorization"),
//	))
//
// # Configuration Options
//
// The middleware supports comprehensive CORS configuration:
//
//   - AllowedOrigins: List of allowed origin patterns (supports wildcards)
//   - AllowedMethods: HTTP methods allowed in cross-origin requests
//   - AllowedHeaders: Request headers allowed in cross-origin requests
//   - ExposedHeaders: Response headers exposed to the client
//   - AllowCredentials: Whether to allow credentials (cookies, auth headers)
//   - MaxAge: Cache duration for preflight requests
//   - OptionsPassthrough: Pass preflight requests to next handler
//
// # Security Considerations
//
// When using AllowCredentials, you must specify exact origins (no wildcards).
// The middleware validates this automatically to prevent security vulnerabilities.
//
// Preflight requests are handled with configurable caching.
package cors
