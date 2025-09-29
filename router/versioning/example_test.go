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

package versioning_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"rivaas.dev/router/versioning"
)

// ExampleNew demonstrates creating a versioning engine with header-based versioning.
func ExampleNew() {
	engine, err := versioning.New(
		versioning.WithHeaderVersioning("API-Version"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("API-Version", "v2")

	version := engine.DetectVersion(req)
	fmt.Println(version)
	// Output: v2
}

// ExampleNew_pathBased demonstrates path-based versioning.
func ExampleNew_pathBased() {
	engine, err := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/v2/users", nil)
	version := engine.DetectVersion(req)

	fmt.Println(version)
	// Output: v2
}

// ExampleNew_queryBased demonstrates query parameter-based versioning.
func ExampleNew_queryBased() {
	engine, err := versioning.New(
		versioning.WithQueryVersioning("version"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/users?version=v2", nil)
	version := engine.DetectVersion(req)

	fmt.Println(version)
	// Output: v2
}

// ExampleNew_withValidation demonstrates version validation.
func ExampleNew_withValidation() {
	engine, err := versioning.New(
		versioning.WithQueryVersioning("v"),
		versioning.WithValidVersions("v1", "v2", "v3"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Valid version
	req1 := httptest.NewRequest("GET", "/users?v=v2", nil)
	fmt.Println(engine.DetectVersion(req1))

	// Invalid version - falls back to default
	req2 := httptest.NewRequest("GET", "/users?v=v99", nil)
	fmt.Println(engine.DetectVersion(req2))

	// Output:
	// v2
	// v1
}

// ExampleNew_multipleStrategies demonstrates multiple version detection strategies.
func ExampleNew_multipleStrategies() {
	engine, err := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithHeaderVersioning("API-Version"),
		versioning.WithQueryVersioning("v"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Path takes priority
	req := httptest.NewRequest("GET", "/v2/users?v=v3", nil)
	req.Header.Set("API-Version", "v4")

	version := engine.DetectVersion(req)
	fmt.Println(version)
	// Output: v2
}

// ExampleEngine_StripPathVersion demonstrates stripping version from path.
func ExampleEngine_StripPathVersion() {
	engine, err := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	strippedPath := engine.StripPathVersion("/v1/users/123", "v1")
	fmt.Println(strippedPath)
	// Output: /users/123
}

// ExampleEngine_ShouldApplyVersioning demonstrates version routing decision logic.
func ExampleEngine_ShouldApplyVersioning() {
	engine, err := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Path with version
	fmt.Println(engine.ShouldApplyVersioning("/v1/users"))

	// Path without version but has default
	fmt.Println(engine.ShouldApplyVersioning("/users"))

	// Output:
	// true
	// true
}

// ExampleWithDeprecatedVersion demonstrates deprecation headers.
func ExampleWithDeprecatedVersion() {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	engine, err := versioning.New(
		versioning.WithHeaderVersioning("API-Version"),
		versioning.WithDeprecatedVersion("v1", sunsetDate),
	)
	if err != nil {
		log.Fatal(err)
	}

	w := httptest.NewRecorder()
	engine.SetLifecycleHeaders(w, "v1", "/api/users")

	fmt.Println("Deprecation:", w.Header().Get("Deprecation"))
	fmt.Println("Sunset:", w.Header().Get("Sunset") != "")
	// Output:
	// Deprecation: true
	// Sunset: true
}

// ExampleWithObserver demonstrates observability hooks.
func ExampleWithObserver() {
	engine, err := versioning.New(
		versioning.WithHeaderVersioning("API-Version"),
		versioning.WithDefaultVersion("v1"),
		versioning.WithObserver(
			versioning.WithOnDetected(func(version, method string) {
				fmt.Printf("Detected %s via %s\n", version, method)
			}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("API-Version", "v2")

	_ = engine.DetectVersion(req)
	// Output: Detected v2 via header
}

// ExampleEngine_ExtractPathSegment demonstrates extracting version segments from paths.
func ExampleEngine_ExtractPathSegment() {
	engine, err := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	segment, found := engine.ExtractPathSegment("/v99/users")
	fmt.Printf("Found: %v, Segment: %s\n", found, segment)

	segment, found = engine.ExtractPathSegment("/users")
	fmt.Printf("Found: %v, Segment: %s\n", found, segment)

	// Output:
	// Found: true, Segment: v99
	// Found: false, Segment:
}

// ExampleEngine_SetLifecycleHeaders demonstrates comprehensive lifecycle header management.
func ExampleEngine_SetLifecycleHeaders() {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	engine, err := versioning.New(
		versioning.WithHeaderVersioning("API-Version"),
		versioning.WithDeprecatedVersion("v1", sunsetDate),
		versioning.WithDeprecationLink("v1", "https://docs.example.com/migration"),
		versioning.WithVersionHeader(),
		versioning.WithWarning299(),
	)
	if err != nil {
		log.Fatal(err)
	}

	w := httptest.NewRecorder()
	isSunset := engine.SetLifecycleHeaders(w, "v1", "/api/users")

	fmt.Println("Is Sunset:", isSunset)
	fmt.Println("Deprecation:", w.Header().Get("Deprecation"))
	fmt.Println("Sunset:", w.Header().Get("Sunset") != "")
	fmt.Println("Warning:", w.Header().Get("Warning") != "")

	// Output:
	// Is Sunset: false
	// Deprecation: true
	// Sunset: true
	// Warning: true
}

// ExampleWithCustomVersionDetector demonstrates custom version detection logic.
func ExampleWithCustomVersionDetector() {
	engine, err := versioning.New(
		versioning.WithCustomVersionDetector(func(req *http.Request) string {
			// Extract version from custom header
			if version := req.Header.Get("X-Custom-Version"); version != "" {
				return version
			}
			return ""
		}),
		versioning.WithDefaultVersion("v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-Custom-Version", "v2")

	version := engine.DetectVersion(req)
	fmt.Println(version)

	// Output: v2
}

// ExampleWithVersionHeader demonstrates sending X-API-Version header.
func ExampleWithVersionHeader() {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	engine, _ := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithDeprecatedVersion("v1", sunsetDate),
		versioning.WithVersionHeader(),
		versioning.WithWarning299(),
		versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2"),
	)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()

	version := engine.DetectVersion(req)
	engine.SetLifecycleHeaders(w, version, "/api/users")

	fmt.Println("Deprecation:", w.Header().Get("Deprecation"))
	fmt.Println("Sunset present:", w.Header().Get("Sunset") != "")
	fmt.Println("X-API-Version:", w.Header().Get("X-API-Version"))
	fmt.Println("Link present:", w.Header().Get("Link") != "")
	fmt.Println("Warning present:", w.Header().Get("Warning") != "")
	// Output:
	// Deprecation: true
	// Sunset present: true
	// X-API-Version: v1
	// Link present: true
	// Warning present: true
}

// ExampleWithSunsetEnforcement demonstrates 410 Gone for sunset versions.
func ExampleWithSunsetEnforcement() {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	engine, _ := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithDeprecatedVersion("v1", sunsetDate),
		versioning.WithSunsetEnforcement(),
		versioning.WithVersionHeader(),
		versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2"),
		versioning.WithClock(func() time.Time {
			return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // After sunset
		}),
	)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()

	version := engine.DetectVersion(req)
	isSunset := engine.SetLifecycleHeaders(w, version, "/api/users")

	if isSunset {
		fmt.Println("Version is sunset")
	}

	fmt.Println("X-API-Version:", w.Header().Get("X-API-Version"))
	fmt.Println("Sunset present:", w.Header().Get("Sunset") != "")
	// Output:
	// Version is sunset
	// X-API-Version: v1
	// Sunset present: true
}

// ExampleWithDeprecatedUseCallback demonstrates monitoring deprecated API usage.
func ExampleWithDeprecatedUseCallback() {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	done := make(chan bool, 1)

	engine, _ := versioning.New(
		versioning.WithPathVersioning("/v{version}/"),
		versioning.WithDeprecatedVersion("v1", sunsetDate),
		versioning.WithDeprecatedUseCallback(func(version, route string) {
			fmt.Printf("Deprecated API used: %s %s\n", version, route)
			done <- true
		}),
	)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()

	version := engine.DetectVersion(req)
	engine.SetLifecycleHeaders(w, version, "/api/users")

	// Wait for async callback
	<-done

	// Output: Deprecated API used: v1 /api/users
}
