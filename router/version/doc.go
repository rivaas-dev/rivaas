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

// Package version provides API versioning support for the Rivaas router.
//
// This package implements a clean, functional options-based API for configuring
// API versioning with excellent developer experience (DX).
//
// # Basic Usage
//
// Create a versioned router with detection strategies.
// An engine can be created with [New] (returns error) or [MustNew] (panics on error).
// Options must not be nil; passing a nil option results in an error (or panic with MustNew).
//
//	r := router.New(
//	    router.WithVersioning(
//	        version.WithPathDetection("/api/v{version}"),
//	        version.WithHeaderDetection("X-API-Version"),
//	        version.WithDefault("v2"),
//	    ),
//	)
//
// # Detection Strategies
//
// The package supports multiple version detection strategies, checked in priority order:
//
//   - Path-based: version.WithPathDetection("/v{version}/")
//   - Header-based: version.WithHeaderDetection("X-API-Version")
//   - Query-based: version.WithQueryDetection("v")
//   - Accept-header: version.WithAcceptDetection("application/vnd.myapi")
//   - Custom: version.WithCustomDetection(func(r *http.Request) string { ... })
//
// # Version Lifecycle
//
// Configure per-version lifecycle using functional options.
// [Engine.ApplyLifecycle] returns an error if any lifecycle option is nil.
//
//	v1 := r.Version("v1",
//	    version.Deprecated(),
//	    version.Sunset(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)),
//	    version.MigrationDocs("https://docs.example.com/v1-to-v2"),
//	)
//	v1.GET("/users", listUsersV1)
//
// # Response Headers
//
// Configure automatic response headers:
//
//	router.WithVersioning(
//	    version.WithDefault("v2"),
//	    version.WithResponseHeaders(),  // X-API-Version header
//	    version.WithWarning299(),        // Warning: 299 for deprecated
//	    version.WithSunsetEnforcement(), // 410 Gone after sunset
//	)
//
// # Design Philosophy
//
// This package follows these principles:
//
//   - Configuration is via functional options only; no user-facing config struct. [Engine] exposes [Engine.DefaultVersion] and behavior such as [Engine.SetLifecycleHeaders] as needed.
//   - [Engine] must be created with [New] or [MustNew]; do not call methods on a nil Engine. Callers (e.g. the router) guard at the call site when versioning is optional.
//   - Observer hooks (version detection events) are configured only via [WithObserver] and [OnDetected], [OnMissing], [OnInvalid], [OnDeprecatedUse]; no observer type is exported.
//   - Constructors: both [New] and [MustNew] are provided (use MustNew in main/init, New when handling errors).
//   - Progressive disclosure: simple cases are simple, complex cases are possible
//   - Self-documenting: code reads like documentation
//   - Cohesive: everything about a version is on the version object
//   - Familiar: follows Go's functional options pattern
package version
