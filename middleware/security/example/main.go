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

// Package main demonstrates how to use the security middleware
// to add security headers to HTTP responses.
package main

import (
	"log"
	"net/http"
	"strings"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/security"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic security with secure defaults
	basicExample(r)

	// Example 2: Custom CSP for web applications
	webAppExample(r)

	// Example 3: API-specific security headers
	apiExample(r)

	// Example 4: Development vs Production
	environmentExample(r)

	// Example 5: All available options (reference)
	allOptionsExample(r)

	// Example 6: CSP builder pattern
	cspBuilderExample(r)

	log.Println("Server starting on http://localhost:8080")
	log.Println("Endpoints: /basic /webapp /api/data /dev /prod /all /builder")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Basic security with secure defaults
func basicExample(r *router.Router) {
	// Use default secure headers
	r.Use(security.New())

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
	webapp.Use(security.New(
		security.WithFrameOptions("SAMEORIGIN"), // Allow embedding in same origin
		security.WithContentSecurityPolicy(
			// Allow scripts and styles from self and CDN
			"default-src 'self'; "+
				"script-src 'self' https://cdn.jsdelivr.net https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self' https://api.example.com",
		),
		security.WithReferrerPolicy("strict-origin-when-cross-origin"),
		security.WithPermissionsPolicy(
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
	api.Use(security.New(
		security.WithFrameOptions("DENY"), // APIs don't need to be framed
		security.WithContentTypeNosniff(true),
		security.WithContentSecurityPolicy("default-src 'none'"), // APIs don't need CSP for rendering
		security.WithReferrerPolicy("no-referrer"),               // Don't leak referrer in API calls
		security.WithHSTS(63072000, true, false),                 // 2 years HSTS
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
	dev.Use(security.New(
		security.WithFrameOptions("SAMEORIGIN"),
		security.WithContentSecurityPolicy("default-src 'self' 'unsafe-inline' 'unsafe-eval'"),
		security.WithHSTS(0, false, false), // Disable HSTS in development
	))

	dev.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"environment": "development",
			"security":    "relaxed (allows unsafe-inline, unsafe-eval)",
			"warning":     "Never use in production!",
		})
	})

	// Production security (strict)
	prod := r.Group("/prod")
	prod.Use(security.New(
		security.WithFrameOptions("DENY"),
		security.WithContentSecurityPolicy(
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'",
		),
		security.WithHSTS(63072000, true, true), // 2 years, subdomains, preload
		security.WithReferrerPolicy("strict-origin-when-cross-origin"),
		security.WithPermissionsPolicy("geolocation=(), microphone=(), camera=()"),
	))

	prod.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"environment": "production",
			"security":    "strict (recommended for production)",
		})
	})
}

// Example 5: All available options (reference)
func allOptionsExample(r *router.Router) {
	all := r.Group("/all")

	// Demonstrates all available security options:
	//   - X-Frame-Options (DENY/SAMEORIGIN)
	//   - X-Content-Type-Options (MIME-sniffing protection)
	//   - X-XSS-Protection (legacy browser support)
	//   - HSTS (HTTP Strict Transport Security)
	//   - Content-Security-Policy (comprehensive CSP)
	//   - Referrer-Policy
	//   - Permissions-Policy (browser feature control)
	//   - Custom security headers
	all.Use(security.New(
		// X-Frame-Options: Controls if page can be embedded in iframe
		security.WithFrameOptions("DENY"), // or "SAMEORIGIN"

		// X-Content-Type-Options: Prevents MIME-sniffing
		security.WithContentTypeNosniff(true),

		// X-XSS-Protection: XSS filter (deprecated but still useful for old browsers)
		security.WithXSSProtection("1; mode=block"),

		// HSTS: Force HTTPS for specified duration
		security.WithHSTS(
			63072000, // 2 years in seconds
			true,     // includeSubDomains
			true,     // preload (for HSTS preload list)
		),

		// Content-Security-Policy: Comprehensive security policy
		security.WithContentSecurityPolicy(
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
		security.WithReferrerPolicy("strict-origin-when-cross-origin"),

		// Permissions-Policy: Control browser features
		security.WithPermissionsPolicy(
			"geolocation=(), microphone=(), camera=(), payment=(), usb=()",
		),

		// Custom security headers
		security.WithCustomHeader("X-Custom-Security", "enabled"),
		security.WithCustomHeader("X-API-Version", "v1"),
	))

	all.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "All security options enabled",
			"note":    "Check response headers to see all options",
		})
	})
}

// CSPBuilder is a helper for building Content Security Policy directives.
// In a real app, you might want this.
type CSPBuilder struct {
	directives map[string][]string
}

// NewCSPBuilder creates a new CSPBuilder instance.
func NewCSPBuilder() *CSPBuilder {
	return &CSPBuilder{
		directives: make(map[string][]string),
	}
}

// DefaultSrc sets the default-src directive for the Content Security Policy.
// This directive serves as a fallback for other fetch directives.
func (b *CSPBuilder) DefaultSrc(sources ...string) *CSPBuilder {
	b.directives["default-src"] = sources
	return b
}

// ScriptSrc sets the script-src directive for the Content Security Policy.
// This directive specifies valid sources for JavaScript execution.
func (b *CSPBuilder) ScriptSrc(sources ...string) *CSPBuilder {
	b.directives["script-src"] = sources
	return b
}

// StyleSrc sets the style-src directive for the Content Security Policy.
// This directive specifies valid sources for stylesheets.
func (b *CSPBuilder) StyleSrc(sources ...string) *CSPBuilder {
	b.directives["style-src"] = sources
	return b
}

// ImgSrc sets the img-src directive for the Content Security Policy.
// This directive specifies valid sources for images.
func (b *CSPBuilder) ImgSrc(sources ...string) *CSPBuilder {
	b.directives["img-src"] = sources
	return b
}

// Build constructs the final Content Security Policy header value
// from all configured directives.
func (b *CSPBuilder) Build() string {
	var parts []string
	for directive, sources := range b.directives {
		parts = append(parts, directive+" "+strings.Join(sources, " "))
	}
	return strings.Join(parts, "; ")
}

// Example 6: CSP builder pattern
func cspBuilderExample(r *router.Router) {
	builder := r.Group("/builder")

	// Use CSP builder for cleaner configuration
	csp := NewCSPBuilder().
		DefaultSrc("'self'").
		ScriptSrc("'self'", "https://cdn.example.com").
		StyleSrc("'self'", "'unsafe-inline'").
		ImgSrc("'self'", "data:", "https:").
		Build()

	builder.Use(security.New(
		security.WithContentSecurityPolicy(csp),
	))

	builder.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "CSP configured with builder pattern",
			"csp":     csp,
			"note":    "Check response headers for Content-Security-Policy",
		})
	})
}
