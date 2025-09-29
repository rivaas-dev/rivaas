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

package versioning

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WithHeaderVersioning configures header-based version detection.
//
// Example:
//
//	versioning.WithHeaderVersioning("API-Version")
//	// Client sends: API-Version: v2
func WithHeaderVersioning(headerName string) Option {
	return func(cfg *Config) error {
		if headerName == "" {
			return fmt.Errorf("header name cannot be empty: provide a header name (example: \"API-Version\" or \"X-API-Version\")")
		}
		cfg.HeaderName = headerName
		cfg.HeaderEnabled = true
		return nil
	}
}

// WithQueryVersioning configures query parameter-based version detection.
//
// Example:
//
//	versioning.WithQueryVersioning("v")
//	// Client sends: GET /api/users?v=v2
func WithQueryVersioning(paramName string) Option {
	return func(cfg *Config) error {
		if paramName == "" {
			return fmt.Errorf("query parameter name cannot be empty: provide a parameter name (example: \"v\" or \"version\")")
		}
		cfg.QueryParam = paramName
		cfg.QueryEnabled = true
		return nil
	}
}

// WithPathVersioning configures path-based version detection.
// pattern should contain {version} placeholder, e.g., "/v{version}/", "/api/{version}/"
// The version must be at a path segment boundary.
//
// Example:
//
//	versioning.WithPathVersioning("/v{version}/")
//	// Client sends: GET /v2/api/users
func WithPathVersioning(pattern string) Option {
	return func(cfg *Config) error {
		if pattern == "" {
			return fmt.Errorf("path pattern cannot be empty: provide a pattern with {version} placeholder (example: \"/v{version}/\")")
		}
		if !strings.Contains(pattern, "{version}") {
			return fmt.Errorf("path pattern %q must contain {version} placeholder (example: \"/v{version}/\")", pattern)
		}
		idx := strings.Index(pattern, "{version}")
		if idx <= 0 {
			return fmt.Errorf("invalid path pattern %q: {version} placeholder must not be at the start of the pattern (example: \"/v{version}/\")", pattern)
		}
		cfg.PathPattern = pattern
		cfg.PathEnabled = true

		// Extract prefix for path matching
		// "/v{version}/" -> "/v"
		// "/api/{version}/" -> "/api/"
		cfg.PathPrefix = pattern[:idx]
		return nil
	}
}

// WithAcceptVersioning configures Accept-header based version detection.
// pattern should contain {version} placeholder, e.g., "application/vnd.myapi.v{version}+json"
// Follows RFC 7231 content negotiation and vendor-specific media types.
//
// Example:
//
//	versioning.WithAcceptVersioning("application/vnd.myapi.v{version}+json")
//	// Client sends: Accept: application/vnd.myapi.v2+json
func WithAcceptVersioning(pattern string) Option {
	return func(cfg *Config) error {
		if pattern == "" {
			return fmt.Errorf("accept pattern cannot be empty: provide a pattern with {version} placeholder (example: \"application/vnd.myapi.v{version}+json\")")
		}
		if !strings.Contains(pattern, "{version}") {
			return fmt.Errorf("accept pattern %q must contain {version} placeholder (example: \"application/vnd.myapi.v{version}+json\")", pattern)
		}
		cfg.AcceptPattern = pattern
		cfg.AcceptEnabled = true
		return nil
	}
}

// WithDefaultVersion sets the default version to use when no version is detected.
//
// Example:
//
//	versioning.WithDefaultVersion("v1")
func WithDefaultVersion(version string) Option {
	return func(cfg *Config) error {
		if version == "" {
			return fmt.Errorf("default version cannot be empty: provide a version string (example: \"v1\")")
		}
		cfg.DefaultVersion = version
		return nil
	}
}

// WithValidVersions sets allowed versions for validation.
// If set, requests with invalid versions will use the default version.
//
// Example:
//
//	versioning.WithValidVersions("v1", "v2", "v3")
func WithValidVersions(versions ...string) Option {
	return func(cfg *Config) error {
		if len(versions) == 0 {
			return fmt.Errorf("ValidVersions cannot be empty: provide at least one version (example: WithValidVersions(\"v1\", \"v2\"))")
		}
		for i, v := range versions {
			if v == "" {
				return fmt.Errorf("ValidVersions[%d] is empty: all versions must be non-empty strings", i)
			}
		}
		cfg.ValidVersions = versions
		return nil
	}
}

// WithDeprecatedVersion marks a version as deprecated with a sunset date.
// Adds Sunset and Deprecation headers per RFC 8594.
//
// Example:
//
//	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
//	versioning.WithDeprecatedVersion("v1", sunsetDate)
func WithDeprecatedVersion(version string, sunsetDate time.Time) Option {
	return func(cfg *Config) error {
		if cfg.DeprecatedVersions == nil {
			cfg.DeprecatedVersions = make(map[string]time.Time)
		}
		cfg.DeprecatedVersions[version] = sunsetDate
		return nil
	}
}

