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

// Package security provides middleware for setting security-related HTTP headers
// such as Content-Security-Policy, X-Frame-Options, and other security headers.
package security

// WithFrameOptions sets the X-Frame-Options header.
// Common values: "DENY", "SAMEORIGIN", "ALLOW-FROM uri"
// Default: "DENY"
//
// Example:
//
//	security.New(security.WithFrameOptions("SAMEORIGIN"))
func WithFrameOptions(value string) Option {
	return func(cfg *config) {
		cfg.frameOptions = value
	}
}

// WithContentTypeNosniff enables or disables X-Content-Type-Options: nosniff.
// This prevents browsers from MIME-sniffing responses.
// Default: true
//
// Example:
//
//	security.New(security.WithContentTypeNosniff(true))
func WithContentTypeNosniff(enabled bool) Option {
	return func(cfg *config) {
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
//	security.New(security.WithXSSProtection("1; mode=block"))
func WithXSSProtection(value string) Option {
	return func(cfg *config) {
		cfg.xssProtection = value
	}
}

// WithHSTS configures HTTP Strict Transport Security.
// maxAge is in seconds (default: 31536000 = 1 year).
//
// Example:
//
//	security.New(security.WithHSTS(63072000, true, true)) // 2 years, includeSubdomains, preload
func WithHSTS(maxAge int, includeSubdomains, preload bool) Option {
	return func(cfg *config) {
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
//	security.New(security.WithContentSecurityPolicy(
//	    "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
//	))
func WithContentSecurityPolicy(policy string) Option {
	return func(cfg *config) {
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
//	security.New(security.WithReferrerPolicy("same-origin"))
func WithReferrerPolicy(policy string) Option {
	return func(cfg *config) {
		cfg.referrerPolicy = policy
	}
}

// WithPermissionsPolicy sets the Permissions-Policy header.
// Controls which browser features and APIs can be used.
//
// Example:
//
//	security.New(security.WithPermissionsPolicy(
//	    "geolocation=(), microphone=(), camera=()",
//	))
func WithPermissionsPolicy(policy string) Option {
	return func(cfg *config) {
		cfg.permissionsPolicy = policy
	}
}

// WithCustomHeader adds a custom security header.
//
// Example:
//
//	security.New(security.WithCustomHeader("X-Custom-Security", "value"))
func WithCustomHeader(name, value string) Option {
	return func(cfg *config) {
		cfg.customHeaders[name] = value
	}
}

// NoSecurityHeaders is an option that disables all security headers.
// This is useful for testing or when security headers are handled by an upstream proxy/gateway.
//
// Example:
//
//	security.New(security.NoSecurityHeaders())
func NoSecurityHeaders() Option {
	return func(cfg *config) {
		cfg.frameOptions = ""
		cfg.contentTypeNosniff = false
		cfg.xssProtection = ""
		cfg.hstsMaxAge = 0
		cfg.hstsIncludeSubdomains = false
		cfg.hstsPreload = false
		cfg.contentSecurityPolicy = ""
		cfg.referrerPolicy = ""
		cfg.permissionsPolicy = ""
		cfg.customHeaders = make(map[string]string)
	}
}

// DevelopmentPreset returns an option with relaxed security headers suitable for development.
// This preset includes basic security headers but allows inline scripts and styles for easier development.
//
// Headers set:
//   - X-Frame-Options: SAMEORIGIN
//   - X-Content-Type-Options: nosniff
//   - X-XSS-Protection: 1; mode=block
//   - Content-Security-Policy: Relaxed policy with unsafe-inline and unsafe-eval
//   - Referrer-Policy: no-referrer-when-downgrade
//   - No HSTS (to allow switching between HTTP/HTTPS in development)
//
// Example:
//
//	security.New(security.DevelopmentPreset())
func DevelopmentPreset() Option {
	return func(cfg *config) {
		cfg.frameOptions = "SAMEORIGIN"
		cfg.contentTypeNosniff = true
		cfg.xssProtection = "1; mode=block"
		cfg.contentSecurityPolicy = "default-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:;"
		cfg.referrerPolicy = "no-referrer-when-downgrade"
		// No HSTS in development
		cfg.hstsMaxAge = 0
		cfg.hstsIncludeSubdomains = false
		cfg.hstsPreload = false
	}
}

// ProductionPreset returns an option with strict security headers for production environments.
// This preset enables all recommended security headers with strict policies.
//
// Headers set:
//   - X-Frame-Options: DENY
//   - X-Content-Type-Options: nosniff
//   - X-XSS-Protection: 1; mode=block
//   - Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
//   - Content-Security-Policy: default-src 'self'
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: Restrict geolocation, microphone, and camera
//
// Example:
//
//	security.New(security.ProductionPreset())
//
// You can override specific headers:
//
//	security.New(
//	    security.ProductionPreset(),
//	    security.WithFrameOptions("SAMEORIGIN"),
//	)
func ProductionPreset() Option {
	return func(cfg *config) {
		cfg.frameOptions = "DENY"
		cfg.contentTypeNosniff = true
		cfg.xssProtection = "1; mode=block"
		cfg.hstsMaxAge = 31536000
		cfg.hstsIncludeSubdomains = true
		cfg.hstsPreload = true
		cfg.contentSecurityPolicy = "default-src 'self'"
		cfg.referrerPolicy = "strict-origin-when-cross-origin"
		cfg.permissionsPolicy = "geolocation=(), microphone=(), camera=()"
	}
}
