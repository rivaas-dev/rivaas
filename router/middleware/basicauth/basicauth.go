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

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

// Option defines functional options for basicauth middleware configuration.
type Option func(*config)

// config holds the configuration for the basicauth middleware.
type config struct {
	// users maps usernames to passwords
	users map[string]string

	// realm is the authentication realm shown to the user
	realm string

	// validator is a custom validation function
	validator func(username, password string) bool

	// unauthorizedHandler is called when authentication fails
	unauthorizedHandler func(c *router.Context)

	// skipPaths are paths that should bypass authentication
	skipPaths map[string]bool
}

// defaultConfig returns the default configuration for basicauth middleware.
func defaultConfig() *config {
	return &config{
		users:               make(map[string]string),
		realm:               "Restricted",
		validator:           nil,
		unauthorizedHandler: defaultUnauthorizedHandler,
		skipPaths:           make(map[string]bool),
	}
}

// defaultUnauthorizedHandler sends a 401 Unauthorized response.
func defaultUnauthorizedHandler(c *router.Context) {
	c.JSON(http.StatusUnauthorized, map[string]string{
		"error": "Unauthorized",
		"code":  "UNAUTHORIZED",
	})
}

// New returns a middleware that implements HTTP Basic Authentication (RFC 7617).
// It validates credentials from the Authorization header and denies access if invalid.
//
// Security considerations:
//   - Always use HTTPS in production - Basic Auth transmits credentials in base64 (not encrypted)
//   - Uses constant-time comparison to prevent timing attacks
//   - Does not cache credentials - validates on every request
//   - Realm is shown to users in browser authentication prompts
//
// Basic usage with static users:
//
//	r := router.MustNew()
//	r.Use(basicauth.New(
//	    basicauth.WithUsers(map[string]string{
//	        "admin": "secretpass",
//	    }),
//	))
//
// With custom realm:
//
//	r.Use(basicauth.New(
//	    basicauth.WithUsers(map[string]string{"user": "pass"}),
//	    basicauth.WithRealm("Admin Panel"),
//	))
//
// With custom validator (database lookup):
//
//	r.Use(basicauth.New(
//	    basicauth.WithValidator(func(username, password string) bool {
//	        user, err := db.GetUser(username)
//	        if err != nil {
//	            return false
//	        }
//	        return bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)) == nil
//	    }),
//	))
//
// Skip authentication for certain paths:
//
//	r.Use(basicauth.New(
//	    basicauth.WithUsers(map[string]string{"admin": "pass"}),
//	    basicauth.WithSkipPaths("/health", "/metrics"),
//	))
//
// Protect specific route groups:
//
//	r := router.MustNew()
//	admin := r.Group("/admin", basicauth.New(
//	    basicauth.WithUsers(map[string]string{"admin": "secret"}),
//	))
//	admin.GET("/dashboard", dashboardHandler)
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Pre-compute the WWW-Authenticate header
	authenticateHeader := `Basic realm="` + cfg.realm + `"`

	return func(c *router.Context) {
		// Check if path should be skipped
		if cfg.skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Get Authorization header
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			c.Abort()

			return
		}

		// Check if it's Basic auth
		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			c.Abort()

			return
		}

		// Decode base64 credentials
		decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			c.Abort()

			return
		}

		// Split username:password
		credentials := string(decoded)
		colonIndex := strings.IndexByte(credentials, ':')
		if colonIndex == -1 {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			c.Abort()

			return
		}

		username := credentials[:colonIndex]
		password := credentials[colonIndex+1:]

		// Validate credentials
		var authenticated bool
		if cfg.validator != nil {
			// Use custom validator
			authenticated = cfg.validator(username, password)
		} else {
			// Use static users map
			expectedPassword, exists := cfg.users[username]
			if exists {
				// Use constant-time comparison to prevent timing attacks
				authenticated = subtle.ConstantTimeCompare(
					[]byte(password),
					[]byte(expectedPassword),
				) == 1
			}
		}

		if !authenticated {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			c.Abort()

			return
		}

		// Authentication successful - store username in request context for later use
		ctx := context.WithValue(c.Request.Context(), middleware.AuthUsernameKey, username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetUsername retrieves the authenticated username from the request context.
// Returns an empty string if no authentication has occurred.
//
// Example:
//
//	func handler(c *router.Context) {
//	    username := basicauth.GetUsername(c)
//	    c.JSON(http.StatusOK, map[string]string{"user": username})
//	}
func GetUsername(c *router.Context) string {
	if username, ok := c.Request.Context().Value(middleware.AuthUsernameKey).(string); ok {
		return username
	}

	return ""
}
