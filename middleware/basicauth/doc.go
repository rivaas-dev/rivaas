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

// Package basicauth provides HTTP Basic Authentication middleware with
// configurable user validation and realm support.
//
// This middleware implements HTTP Basic Authentication (RFC 7617) for protecting
// routes with username/password authentication. It validates credentials using
// a configurable validator function and stores the authenticated username in
// the request context for use by handlers.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/basicauth"
//
//	r := router.MustNew()
//	r.Use(basicauth.New(
//	    basicauth.WithValidator(func(username, password string) bool {
//	        return username == "admin" && password == "secret"
//	    }),
//	    basicauth.WithRealm("Restricted Area"),
//	))
//
// # Configuration Options
//
//   - Validator: Function to validate username/password credentials
//   - Realm: Authentication realm name (displayed in browser prompt)
//   - SkipPaths: Paths to skip authentication (e.g., /health, /public)
//
// # Accessing Authenticated User
//
// The authenticated username is stored in the request context and can be
// retrieved using the Username function:
//
//	import "rivaas.dev/middleware/basicauth"
//
//	func handler(c *router.Context) {
//	    username := basicauth.Username(c)
//	    if username == "" {
//	        c.JSON(http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
//	        return
//	    }
//	    // Use username...
//	}
//
// # Security Considerations
//
// Basic Authentication sends credentials in base64-encoded form with each request.
// Always use HTTPS in production to protect credentials in transit. Consider
// using more secure authentication methods (OAuth2, JWT) for production APIs.
package basicauth
