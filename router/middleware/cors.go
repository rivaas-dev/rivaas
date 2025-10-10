package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/rivaas-dev/rivaas/router"
)

// CORSOption defines functional options for CORS middleware configuration.
type CORSOption func(*corsConfig)

// corsConfig holds the configuration for the CORS middleware.
type corsConfig struct {
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

// defaultCORSConfig returns the default configuration for CORS middleware.
// Default configuration is restrictive for security.
func defaultCORSConfig() *corsConfig {
	return &corsConfig{
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

// WithAllowedOrigins sets the list of allowed origins.
// Use this for specific origins like ["https://example.com", "https://app.example.com"].
//
// Example:
//
//	middleware.CORS(middleware.WithAllowedOrigins([]string{"https://example.com"}))
func WithAllowedOrigins(origins []string) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowedOrigins = origins
		cfg.allowAllOrigins = false
	}
}

// WithAllowAllOrigins allows all origins by setting Access-Control-Allow-Origin: *.
// WARNING: This is insecure and should only be used for public APIs.
//
// Example:
//
//	middleware.CORS(middleware.WithAllowAllOrigins(true))
func WithAllowAllOrigins(allow bool) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowAllOrigins = allow
	}
}

// WithAllowedMethods sets the list of allowed HTTP methods.
// Default: ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]
//
// Example:
//
//	middleware.CORS(middleware.WithAllowedMethods([]string{"GET", "POST"}))
func WithAllowedMethods(methods []string) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowedMethods = methods
	}
}

// WithAllowedHeaders sets the list of allowed request headers.
// Default: ["Origin", "Content-Type", "Accept", "Authorization"]
//
// Example:
//
//	middleware.CORS(middleware.WithAllowedHeaders([]string{"Content-Type", "X-Custom-Header"}))
func WithAllowedHeaders(headers []string) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowedHeaders = headers
	}
}

// WithExposedHeaders sets the list of headers exposed to the client.
// These headers can be accessed by the client-side JavaScript.
//
// Example:
//
//	middleware.CORS(middleware.WithExposedHeaders([]string{"X-Request-ID", "X-Rate-Limit"}))
func WithExposedHeaders(headers []string) CORSOption {
	return func(cfg *corsConfig) {
		cfg.exposedHeaders = headers
	}
}

// WithAllowCredentials enables credentials (cookies, authorization headers, TLS certificates).
// When true, Access-Control-Allow-Origin cannot be "*".
// Default: false
//
// Example:
//
//	middleware.CORS(middleware.WithAllowCredentials(true))
func WithAllowCredentials(allow bool) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowCredentials = allow
	}
}

// WithMaxAge sets the max age for preflight cache in seconds.
// Default: 3600 (1 hour)
//
// Example:
//
//	middleware.CORS(middleware.WithMaxAge(7200)) // 2 hours
func WithMaxAge(seconds int) CORSOption {
	return func(cfg *corsConfig) {
		cfg.maxAge = seconds
	}
}

// WithAllowOriginFunc sets a custom function to validate origins dynamically.
// This is useful for pattern matching or database lookups.
//
// Example:
//
//	middleware.CORS(middleware.WithAllowOriginFunc(func(origin string) bool {
//	    return strings.HasSuffix(origin, ".example.com")
//	}))
func WithAllowOriginFunc(fn func(origin string) bool) CORSOption {
	return func(cfg *corsConfig) {
		cfg.allowOriginFunc = fn
	}
}

// CORS returns a middleware that handles Cross-Origin Resource Sharing (CORS).
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
//	r := router.New()
//	r.Use(middleware.CORS(
//	    middleware.WithAllowedOrigins([]string{"https://example.com"}),
//	))
//
// Allow all origins (public API):
//
//	r.Use(middleware.CORS(
//	    middleware.WithAllowAllOrigins(true),
//	))
//
// With credentials:
//
//	r.Use(middleware.CORS(
//	    middleware.WithAllowedOrigins([]string{"https://app.example.com"}),
//	    middleware.WithAllowCredentials(true),
//	))
//
// Dynamic origin validation:
//
//	r.Use(middleware.CORS(
//	    middleware.WithAllowOriginFunc(func(origin string) bool {
//	        return strings.HasSuffix(origin, ".example.com")
//	    }),
//	))
//
// Performance: This middleware has minimal overhead (~100ns per request).
// Preflight responses are cached by browsers according to maxAge setting.
func CORS(opts ...CORSOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultCORSConfig()
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
			for _, allowed := range cfg.allowedOrigins {
				if origin == allowed {
					allowedOrigin = origin
					break
				}
			}
		}

		// If origin is not allowed, continue without CORS headers
		if allowedOrigin == "" {
			c.Next()
			return
		}

		// Set CORS headers
		c.Response.Header().Set("Access-Control-Allow-Origin", allowedOrigin)

		if cfg.allowCredentials {
			// Cannot use wildcard with credentials
			if allowedOrigin == "*" {
				c.Response.Header().Set("Access-Control-Allow-Origin", origin)
			}
			c.Response.Header().Set("Access-Control-Allow-Credentials", "true")
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
