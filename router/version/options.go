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

package version

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Detection Strategy Options
// ═══════════════════════════════════════════════════════════════════════════════

// WithPathDetection configures path-based version detection.
// Pattern must contain {version} placeholder.
//
// Example:
//
//	version.WithPathDetection("/api/v{version}")
//	// Matches: /api/v1/users, /api/v2/users
//
//	version.WithPathDetection("/v{version}/")
//	// Matches: /v1/users, /v2/users
func WithPathDetection(pattern string) Option {
	return func(cfg *Config) {
		if pattern == "" {
			cfg.validationErrors = append(cfg.validationErrors, ErrEmptyPathPattern)
			return
		}
		if !strings.Contains(pattern, "{version}") {
			cfg.validationErrors = append(cfg.validationErrors, fmt.Errorf("%w: path pattern %q", ErrMissingVersionPlaceholder, pattern))
			return
		}
		cfg.detectors = append(cfg.detectors, newPathDetector(pattern))
	}
}

// WithHeaderDetection configures header-based version detection.
//
// Example:
//
//	version.WithHeaderDetection("X-API-Version")
//	// Client sends: X-API-Version: v2
//
//	version.WithHeaderDetection("API-Version")
//	// Client sends: API-Version: v2
func WithHeaderDetection(headerName string) Option {
	return func(cfg *Config) {
		if headerName == "" {
			cfg.validationErrors = append(cfg.validationErrors, ErrEmptyHeaderName)
			return
		}
		cfg.detectors = append(cfg.detectors, &headerDetector{header: headerName})
	}
}

// WithQueryDetection configures query parameter-based version detection.
//
// Example:
//
//	version.WithQueryDetection("v")
//	// Client sends: GET /users?v=v2
//
//	version.WithQueryDetection("version")
//	// Client sends: GET /users?version=v2
func WithQueryDetection(paramName string) Option {
	return func(cfg *Config) {
		if paramName == "" {
			cfg.validationErrors = append(cfg.validationErrors, ErrEmptyQueryParam)
			return
		}
		cfg.detectors = append(cfg.detectors, &queryDetector{param: paramName})
	}
}

// WithAcceptDetection configures Accept-header based version detection.
// Follows RFC 7231 vendor-specific media types.
//
// Example:
//
//	version.WithAcceptDetection("application/vnd.myapi.v{version}+json")
//	// Client sends: Accept: application/vnd.myapi.v2+json
func WithAcceptDetection(pattern string) Option {
	return func(cfg *Config) {
		if pattern == "" {
			cfg.validationErrors = append(cfg.validationErrors, ErrEmptyAcceptPattern)
			return
		}
		if !strings.Contains(pattern, "{version}") {
			cfg.validationErrors = append(cfg.validationErrors, fmt.Errorf("%w: accept pattern %q", ErrMissingVersionPlaceholder, pattern))
			return
		}
		cfg.detectors = append(cfg.detectors, &acceptDetector{pattern: pattern})
	}
}

// WithCustomDetection configures a custom version detection function.
// Custom detectors have the highest priority when used.
//
// Example:
//
//	version.WithCustomDetection(func(r *http.Request) string {
//	    // Extract version from JWT token
//	    := r.Header.Get("Authorization")
//	    return extractVersionFromToken(token)
//	})
func WithCustomDetection(fn func(*http.Request) string) Option {
	return func(cfg *Config) {
		if fn == nil {
			cfg.validationErrors = append(cfg.validationErrors, ErrNilCustomDetector)
			return
		}
		// Insert at the beginning for highest priority
		cfg.detectors = append([]Detector{&customDetector{fn: fn}}, cfg.detectors...)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Configuration Options
// ═══════════════════════════════════════════════════════════════════════════════

// WithDefault sets the default version when none is detected.
//
// Example:
//
//	version.WithDefault("v2")
func WithDefault(version string) Option {
	return func(cfg *Config) {
		if version == "" {
			cfg.validationErrors = append(cfg.validationErrors, ErrEmptyDefaultVersion)
			return
		}
		cfg.defaultVersion = version
	}
}

// WithValidVersions sets the allowed versions for validation.
// Requests with invalid versions will fall back to the default version.
//
// Example:
//
//	version.WithValidVersions("v1", "v2", "v3")
func WithValidVersions(versions ...string) Option {
	return func(cfg *Config) {
		if len(versions) == 0 {
			cfg.validationErrors = append(cfg.validationErrors, ErrNoValidVersions)
			return
		}
		for i, v := range versions {
			if v == "" {
				cfg.validationErrors = append(cfg.validationErrors, fmt.Errorf("%w at index %d", ErrEmptyVersionEntry, i))
				return
			}
		}
		cfg.validVersions = versions
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Response Behavior Options
// ═══════════════════════════════════════════════════════════════════════════════

// WithResponseHeaders enables sending X-API-Version header in all versioned responses.
//
// Example:
//
//	version.WithResponseHeaders()
//	// Response includes: X-API-Version: v2
func WithResponseHeaders() Option {
	return func(cfg *Config) {
		cfg.sendVersionHeader = true
	}
}

// WithWarning299 enables RFC 7234 Warning: 299 headers for deprecated versions.
//
// Example:
//
//	version.WithWarning299()
//	// Response includes: Warning: 299 - "API v1 is deprecated..."
func WithWarning299() Option {
	return func(cfg *Config) {
		cfg.sendWarning299 = true
	}
}

// WithSunsetEnforcement enables 410 Gone responses for versions past their sunset date.
//
// Example:
//
//	version.WithSunsetEnforcement()
func WithSunsetEnforcement() Option {
	return func(cfg *Config) {
		cfg.enforceSunset = true
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Observer Options
// ═══════════════════════════════════════════════════════════════════════════════

// ObserverOption configures the version observer.
type ObserverOption func(*Observer)

// WithObserver configures observability hooks for version detection events.
//
// Example:
//
//	version.WithObserver(
//	    version.OnDetected(func(v, method string) {
//	        metrics.RecordVersionUsage(v, method)
//	    }),
//	    version.OnDeprecatedUse(func(v, route string) {
//	        log.Warn("deprecated API", "version", v, "route", route)
//	    }),
//	)
func WithObserver(opts ...ObserverOption) Option {
	return func(cfg *Config) {
		obs := &Observer{}
		for _, opt := range opts {
			opt(obs)
		}
		cfg.observer = obs
	}
}

// OnDetected sets the callback for successful version detection.
func OnDetected(fn func(version, method string)) ObserverOption {
	return func(o *Observer) {
		o.OnDetected = fn
	}
}

// OnMissing sets the callback for when no version is found (using default).
func OnMissing(fn func()) ObserverOption {
	return func(o *Observer) {
		o.OnMissing = fn
	}
}

// OnInvalid sets the callback for invalid version detection.
func OnInvalid(fn func(attempted string)) ObserverOption {
	return func(o *Observer) {
		o.OnInvalid = fn
	}
}

// OnDeprecatedUse sets the callback for deprecated version usage.
func OnDeprecatedUse(fn func(version, route string)) ObserverOption {
	return func(o *Observer) {
		o.OnDeprecatedUse = fn
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Testing Options
// ═══════════════════════════════════════════════════════════════════════════════

// WithClock sets a custom clock function for testing.
//
// Example:
//
//	version.WithClock(func() time.Time {
//	    return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
//	})
func WithClock(nowFn func() time.Time) Option {
	return func(cfg *Config) {
		cfg.now = nowFn
	}
}
