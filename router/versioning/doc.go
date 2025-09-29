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

// Package versioning provides comprehensive API versioning support for the Rivaas router.
//
// # Overview
//
// This package handles:
//   - Multiple version detection strategies (path, header, query, accept-header)
//   - Version validation and routing
//   - API lifecycle management (deprecation, sunset enforcement)
//   - RFC 8594 (HTTP Sunset) and RFC 7234 (Warning) headers
//   - Metrics and monitoring hooks
//
// # Version Detection
//
// The engine supports four version detection strategies:
//
// 1. Path-based: /v1/users, /v2/users
// 2. Header-based: API-Version: v1
// 3. Query parameter: /users?v=v1
// 4. Accept-header: Accept: application/vnd.myapi.v1+json
//
// Example:
//
//	engine, err := versioning.New(
//	    versioning.WithPathVersioning("/v{version}/"),
//	    versioning.WithDefaultVersion("v1"),
//	    versioning.WithValidVersions("v1", "v2", "v3"),
//	)
//
// # API Lifecycle Management
//
// The package provides comprehensive lifecycle management features:
//
// ## Deprecation Warnings
//
// Mark versions as deprecated with sunset dates:
//
//	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
//	engine, _ := versioning.New(
//	    versioning.WithPathVersioning("/v{version}/"),
//	    versioning.WithDeprecatedVersion("v1", sunsetDate),
//	    versioning.WithVersionHeader(),          // Send X-API-Version header
//	    versioning.WithWarning299(),             // Send Warning: 299 header
//	    versioning.WithDeprecationLink("v1", "https://docs.example.com/migration"),
//	)
//
// This automatically adds headers to deprecated version responses:
//   - Deprecation: true
//   - Sunset: Sat, 31 Dec 2025 00:00:00 GMT
//   - Link: <https://docs.example.com/migration>; rel="deprecation", rel="sunset"
//   - Warning: 299 - "API v1 is deprecated and will be removed on 2025-12-31..."
//   - X-API-Version: v1
//
// ## Sunset Enforcement
//
// Return 410 Gone for versions past their sunset date:
//
//	engine, _ := versioning.New(
//	    versioning.WithPathVersioning("/v{version}/"),
//	    versioning.WithDeprecatedVersion("v1", sunsetDate),
//	    versioning.WithSunsetEnforcement(),      // Return 410 Gone after sunset
//	)
//
// ## Monitoring and Metrics
//
// Track deprecated API usage for monitoring:
//
//	engine, _ := versioning.New(
//	    versioning.WithPathVersioning("/v{version}/"),
//	    versioning.WithDeprecatedVersion("v1", sunsetDate),
//	    versioning.WithDeprecatedUseCallback(func(version, route string) {
//	        metrics.DeprecatedAPIUsage.WithLabels(version, route).Inc()
//	        log.Warn("deprecated API used", "version", version, "route", route)
//	    }),
//	)
//
// The callback is called asynchronously to avoid blocking the request.
//
// # Integration with Router
//
// The versioning engine integrates seamlessly with the Rivaas router:
//
//	import (
//	    "rivaas.dev/router"
//	    "rivaas.dev/router/versioning"
//	)
//
//	r := router.New(
//	    router.WithVersioning(
//	        versioning.WithPathVersioning("/v{version}/"),
//	        versioning.WithDefaultVersion("v1"),
//	        versioning.WithDeprecatedVersion("v1", sunsetDate),
//	        versioning.WithSunsetEnforcement(),
//	        versioning.WithVersionHeader(),
//	        versioning.WithWarning299(),
//	    ),
//	)
//
//	// Define version-specific routes
//	r.Version("v1").GET("/users", handleUsersV1)
//	r.Version("v2").GET("/users", handleUsersV2)
//
// # Complete Example
//
//	package main
//
//	import (
//	    "log/slog"
//	    "time"
//	    "rivaas.dev/router"
//	    "rivaas.dev/router/versioning"
//	)
//
//	func main() {
//	    sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
//
//	    r := router.New(
//	        router.WithVersioning(
//	            versioning.WithPathVersioning("/v{version}/"),
//	            versioning.WithDefaultVersion("v2"),
//	            versioning.WithValidVersions("v1", "v2", "v3"),
//	            versioning.WithDeprecatedVersion("v1", sunsetDate),
//	            versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2"),
//	            versioning.WithSunsetEnforcement(),
//	            versioning.WithVersionHeader(),
//	            versioning.WithWarning299(),
//	            versioning.WithDeprecatedUseCallback(func(version, route string) {
//	                slog.Warn("deprecated API usage",
//	                    "version", version,
//	                    "route", route)
//	            }),
//	        ),
//	    )
//
//	    // v1 - deprecated
//	    r.Version("v1").GET("/users", func(c *router.Context) {
//	        c.JSON(200, map[string]string{"message": "v1 users endpoint"})
//	    })
//
//	    // v2 - current stable
//	    r.Version("v2").GET("/users", func(c *router.Context) {
//	        c.JSON(200, map[string]string{"message": "v2 users endpoint"})
//	    })
//
//	    // v3 - experimental
//	    r.Version("v3").GET("/users", func(c *router.Context) {
//	        c.JSON(200, map[string]string{"message": "v3 users endpoint"})
//	    })
//
//	    r.Run(":8080")
//	}
//
// # Standards and RFCs
//
// This package implements several RFC standards:
//
//   - RFC 8594: HTTP Sunset Header - https://www.rfc-editor.org/rfc/rfc8594
//   - RFC 7234: HTTP Warning Header - https://www.rfc-editor.org/rfc/rfc7234#section-5.5
//   - RFC 7231: HTTP Accept Content Negotiation - https://www.rfc-editor.org/rfc/rfc7231#section-5.3.2
//   - RFC 9457: Problem Details for HTTP APIs - https://www.rfc-editor.org/rfc/rfc9457
//
// # Design
//
// The versioning engine is designed for production use:
//
//   - Thread-safe operations for version tree selection
//   - Header lookups and map operations
//   - Async callbacks for monitoring
//   - Compiled route tables for versioned paths
//
// # Best Practices
//
//   - Always set a default version for fallback behavior
//   - Use semantic versioning (v1, v2, v3)
//   - Document migration paths in deprecation links
//   - Give clients 6-12 months between deprecation and sunset
//   - Monitor deprecated API usage to track migration progress
//   - Test sunset behavior before enforcement date
//   - Use version headers to help clients debug issues
//
// # Testing
//
// The package includes testing utilities:
//
//	engine, _ := versioning.New(
//	    versioning.WithPathVersioning("/v{version}/"),
//	    versioning.WithDeprecatedVersion("v1", sunsetDate),
//	    versioning.WithSunsetEnforcement(),
//	    versioning.WithClock(func() time.Time {
//	        return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)  // After sunset
//	    }),
//	)
//
// Use WithClock() to inject a fake time source for deterministic tests.
package versioning
