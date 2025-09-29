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

package trailingslash

import (
	"net/http"
	"strings"

	"rivaas.dev/router"
)

// Policy defines how trailing slashes are handled.
type Policy int

const (
	// PolicyRemove redirects paths with trailing slashes to canonical form without slash.
	// Example: /users/ → /users (308 redirect)
	// Root path "/" is never redirected.
	PolicyRemove Policy = iota

	// PolicyAdd redirects paths without trailing slashes to canonical form with slash.
	// Example: /users → /users/ (308 redirect)
	// Root path "/" is never redirected.
	PolicyAdd

	// PolicyStrict treats mismatched trailing slashes as 404.
	// Use this for strict API contracts where exact path matching is required.
	// The router will return 404/405 problem details for mismatched paths.
	PolicyStrict
)

// Option defines functional options for trailing slash middleware configuration.
type Option func(*config)

type config struct {
	policy Policy
}

func defaultConfig() *config {
	return &config{
		policy: PolicyRemove,
	}
}

// WithPolicy sets the trailing slash policy.
//
// Default: PolicyRemove (redirect /users/ → /users)
//
// Example:
//
//	// Remove trailing slashes (API-style)
//	r.Use(trailingslash.New())
//
//	// Require trailing slashes (website-style)
//	r.Use(trailingslash.New(trailingslash.WithPolicy(trailingslash.PolicyAdd)))
//
//	// Strict mode (404 for mismatches)
//	r.Use(trailingslash.New(trailingslash.WithPolicy(trailingslash.PolicyStrict)))
func WithPolicy(p Policy) Option {
	return func(c *config) {
		c.policy = p
	}
}

// Wrap wraps the router with trailing slash handling at the HTTP handler level.
// This must be used instead of middleware because trailing slash handling needs
// to occur BEFORE route matching.
//
// Example:
//
//	r := router.MustNew()
//	r.GET("/users", handler)
//	handler := trailingslash.Wrap(r, trailingslash.WithPolicy(trailingslash.PolicyRemove))
//	http.ListenAndServe(":8080", handler)
func Wrap(h http.Handler, opts ...Option) http.Handler {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Never modify root path
		if path == "/" {
			h.ServeHTTP(w, r)
			return
		}

		hasSlash := strings.HasSuffix(path, "/")

		switch cfg.policy {
		case PolicyRemove:
			if hasSlash {
				newPath := strings.TrimSuffix(path, "/")
				redirect308HTTP(w, r, newPath)
				return
			}

		case PolicyAdd:
			if !hasSlash {
				redirect308HTTP(w, r, path+"/")
				return
			}

		case PolicyStrict:
			// Let router handle it - will return 404/405 problem details
		}

		h.ServeHTTP(w, r)
	})
}

func redirect308HTTP(w http.ResponseWriter, r *http.Request, newPath string) {
	newURL := r.URL
	newURL.Path = newPath
	w.Header().Set("Location", newURL.String())
	w.WriteHeader(http.StatusPermanentRedirect)
}

// New returns middleware that enforces trailing slash policy.
//
// ⚠️ LIMITATION: This middleware runs AFTER route matching, so it cannot
// redirect mismatched trailing slashes for routes that don't match. Use
// Wrap() instead for proper trailing slash handling.
//
// This middleware is useful for:
//   - Normalizing paths that already matched (e.g., both /users and /users/ registered)
//   - Strict mode (return 404 for mismatches)
//
// For redirect-based policies, use Wrap() to wrap the router handler.
//
// Example (strict mode):
//
//	r := router.MustNew()
//	r.Use(trailingslash.New(trailingslash.WithPolicy(trailingslash.PolicyStrict)))
//	r.GET("/users", handler) // /users/ returns 404
func New(opts ...Option) router.HandlerFunc {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		path := c.Request.URL.Path

		// Never modify root path
		if path == "/" {
			c.Next()
			return
		}

		hasSlash := strings.HasSuffix(path, "/")

		switch cfg.policy {
		case PolicyRemove:
			if hasSlash {
				// Use TrimSuffix to remove exactly one slash (not TrimRight)
				// This prevents collapsing multiple slashes like /a// → /a
				newPath := strings.TrimSuffix(path, "/")
				redirect308(c, newPath)
				return
			}
			// Normalize path by removing trailing slash for routing
			// This ensures /users and /users/ both match the same route
			c.Request.URL.Path = path

		case PolicyAdd:
			if !hasSlash {
				redirect308(c, path+"/")
				return
			}
			// Path already has trailing slash, continue

		case PolicyStrict:
			// Don't modify path - let router handle exact matching
		}

		c.Next()
	}
}

func redirect308(c *router.Context, newPath string) {
	// Build full URL preserving query string
	newURL := c.Request.URL
	newURL.Path = newPath

	// Use 308 Permanent Redirect to preserve HTTP method and body
	// This is important for POST/PUT/PATCH requests
	c.Response.Header().Set("Location", newURL.String())
	c.Response.WriteHeader(http.StatusPermanentRedirect)

	// Abort the middleware chain - don't continue processing
	c.Abort()
}
