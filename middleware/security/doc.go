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
// to protect against common web vulnerabilities.
//
// This middleware sets various security headers recommended by security best
// practices and standards (OWASP, Mozilla, etc.) to protect web applications
// from common attacks like XSS, clickjacking, MIME type sniffing, and more.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/security"
//
//	r := router.MustNew()
//	r.Use(security.New())
//
// # Security Headers
//
// The middleware sets the following security headers by default:
//
//   - X-Content-Type-Options: Prevents MIME type sniffing (nosniff)
//   - X-Frame-Options: Prevents clickjacking (DENY)
//   - X-XSS-Protection: Legacy XSS protection (0; mode=block)
//   - Referrer-Policy: Controls referrer information (no-referrer-when-downgrade)
//   - Permissions-Policy: Controls browser features and APIs
//
// # Configuration Options
//
//   - WithFrameOptions: X-Frame-Options value (DENY, SAMEORIGIN, ALLOW-FROM)
//   - WithContentTypeOptions: X-Content-Type-Options value (nosniff)
//   - WithXSSProtection: X-XSS-Protection value
//   - WithReferrerPolicy: Referrer-Policy value
//   - WithPermissionsPolicy: Permissions-Policy value
//   - WithHSTS: HTTP Strict Transport Security configuration
//   - WithCSP: Content Security Policy configuration
//
// # HSTS Configuration
//
// HTTP Strict Transport Security (HSTS) forces HTTPS connections:
//
//	r.Use(security.New(
//	    security.WithHSTS(
//	        security.HSTSMaxAge(31536000), // 1 year
//	        security.HSTSIncludeSubdomains(true),
//	        security.HSTSPreload(true),
//	    ),
//	))
//
// # CSP Configuration
//
// Content Security Policy (CSP) controls resource loading:
//
//	r.Use(security.New(
//	    security.WithCSP("default-src 'self'; script-src 'self' 'unsafe-inline'"),
//	))
//
// # Security Best Practices
//
// This middleware implements security headers recommended by:
//
//   - OWASP Secure Headers Project
//   - Mozilla Observatory
//   - Security Headers (securityheaders.com)
//
// Always use HTTPS in production and configure HSTS appropriately.
package security
