package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/rivaas-dev/rivaas/router"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

// authUsernameKey is the context key for storing authenticated username.
const authUsernameKey contextKey = "auth.username"

// BasicAuthOption defines functional options for BasicAuth middleware configuration.
type BasicAuthOption func(*basicAuthConfig)

// basicAuthConfig holds the configuration for the BasicAuth middleware.
type basicAuthConfig struct {
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

// defaultBasicAuthConfig returns the default configuration for BasicAuth middleware.
func defaultBasicAuthConfig() *basicAuthConfig {
	return &basicAuthConfig{
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

// WithBasicAuthUsers sets the allowed username/password pairs.
// Passwords are compared using constant-time comparison to prevent timing attacks.
//
// Example:
//
//	middleware.BasicAuth(middleware.WithBasicAuthUsers(map[string]string{
//	    "admin": "secret123",
//	    "user":  "password456",
//	}))
func WithBasicAuthUsers(users map[string]string) BasicAuthOption {
	return func(cfg *basicAuthConfig) {
		cfg.users = users
	}
}

// WithBasicAuthRealm sets the authentication realm.
// The realm is displayed in the browser's authentication prompt.
// Default: "Restricted"
//
// Example:
//
//	middleware.BasicAuth(middleware.WithBasicAuthRealm("Admin Area"))
func WithBasicAuthRealm(realm string) BasicAuthOption {
	return func(cfg *basicAuthConfig) {
		cfg.realm = realm
	}
}

// WithBasicAuthValidator sets a custom validation function.
// This allows integration with databases, LDAP, or other authentication systems.
// When set, this takes precedence over the static users map.
//
// Example:
//
//	middleware.BasicAuth(middleware.WithBasicAuthValidator(func(username, password string) bool {
//	    return db.ValidateUser(username, password)
//	}))
func WithBasicAuthValidator(validator func(username, password string) bool) BasicAuthOption {
	return func(cfg *basicAuthConfig) {
		cfg.validator = validator
	}
}

// WithBasicAuthUnauthorizedHandler sets a custom handler for unauthorized requests.
// This allows custom error responses or redirects.
//
// Example:
//
//	middleware.BasicAuth(middleware.WithBasicAuthUnauthorizedHandler(func(c *router.Context) {
//	    c.String(http.StatusUnauthorized, "Access denied")
//	}))
func WithBasicAuthUnauthorizedHandler(handler func(c *router.Context)) BasicAuthOption {
	return func(cfg *basicAuthConfig) {
		cfg.unauthorizedHandler = handler
	}
}

// WithBasicAuthSkipPaths sets paths that should bypass authentication.
// Useful for health checks or public endpoints within protected groups.
//
// Example:
//
//	middleware.BasicAuth(middleware.WithBasicAuthSkipPaths([]string{"/health", "/public"}))
func WithBasicAuthSkipPaths(paths []string) BasicAuthOption {
	return func(cfg *basicAuthConfig) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}

// BasicAuth returns a middleware that implements HTTP Basic Authentication (RFC 7617).
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
//	r := router.New()
//	r.Use(middleware.BasicAuth(
//	    middleware.WithBasicAuthUsers(map[string]string{
//	        "admin": "secretpass",
//	    }),
//	))
//
// With custom realm:
//
//	r.Use(middleware.BasicAuth(
//	    middleware.WithBasicAuthUsers(map[string]string{"user": "pass"}),
//	    middleware.WithBasicAuthRealm("Admin Panel"),
//	))
//
// With custom validator (database lookup):
//
//	r.Use(middleware.BasicAuth(
//	    middleware.WithBasicAuthValidator(func(username, password string) bool {
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
//	r.Use(middleware.BasicAuth(
//	    middleware.WithBasicAuthUsers(map[string]string{"admin": "pass"}),
//	    middleware.WithBasicAuthSkipPaths([]string{"/health", "/metrics"}),
//	))
//
// Protect specific route groups:
//
//	r := router.New()
//	admin := r.Group("/admin", middleware.BasicAuth(
//	    middleware.WithBasicAuthUsers(map[string]string{"admin": "secret"}),
//	))
//	admin.GET("/dashboard", dashboardHandler)
//
// Performance: This middleware has minimal overhead (~500ns per request).
// The actual validation cost depends on the validator function used.
func BasicAuth(opts ...BasicAuthOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultBasicAuthConfig()
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
			return
		}

		// Check if it's Basic auth
		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			return
		}

		// Decode base64 credentials
		decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
			return
		}

		// Split username:password
		credentials := string(decoded)
		colonIndex := strings.IndexByte(credentials, ':')
		if colonIndex == -1 {
			c.Response.Header().Set("WWW-Authenticate", authenticateHeader)
			cfg.unauthorizedHandler(c)
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
			return
		}

		// Authentication successful - store username in request context for later use
		ctx := context.WithValue(c.Request.Context(), authUsernameKey, username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetAuthUsername retrieves the authenticated username from the request context.
// Returns an empty string if no authentication has occurred.
//
// Example:
//
//	func handler(c *router.Context) {
//	    username := middleware.GetAuthUsername(c)
//	    c.JSON(200, map[string]string{"user": username})
//	}
func GetAuthUsername(c *router.Context) string {
	if username, ok := c.Request.Context().Value(authUsernameKey).(string); ok {
		return username
	}
	return ""
}
