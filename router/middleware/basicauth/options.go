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

package basicauth

import "rivaas.dev/router"

// WithUsers sets the allowed username/password pairs.
// Passwords are compared using constant-time comparison to prevent timing attacks.
//
// Example:
//
//	basicauth.New(basicauth.WithUsers(map[string]string{
//	    "admin": "secret123",
//	    "user":  "password456",
//	}))
func WithUsers(users map[string]string) Option {
	return func(cfg *config) {
		cfg.users = users
	}
}

// WithRealm sets the authentication realm.
// The realm is displayed in the browser's authentication prompt.
// Default: "Restricted"
//
// Example:
//
//	basicauth.New(basicauth.WithRealm("Admin Area"))
func WithRealm(realm string) Option {
	return func(cfg *config) {
		cfg.realm = realm
	}
}

// WithValidator sets a custom validation function.
// This allows integration with databases, LDAP, or other authentication systems.
// When set, this takes precedence over the static users map.
//
// Example:
//
//	basicauth.New(basicauth.WithValidator(func(username, password string) bool {
//	    return db.ValidateUser(username, password)
//	}))
func WithValidator(validator func(username, password string) bool) Option {
	return func(cfg *config) {
		cfg.validator = validator
	}
}

// WithUnauthorizedHandler sets a custom handler for unauthorized requests.
// This allows custom error responses or redirects.
//
// Example:
//
//	basicauth.New(basicauth.WithUnauthorizedHandler(func(c *router.Context) {
//	    c.String(http.StatusUnauthorized, "Access denied")
//	}))
func WithUnauthorizedHandler(handler func(c *router.Context)) Option {
	return func(cfg *config) {
		cfg.unauthorizedHandler = handler
	}
}

// WithSkipPaths sets paths that should bypass authentication.
// Useful for health checks or public endpoints within protected groups.
//
// Example:
//
//	basicauth.New(basicauth.WithSkipPaths("/health", "/public"))
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}
