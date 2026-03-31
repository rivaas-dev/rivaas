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

// Package middleware provides production-ready HTTP middleware for Go applications.
//
// The middleware package includes 12+ middleware components for common HTTP concerns:
//   - Access logging (rivaas.dev/middleware/accesslog)
//   - Basic authentication (rivaas.dev/middleware/basicauth)
//   - Body size limits (rivaas.dev/middleware/bodylimit)
//   - Compression (rivaas.dev/middleware/compression)
//   - CORS (rivaas.dev/middleware/cors)
//   - Method override (rivaas.dev/middleware/methodoverride)
//   - Rate limiting (rivaas.dev/middleware/ratelimit)
//   - Panic recovery (rivaas.dev/middleware/recovery)
//   - Request ID (rivaas.dev/middleware/requestid)
//   - Security headers (rivaas.dev/middleware/security)
//   - Request timeout (rivaas.dev/middleware/timeout)
//   - Trailing slash normalization (rivaas.dev/middleware/trailingslash)
//
// All middleware follows the same pattern and can be used with any HTTP handler.
// See individual middleware packages for detailed documentation.
//
// # Usage with Rivaas Router
//
//	import (
//	    "rivaas.dev/router"
//	    "rivaas.dev/middleware/cors"
//	    "rivaas.dev/middleware/recovery"
//	)
//
//	r := router.MustNew()
//	r.Use(cors.New())
//	r.Use(recovery.New())
//	r.GET("/", handler)
//
// # Usage with Standard Library
//
//	import (
//	    "net/http"
//	    "rivaas.dev/middleware/cors"
//	    "rivaas.dev/middleware/recovery"
//	)
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", handler)
//
//	// Wrap with middleware
//	handler := recovery.New()(cors.New()(mux))
//	http.ListenAndServe(":8080", handler)
//
// # Standalone Usage
//
// This package works independently without the full Rivaas framework. Use it
// with any Go HTTP handler (net/http, Gin, Echo, etc.).
package middleware
