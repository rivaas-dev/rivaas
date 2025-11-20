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

package methodoverride

import (
	"context"
	"strings"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

// CSRFVerifiedKey is the context key for CSRF verification status.
// Other middleware (e.g., CSRF middleware) should set this to true when CSRF is verified.
var CSRFVerifiedKey middleware.ContextKey = "middleware.csrf_verified"

// New creates a new HTTP method override middleware.
//
// This middleware allows clients to override the HTTP method using a header
// or query parameter, which is useful for HTML forms that only support GET/POST.
//
// SECURITY WARNING: This middleware should only be used when you control
// the client (e.g., HTML forms). Never enable for public APIs without
// WithRequireCSRFToken(true), as it can be exploited for CSRF attacks.
//
// Basic usage:
//
//	r.Use(methodoverride.New())
//
// With CSRF protection:
//
//	r.Use(csrf.Verify()) // Sets CSRF verification flag
//	r.Use(methodoverride.New(
//	    methodoverride.WithRequireCSRFToken(true),
//	    methodoverride.WithAllow("PUT", "PATCH", "DELETE"),
//	    methodoverride.WithOnlyOn("POST"),
//	))
//
// Custom header:
//
//	r.Use(methodoverride.New(
//	    methodoverride.WithHeader("X-HTTP-Method"),
//	))
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Build allow map for fast lookup
	allowMap := make(map[string]bool, len(cfg.allow))
	for _, m := range cfg.allow {
		allowMap[strings.ToUpper(m)] = true
	}

	// Build onlyOn map for fast lookup
	onlyOnMap := make(map[string]bool, len(cfg.onlyOn))
	for _, m := range cfg.onlyOn {
		onlyOnMap[strings.ToUpper(m)] = true
	}

	return func(c *router.Context) {
		originalMethod := c.Request.Method

		// Check if request method is in OnlyOn list
		if !onlyOnMap[strings.ToUpper(originalMethod)] {
			c.Next()
			return
		}

		// Check CSRF requirement
		if cfg.requireCSRFToken {
			if verified, ok := c.Request.Context().Value(CSRFVerifiedKey).(bool); !ok || !verified {
				// CSRF not verified, skip override
				c.Next()
				return
			}
		}

		// Try to get override method from header first
		overrideMethod := c.Request.Header.Get(cfg.header)
		if overrideMethod == "" && cfg.queryParam != "" {
			// Try query parameter
			overrideMethod = c.Request.URL.Query().Get(cfg.queryParam)
		}

		if overrideMethod == "" {
			c.Next()
			return
		}

		// Normalize method
		overrideMethod = strings.ToUpper(strings.TrimSpace(overrideMethod))

		// Check if method is in allowlist
		if !allowMap[overrideMethod] {
			c.Next()
			return
		}

		// Check body requirement
		if cfg.respectBody && c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		// Store original method in context for logging
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, middleware.OriginalMethodKey, originalMethod)
		c.Request = c.Request.WithContext(ctx)

		// Override method
		c.Request.Method = overrideMethod

		c.Next()
	}
}

// GetOriginalMethod retrieves the original HTTP method before override.
// Returns the current method if no override occurred.
func GetOriginalMethod(c *router.Context) string {
	if orig, ok := c.Request.Context().Value(middleware.OriginalMethodKey).(string); ok {
		return orig
	}
	return c.Request.Method
}
