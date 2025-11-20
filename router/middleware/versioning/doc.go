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

// Package versioning provides middleware for API versioning support,
// allowing routes to be versioned based on headers, query parameters, or paths.
//
// This middleware enables API versioning strategies and provides deprecation
// and sunset warnings to clients. It integrates with the router's versioning
// system to route requests to version-specific handlers.
//
// # Basic Usage
//
//	import "rivaas.dev/router/middleware/versioning"
//
//	r := router.MustNew()
//	r.Use(versioning.New(
//	    versioning.WithVersions(
//	        versioning.VersionInfo{
//	            Version: "v1",
//	            Deprecated: false,
//	        },
//	        versioning.VersionInfo{
//	            Version: "v2",
//	            Deprecated: false,
//	        },
//	    ),
//	))
//
// # Version Information
//
// Each version can include:
//
//   - Version: Version string (e.g., "v1", "v2")
//   - Deprecated: Whether the version is deprecated
//   - DeprecationDate: Date when version was deprecated
//   - SunsetDate: Date when version will be removed
//
// # Deprecation Warnings
//
// When a deprecated version is used, the middleware sets warning headers:
//
//   - Deprecation: Indicates the version is deprecated
//   - Sunset: Date when the version will be removed
//   - Link: Link to migration guide or newer version
//
// # Configuration Options
//
//   - Versions: List of version information
//   - WarningHandler: Custom handler for version warnings
//
// # Integration with Router Versioning
//
// This middleware works with the router's built-in versioning system:
//
//	r := router.MustNew(
//	    router.WithVersioning(
//	        router.VersionByHeader("X-API-Version"),
//	    ),
//	)
//	r.Use(versioning.New(...))
//
// # Performance
//
// Version checking adds minimal overhead (~50-100ns) per request.
package versioning
