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

// Package trailingslash provides middleware for handling trailing slashes in URLs,
// ensuring consistent URL canonicalization and preventing duplicate content issues.
//
// This middleware normalizes URLs by adding or removing trailing slashes according
// to a configured policy. This helps with SEO, prevents duplicate content issues,
// and ensures consistent routing behavior.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/trailingslash"
//
//	r := router.MustNew()
//	r.Use(trailingslash.New(
//	    trailingslash.WithPolicy(trailingslash.PolicyRemove),
//	))
//
// # Policies
//
// The middleware supports three policies:
//
//   - PolicyRemove: Redirects paths with trailing slashes to canonical form without slash
//     Example: /users/ → /users (308 redirect)
//   - PolicyAdd: Redirects paths without trailing slashes to canonical form with slash
//     Example: /users → /users/ (308 redirect)
//   - PolicyIgnore: No action taken (passes through unchanged)
//
// # Configuration Options
//
//   - Policy: Trailing slash handling policy (Remove, Add, or Ignore)
//   - RedirectCode: HTTP status code for redirects (default: 308 Permanent Redirect)
//   - SkipPaths: Paths to exclude from trailing slash handling
//
// # SEO Considerations
//
// Trailing slash normalization helps with:
//
//   - Preventing duplicate content (same content at /users and /users/)
//   - Consistent URL canonicalization
//   - Better search engine indexing
//
// Choose a policy and stick with it consistently across your application.
//
// Redirects use HTTP 308 status codes for permanent redirects.
package trailingslash
