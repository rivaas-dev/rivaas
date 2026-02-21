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

package cors

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"rivaas.dev/router"
)

// Option defines functional options for cors middleware configuration.
type Option func(*config)

// config holds the configuration for the cors middleware.
type config struct {
	// allowedOrigins is the list of allowed origins for CORS requests
	allowedOrigins []string

	// allowedMethods is the list of allowed HTTP methods
	allowedMethods []string

	// allowedHeaders is the list of allowed request headers
	allowedHeaders []string

	// exposedHeaders is the list of headers exposed to the client
	exposedHeaders []string

	// allowCredentials indicates whether credentials are allowed
	allowCredentials bool

	// maxAge is the max age for preflight cache in seconds
	maxAge int

	// allowAllOrigins allows all origins (sets Access-Control-Allow-Origin: *)
	allowAllOrigins bool

	// allowOriginFunc is a custom function to validate origins
	allowOriginFunc func(origin string) bool
}

// defaultConfig returns the default configuration for cors middleware.
// Default configuration is restrictive for security.
func defaultConfig() *config {
	return &config{
		allowedOrigins:   []string{},
		allowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		allowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
		exposedHeaders:   []string{},
		allowCredentials: false,
		maxAge:           3600, // 1 hour
		allowAllOrigins:  false,
		allowOriginFunc:  nil,
	}
}

// New returns a middleware that handles Cross-Origin Resource Sharing (CORS).
// It automatically handles preflight requests and sets appropriate CORS headers.
//
// Security considerations:
//   - Default configuration is restrictive (no origins allowed by default)
//   - Use WithAllowedOrigins() to specify exact origins
//   - Avoid WithAllowAllOrigins() unless building a public API
//   - When using credentials, cannot use wildcard origins
//
// Basic usage:
//
//	r := router.MustNew()
//	r.Use(cors.New(
//	    cors.WithAllowedOrigins("https://example.com"),
//	))
//
// Allow all origins (public API):
//
//	r.Use(cors.New(
//	    cors.WithAllowAllOrigins(true),
//	))
//
// With credentials:
//
//	r.Use(cors.New(
//	    cors.WithAllowedOrigins("https://app.example.com"),
//	    cors.WithAllowCredentials(true),
//	))
//
// Dynamic origin validation:
//
//	r.Use(cors.New(
//	    cors.WithAllowOriginFunc(func(origin string) bool {
//	        return strings.HasSuffix(origin, ".example.com")
//	    }),
//	))
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Pre-compute common headers for performance
	allowedMethodsHeader := strings.Join(cfg.allowedMethods, ", ")
	allowedHeadersHeader := strings.Join(cfg.allowedHeaders, ", ")
	exposedHeadersHeader := ""
	if len(cfg.exposedHeaders) > 0 {
		exposedHeadersHeader = strings.Join(cfg.exposedHeaders, ", ")
	}
	maxAgeHeader := strconv.Itoa(cfg.maxAge)

	return func(c *router.Context) {
		origin := c.Request.Header.Get("Origin")

		// If no origin header, this is not a CORS request
		if origin == "" {
			c.Next()
			return
		}

		// Determine if origin is allowed
		allowedOrigin := ""
		if cfg.allowAllOrigins {
			allowedOrigin = "*"
		} else if cfg.allowOriginFunc != nil {
			if cfg.allowOriginFunc(origin) {
				allowedOrigin = origin
			}
		} else {
			// Check if origin is in allowed list
			if slices.Contains(cfg.allowedOrigins, origin) {
				allowedOrigin = origin
			}
		}

		// If origin is not allowed, continue without CORS headers
		if allowedOrigin == "" {
			c.Next()
			return
		}

		// Set CORS headers
		// Handle credentials + wildcard incompatibility first
		if cfg.allowCredentials && allowedOrigin == "*" {
			// Cannot use wildcard with credentials - use specific origin instead
			c.Response.Header().Set("Access-Control-Allow-Origin", origin)
			c.Response.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			// Normal case: set allowed origin
			c.Response.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			if cfg.allowCredentials {
				c.Response.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if exposedHeadersHeader != "" {
			c.Response.Header().Set("Access-Control-Expose-Headers", exposedHeadersHeader)
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.Response.Header().Set("Access-Control-Allow-Methods", allowedMethodsHeader)
			c.Response.Header().Set("Access-Control-Allow-Headers", allowedHeadersHeader)
			c.Response.Header().Set("Access-Control-Max-Age", maxAgeHeader)

			// Preflight successful, return 204 No Content
			c.Response.WriteHeader(http.StatusNoContent)

			return
		}

		// Continue with actual request
		c.Next()
	}
}
