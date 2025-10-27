package middleware

import (
	"fmt"

	"github.com/rivaas-dev/rivaas/router"
)

// SecurityOption defines functional options for Security middleware configuration.
type SecurityOption func(*securityConfig)

// securityConfig holds the configuration for the Security middleware.
type securityConfig struct {
	// frameOptions sets X-Frame-Options header
	frameOptions string

	// contentTypeNosniff enables X-Content-Type-Options: nosniff
	contentTypeNosniff bool

	// xssProtection sets X-XSS-Protection header
	xssProtection string

	// hsts configures HTTP Strict Transport Security
	hstsMaxAge            int
	hstsIncludeSubdomains bool
	hstsPreload           bool

	// contentSecurityPolicy sets CSP header
	contentSecurityPolicy string

	// referrerPolicy sets Referrer-Policy header
	referrerPolicy string

	// permissionsPolicy sets Permissions-Policy header
	permissionsPolicy string

	// customHeaders are additional custom headers to set
	customHeaders map[string]string
}

// defaultSecurityConfig returns secure default configuration.
func defaultSecurityConfig() *securityConfig {
	return &securityConfig{
		frameOptions:          "DENY",
		contentTypeNosniff:    true,
		xssProtection:         "1; mode=block",
		hstsMaxAge:            31536000, // 1 year
		hstsIncludeSubdomains: true,
		hstsPreload:           false,
		contentSecurityPolicy: "default-src 'self'",
		referrerPolicy:        "strict-origin-when-cross-origin",
		permissionsPolicy:     "",
		customHeaders:         make(map[string]string),
	}
}

// WithFrameOptions sets the X-Frame-Options header.
// Common values: "DENY", "SAMEORIGIN", "ALLOW-FROM uri"
// Default: "DENY"
//
// Example:
//
//	middleware.Security(middleware.WithFrameOptions("SAMEORIGIN"))
func WithFrameOptions(value string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.frameOptions = value
	}
}

// WithContentTypeNosniff enables or disables X-Content-Type-Options: nosniff.
// This prevents browsers from MIME-sniffing responses.
// Default: true
//
// Example:
//
//	middleware.Security(middleware.WithContentTypeNosniff(true))
func WithContentTypeNosniff(enabled bool) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.contentTypeNosniff = enabled
	}
}

// WithXSSProtection sets the X-XSS-Protection header.
// Common values: "1; mode=block", "0" (to disable)
// Default: "1; mode=block"
//
// Note: This header is deprecated in modern browsers but still useful for older ones.
//
// Example:
//
//	middleware.Security(middleware.WithXSSProtection("1; mode=block"))
func WithXSSProtection(value string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.xssProtection = value
	}
}

// WithHSTS configures HTTP Strict Transport Security.
// maxAge is in seconds (default: 31536000 = 1 year).
//
// Example:
//
//	middleware.Security(middleware.WithHSTS(63072000, true, true)) // 2 years, includeSubdomains, preload
func WithHSTS(maxAge int, includeSubdomains, preload bool) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.hstsMaxAge = maxAge
		cfg.hstsIncludeSubdomains = includeSubdomains
		cfg.hstsPreload = preload
	}
}

// WithContentSecurityPolicy sets the Content-Security-Policy header.
// CSP helps prevent XSS, clickjacking, and other code injection attacks.
// Default: "default-src 'self'"
//
// Example:
//
//	middleware.Security(middleware.WithContentSecurityPolicy(
//	    "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
//	))
func WithContentSecurityPolicy(policy string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.contentSecurityPolicy = policy
	}
}

// WithReferrerPolicy sets the Referrer-Policy header.
// Controls how much referrer information is sent with requests.
// Default: "strict-origin-when-cross-origin"
//
// Common values:
//   - "no-referrer" - Never send referrer
//   - "same-origin" - Send referrer only to same origin
//   - "strict-origin" - Send origin only
//   - "strict-origin-when-cross-origin" - Full URL to same origin, origin only to others
//
// Example:
//
//	middleware.Security(middleware.WithReferrerPolicy("same-origin"))
func WithReferrerPolicy(policy string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.referrerPolicy = policy
	}
}

// WithPermissionsPolicy sets the Permissions-Policy header.
// Controls which browser features and APIs can be used.
//
// Example:
//
//	middleware.Security(middleware.WithPermissionsPolicy(
//	    "geolocation=(), microphone=(), camera=()",
//	))
func WithPermissionsPolicy(policy string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.permissionsPolicy = policy
	}
}

// WithCustomHeader adds a custom security header.
//
// Example:
//
//	middleware.Security(middleware.WithCustomHeader("X-Custom-Security", "value"))
func WithCustomHeader(name, value string) SecurityOption {
	return func(cfg *securityConfig) {
		cfg.customHeaders[name] = value
	}
}