// WithCustomVersionDetector sets a custom version detection function.
// This takes precedence over all other detection methods if set.
//
// Example:
//
//	versioning.WithCustomVersionDetector(func(req *http.Request) string {
//	    // Extract version from JWT token
//	    token := req.Header.Get("Authorization")
//	    return extractVersionFromToken(token)
//	})
func WithCustomVersionDetector(detector func(*http.Request) string) Option {
	return func(cfg *Config) error {
		cfg.CustomDetector = detector
		return nil
	}
}

// WithObserver configures observability hooks for version detection events
// using the functional options pattern.
//
// Example:
//
//	versioning.WithObserver(
//	    versioning.WithOnDetected(func(version, method string) {
//	        metrics.RecordVersionUsage(version, method)
//	    }),
//	    versioning.WithOnMissing(func() {
//	        log.Warn("client using default version")
//	    }),
//	    versioning.WithOnInvalid(func(attempted string) {
//	        metrics.RecordInvalidVersion(attempted)
//	    }),
//	)
func WithObserver(opts ...ObserverOption) Option {
	return func(cfg *Config) error {
		observer := &Observer{}
		for _, opt := range opts {
			opt(observer)
		}
		cfg.Observer = observer
		return nil
	}
}

// WithOnDetected sets the callback for when a version is successfully detected.
// The callback receives the detected version and the detection method used.
func WithOnDetected(fn func(version string, method string)) ObserverOption {
	return func(o *Observer) {
		o.OnDetected = fn
	}
}

// WithOnMissing sets the callback for when no version information is found in the request.
// The engine will use the default version in this case.
func WithOnMissing(fn func()) ObserverOption {
	return func(o *Observer) {
		o.OnMissing = fn
	}
}

// WithOnInvalid sets the callback for when a version is detected but fails validation.
// The callback receives the invalid version string that was attempted.
func WithOnInvalid(fn func(attempted string)) ObserverOption {
	return func(o *Observer) {
		o.OnInvalid = fn
	}
}

// WithVersionHeader enables sending X-API-Version response header with the detected version.
//
// Example:
//
//	versioning.WithVersionHeader()
//	// Response includes: X-API-Version: v2
func WithVersionHeader() Option {
	return func(cfg *Config) error {
		cfg.SendVersionHeader = true
		return nil
	}
}

// WithWarning299 enables RFC 7234 Warning: 299 headers for deprecated versions.
// This adds a human-readable warning message to deprecated API responses.
//
// Example:
//
//	versioning.WithWarning299()
//	// Response includes: Warning: 299 - "API v1 is deprecated and will be removed on 2025-12-31..."
func WithWarning299() Option {
	return func(cfg *Config) error {
		cfg.EmitWarning299 = true
		return nil
	}
}

// WithDeprecationLink sets a documentation URL for a deprecated version.
// This URL will be included in Link headers with rel=deprecation and rel=sunset.
//
// Example:
//
//	versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2")
func WithDeprecationLink(version, url string) Option {
	return func(cfg *Config) error {
		if cfg.DeprecationLinks == nil {
			cfg.DeprecationLinks = make(map[string]string)
		}
		cfg.DeprecationLinks[version] = url
		return nil
	}
}

// WithSunsetEnforcement enables 410 Gone responses for versions past their sunset date.
// When enabled, requests to sunset versions will receive 410 Gone instead of being served.
//
// Example:
//
//	versioning.WithSunsetEnforcement()
func WithSunsetEnforcement() Option {
	return func(cfg *Config) error {
		cfg.EnforceSunset = true
		return nil
	}
}

// WithDeprecatedUseCallback sets a callback function that is called when a deprecated
// API version is used. This is useful for metrics collection and monitoring.
//
// The callback is called asynchronously to avoid blocking the request.
//
// Example:
//
//	versioning.WithDeprecatedUseCallback(func(version, route string) {
//	    metrics.DeprecatedAPIUsage.WithLabels(version, route).Inc()
//	    log.Warn("deprecated API used", "version", version, "route", route)
//	})
func WithDeprecatedUseCallback(fn OnDeprecatedUseFunc) Option {
	return func(cfg *Config) error {
		cfg.OnDeprecatedUse = fn
		return nil
	}
}

// WithClock sets a custom clock function for testing.
// This allows injecting a fake time.Now function for deterministic tests.
//
// Example:
//
//	versioning.WithClock(func() time.Time {
//	    return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
//	})
func WithClock(nowFn func() time.Time) Option {
	return func(cfg *Config) error {
		cfg.Now = nowFn
		return nil
	}
}
