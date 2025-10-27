package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
)

func main() {
	r := router.New()

	// Example 1: Basic security with secure defaults
	basicExample(r)

	// Example 2: Custom CSP for web applications
	webAppExample(r)

	// Example 3: API-specific security headers
	apiExample(r)

	// Example 4: Development vs Production
	environmentExample(r)

	fmt.Println("Server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  - GET /basic - Basic security headers")
	fmt.Println("  - GET /webapp - Web app with custom CSP")
	fmt.Println("  - GET /api/data - API with strict security")
	fmt.Println("  - GET /dev - Development security (relaxed)")
	fmt.Println("  - GET /prod - Production security (strict)")
	fmt.Println("")
	fmt.Println("Check headers with: curl -I http://localhost:8080/basic")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// Example 1: Basic security with secure defaults
func basicExample(r *router.Router) {
	// Use default secure headers
	r.Use(middleware.Security())

	r.GET("/basic", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Response with default security headers",
			"headers": "X-Frame-Options, X-Content-Type-Options, X-XSS-Protection, CSP, etc.",
		})
	})
}

// Example 2: Custom CSP for web applications
func webAppExample(r *router.Router) {
	webapp := r.Group("/webapp")

	// Web app with custom Content Security Policy
	webapp.Use(middleware.Security(
		middleware.WithFrameOptions("SAMEORIGIN"), // Allow embedding in same origin
		middleware.WithContentSecurityPolicy(
			// Allow scripts and styles from self and CDN
			"default-src 'self'; "+
				"script-src 'self' https://cdn.jsdelivr.net https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self' https://api.example.com",
		),
		middleware.WithReferrerPolicy("strict-origin-when-cross-origin"),
		middleware.WithPermissionsPolicy(
			// Restrict powerful browser features
			"geolocation=(), microphone=(), camera=(), payment=()",
		),
	))

	webapp.GET("", func(c *router.Context) {
		c.HTML(http.StatusOK, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Secure Web App</title>
				<link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto">
			</head>
			<body>
				<h1>Web Application with Security Headers</h1>
				<p>Check the response headers to see the CSP configuration.</p>
			</body>
			</html>
		`)
	})
}

// Example 3: API-specific security headers
func apiExample(r *router.Router) {
	api := r.Group("/api")

	// API with strict security
	api.Use(middleware.Security(
		middleware.WithFrameOptions("DENY"), // APIs don't need to be framed
		middleware.WithContentTypeNosniff(true),
		middleware.WithContentSecurityPolicy("default-src 'none'"), // APIs don't need CSP for rendering
		middleware.WithReferrerPolicy("no-referrer"),               // Don't leak referrer in API calls
		middleware.WithHSTS(63072000, true, false),                 // 2 years HSTS
	))

	api.GET("/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Secure API response",
			"data": map[string]string{
				"security": "strict",
				"headers":  "optimized for APIs",
			},
		})
	})
}

// Example 4: Environment-based configuration
func environmentExample(r *router.Router) {
	// Development security (relaxed for easier development)
	dev := r.Group("/dev")
	dev.Use(middleware.DevelopmentSecurity())

	dev.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"environment": "development",
			"security":    "relaxed (allows unsafe-inline, unsafe-eval)",
			"warning":     "Never use in production!",
		})
	})

	// Production security (strict)
	prod := r.Group("/prod")
	prod.Use(middleware.Security(
		middleware.WithFrameOptions("DENY"),
		middleware.WithContentSecurityPolicy(
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'",
		),
		middleware.WithHSTS(63072000, true, true), // 2 years, subdomains, preload
		middleware.WithReferrerPolicy("strict-origin-when-cross-origin"),
		middleware.WithPermissionsPolicy("geolocation=(), microphone=(), camera=()"),
	))

	prod.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"environment": "production",
			"security":    "strict (recommended for production)",
		})
	})
}

// Example showing all available options
func allOptionsExample() *router.Router {
	r := router.New()

	r.Use(middleware.Security(
		// X-Frame-Options: Controls if page can be embedded in iframe
		middleware.WithFrameOptions("DENY"), // or "SAMEORIGIN"

		// X-Content-Type-Options: Prevents MIME-sniffing
		middleware.WithContentTypeNosniff(true),

		// X-XSS-Protection: XSS filter (deprecated but still useful for old browsers)
		middleware.WithXSSProtection("1; mode=block"),

		// HSTS: Force HTTPS for specified duration
		middleware.WithHSTS(
			63072000, // 2 years in seconds
			true,     // includeSubDomains
			true,     // preload (for HSTS preload list)
		),

		// Content-Security-Policy: Comprehensive security policy
		middleware.WithContentSecurityPolicy(
			"default-src 'self'; "+
				"script-src 'self' https://trusted-cdn.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"connect-src 'self' https://api.example.com; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'",
		),

		// Referrer-Policy: Control referrer information
		middleware.WithReferrerPolicy("strict-origin-when-cross-origin"),

		// Permissions-Policy: Control browser features
		middleware.WithPermissionsPolicy(
			"geolocation=(), microphone=(), camera=(), payment=(), usb=()",
		),

		// Custom security headers
		middleware.WithCustomHeader("X-Custom-Security", "enabled"),
		middleware.WithCustomHeader("X-API-Version", "v1"),
	))

	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "All security options enabled",
		})
	})

	return r
}

// CSP Builder helper (in a real app, you might want this)
type CSPBuilder struct {
	directives map[string][]string
}

func NewCSPBuilder() *CSPBuilder {
	return &CSPBuilder{
		directives: make(map[string][]string),
	}
}

func (b *CSPBuilder) DefaultSrc(sources ...string) *CSPBuilder {
	b.directives["default-src"] = sources
	return b
}

func (b *CSPBuilder) ScriptSrc(sources ...string) *CSPBuilder {
	b.directives["script-src"] = sources
	return b
}

func (b *CSPBuilder) StyleSrc(sources ...string) *CSPBuilder {
	b.directives["style-src"] = sources
	return b
}

func (b *CSPBuilder) ImgSrc(sources ...string) *CSPBuilder {
	b.directives["img-src"] = sources
	return b
}

func (b *CSPBuilder) Build() string {
	var parts []string
	for directive, sources := range b.directives {
		parts = append(parts, directive+" "+strings.Join(sources, " "))
	}
	return strings.Join(parts, "; ")
}

func cspBuilderExample() {
	r := router.New()

	// Use CSP builder for cleaner configuration
	csp := NewCSPBuilder().
		DefaultSrc("'self'").
		ScriptSrc("'self'", "https://cdn.example.com").
		StyleSrc("'self'", "'unsafe-inline'").
		ImgSrc("'self'", "data:", "https:").
		Build()

	r.Use(middleware.Security(
		middleware.WithContentSecurityPolicy(csp),
	))

	r.GET("/", func(c *router.Context) {
		c.String(http.StatusOK, "CSP configured with builder pattern")
	})
}