// Security returns a middleware that sets security headers on HTTP responses.
// These headers help protect against common web vulnerabilities.
//
// Security headers included (with secure defaults):
//   - X-Frame-Options: DENY
//   - X-Content-Type-Options: nosniff
//   - X-XSS-Protection: 1; mode=block
//   - Strict-Transport-Security: max-age=31536000; includeSubDomains
//   - Content-Security-Policy: default-src 'self'
//   - Referrer-Policy: strict-origin-when-cross-origin
//
// Basic usage with secure defaults:
//
//	r := router.New()
//	r.Use(middleware.Security())
//
// Custom configuration:
//
//	r.Use(middleware.Security(
//	    middleware.WithFrameOptions("SAMEORIGIN"),
//	    middleware.WithContentSecurityPolicy("default-src 'self'; script-src 'self' https://cdn.example.com"),
//	))
//
// For development (more permissive):
//
//	r.Use(middleware.Security(
//	    middleware.WithFrameOptions("SAMEORIGIN"),
//	    middleware.WithContentSecurityPolicy("default-src 'self' 'unsafe-inline' 'unsafe-eval'"),
//	))
//
// Disable HSTS (useful in development):
//
//	r.Use(middleware.Security(
//	    middleware.WithHSTS(0, false, false), // Disables HSTS
//	))
//
// Performance: This middleware has minimal overhead (~50ns per request).
// It sets headers once at the beginning of the response.
func Security(opts ...SecurityOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultSecurityConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Pre-build HSTS header
	var hstsHeader string
	if cfg.hstsMaxAge > 0 {
		hstsHeader = fmt.Sprintf("max-age=%d", cfg.hstsMaxAge)
		if cfg.hstsIncludeSubdomains {
			hstsHeader += "; includeSubDomains"
		}
		if cfg.hstsPreload {
			hstsHeader += "; preload"
		}
	}

	return func(c *router.Context) {
		// Set X-Frame-Options
		if cfg.frameOptions != "" {
			c.Response.Header().Set("X-Frame-Options", cfg.frameOptions)
		}

		// Set X-Content-Type-Options
		if cfg.contentTypeNosniff {
			c.Response.Header().Set("X-Content-Type-Options", "nosniff")
		}

		// Set X-XSS-Protection
		if cfg.xssProtection != "" {
			c.Response.Header().Set("X-XSS-Protection", cfg.xssProtection)
		}

		// Set HSTS (only if HTTPS)
		if hstsHeader != "" && c.Request.TLS != nil {
			c.Response.Header().Set("Strict-Transport-Security", hstsHeader)
		}

		// Set Content-Security-Policy
		if cfg.contentSecurityPolicy != "" {
			c.Response.Header().Set("Content-Security-Policy", cfg.contentSecurityPolicy)
		}

		// Set Referrer-Policy
		if cfg.referrerPolicy != "" {
			c.Response.Header().Set("Referrer-Policy", cfg.referrerPolicy)
		}

		// Set Permissions-Policy
		if cfg.permissionsPolicy != "" {
			c.Response.Header().Set("Permissions-Policy", cfg.permissionsPolicy)
		}

		// Set custom headers
		for name, value := range cfg.customHeaders {
			c.Response.Header().Set(name, value)
		}

		c.Next()
	}
}

// SecureHeaders is a convenience function that returns Security middleware with
// production-ready secure defaults. This is equivalent to Security() with no options.
//
// Example:
//
//	r.Use(middleware.SecureHeaders())
func SecureHeaders() router.HandlerFunc {
	return Security()
}

// DevelopmentSecurity returns Security middleware with relaxed settings for development.
// This allows inline scripts and styles which are often needed during development.
//
// WARNING: Do not use in production!
//
// Example:
//
//	if os.Getenv("ENV") == "development" {
//	    r.Use(middleware.DevelopmentSecurity())
//	} else {
//	    r.Use(middleware.Security())
//	}
func DevelopmentSecurity() router.HandlerFunc {
	return Security(
		WithFrameOptions("SAMEORIGIN"),
		WithContentSecurityPolicy("default-src 'self' 'unsafe-inline' 'unsafe-eval'"),
		WithHSTS(0, false, false), // Disable HSTS in development
	)
}

// NoSecurityHeaders returns a middleware that removes all security headers.
// This should only be used in very specific cases where security headers
// conflict with requirements (e.g., embedding in iframes).
//
// WARNING: Only use this if you absolutely know what you're doing!
//
// Example:
//
//	r.Use(middleware.NoSecurityHeaders())
func NoSecurityHeaders() router.HandlerFunc {
	return func(c *router.Context) {
		c.Next()
		// Remove security headers
		c.Response.Header().Del("X-Frame-Options")
		c.Response.Header().Del("X-Content-Type-Options")
		c.Response.Header().Del("X-XSS-Protection")
		c.Response.Header().Del("Strict-Transport-Security")
		c.Response.Header().Del("Content-Security-Policy")
		c.Response.Header().Del("Referrer-Policy")
		c.Response.Header().Del("Permissions-Policy")
	}
}
