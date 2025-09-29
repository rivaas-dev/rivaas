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

package security

import (
	"fmt"

	"rivaas.dev/router"
)

// Option defines functional options for security middleware configuration.
type Option func(*config)

// config holds the configuration for the security middleware.
type config struct {
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

// defaultConfig returns secure default configuration.
func defaultConfig() *config {
	return &config{
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

// New returns a middleware that sets security headers on HTTP responses.
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
//	r := router.MustNew()
//	r.Use(security.New())
//
// Custom configuration:
//
//	r.Use(security.New(
//	    security.WithFrameOptions("SAMEORIGIN"),
//	    security.WithContentSecurityPolicy("default-src 'self'; script-src 'self' https://cdn.example.com"),
//	))
//
// For development (more permissive):
//
//	r.Use(security.New(
//	    security.WithFrameOptions("SAMEORIGIN"),
//	    security.WithContentSecurityPolicy("default-src 'self' 'unsafe-inline' 'unsafe-eval'"),
//	))
//
// Disable HSTS (useful in development):
//
//	r.Use(security.New(
//	    security.WithHSTS(0, false, false), // Disables HSTS
//	))
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
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
