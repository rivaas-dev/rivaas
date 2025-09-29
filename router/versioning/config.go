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

// Config holds configuration for version detection.
// It defines how API versions are extracted from HTTP requests using various strategies.
type Config struct {
	// Header-based versioning
	HeaderName    string // e.g., "API-Version", "X-API-Version", "Accept-Version"
	HeaderEnabled bool

	// Query parameter-based versioning
	QueryParam   string // e.g., "version", "v", "api_version"
	QueryEnabled bool

	// Path-based versioning
	PathPattern string // e.g., "/v{version}/", "/api/{version}/"
	PathEnabled bool
	PathPrefix  string // extracted prefix like "/v", "/api/v"

	// Accept-header based content negotiation (RFC 7231)
	AcceptPattern string // e.g., "application/vnd.myapi.v{version}+json"
	AcceptEnabled bool

	// Default version when no version is specified
	DefaultVersion string

	// Version validation (optional)
	ValidVersions []string // e.g., ["v1", "v2", "latest"]

	// Deprecated versions with sunset dates (RFC 8594)
	DeprecatedVersions map[string]time.Time // version -> sunset date

	// Custom version detection function
	CustomDetector func(*http.Request) string

	// Lifecycle management options
	SendVersionHeader bool                // Add X-API-Version response header
	EmitWarning299    bool                // Emit Warning: 299 header for deprecated versions
	OnDeprecatedUse   OnDeprecatedUseFunc // Callback when deprecated API is used
	DeprecationLinks  map[string]string   // version -> documentation URL
	EnforceSunset     bool                // Return 410 Gone for past-sunset versions
	Now               func() time.Time    // Clock function for testing

	// Observer provides structured observability hooks for version detection events.
	// Use WithVersionObserver() to configure callbacks for tracking version usage.
	Observer *Observer
}

// OnDeprecatedUseFunc is called when a deprecated API version is used.
// Parameters:
//   - version: The deprecated version being used
//   - route: The route template being accessed
type OnDeprecatedUseFunc func(version, route string)

// Observer holds optional callbacks for version detection events.
// This struct provides hooks for monitoring, logging, and metrics collection
// during version detection.
//
// Example:
//
//	versioning.WithObserver(
//	    versioning.WithOnDetected(func(version, method string) {
//	        metrics.RecordVersionUsage(version, method)
//	    }),
//	    versioning.WithOnInvalid(func(attempted string) {
//	        log.Warn("invalid version attempted", "version", attempted)
//	    }),
//	)
type Observer struct {
	// OnDetected is called when a version is successfully detected from the request.
	// Parameters:
	//   - version: The detected version string (e.g., "v1", "v2")
	//   - method: The detection method used ("path", "header", "query", "accept", "custom")
	OnDetected func(version string, method string)

	// OnMissing is called when no version information is found in the request.
	// The router will use the default version in this case.
	OnMissing func()

	// OnInvalid is called when a version is detected but fails validation.
	// Parameter:
	//   - attempted: The invalid version string that was attempted
	OnInvalid func(attempted string)
}

// DeprecationConfig holds configuration for a deprecated API version (RFC 8594).
type DeprecationConfig struct {
	// SunsetDate is when the version will be removed
	SunsetDate time.Time

	// Link provides URL to migration guide or new version documentation
	Link string

	// Message provides human-readable deprecation notice
	Message string
}

// Option is a functional option for configuring the versioning engine.
type Option func(*Config) error

// ObserverOption is a functional option for configuring an Observer.
type ObserverOption func(*Observer)

// validate checks the configuration for errors and inconsistencies.
// Returns an error if the configuration is invalid.
//
// Validations performed:
//   - Path pattern must contain {version} placeholder if path versioning is enabled
//   - Accept pattern must contain {version} placeholder if accept versioning is enabled
//   - Header name must be non-empty if header versioning is enabled
//   - Query parameter name must be non-empty if query versioning is enabled
//   - ValidVersions must include all deprecated versions
//   - Sunset dates should be in the future (warning only, not an error)
func (c *Config) validate() error {
	// Validate path-based versioning
	if c.PathEnabled {
		if c.PathPattern == "" {
			return fmt.Errorf("path versioning enabled but PathPattern is empty: use WithPathVersioning(pattern) with pattern containing {version} placeholder (example: \"/v{version}/\")")
		}
		if !strings.Contains(c.PathPattern, "{version}") {
			return fmt.Errorf("invalid path pattern %q: must contain {version} placeholder (example: \"/v{version}/\")", c.PathPattern)
		}
		if c.PathPrefix == "" {
			return fmt.Errorf("invalid path pattern %q: could not extract prefix from pattern (pattern must contain {version} placeholder)", c.PathPattern)
		}
	}

	// Validate accept-header versioning
	if c.AcceptEnabled {
		if c.AcceptPattern == "" {
			return fmt.Errorf("accept versioning enabled but AcceptPattern is empty: use WithAcceptVersioning(pattern) with pattern containing {version} placeholder (example: \"application/vnd.myapi.v{version}+json\")")
		}
		if !strings.Contains(c.AcceptPattern, "{version}") {
			return fmt.Errorf("invalid accept pattern %q: must contain {version} placeholder (example: \"application/vnd.myapi.v{version}+json\")", c.AcceptPattern)
		}
	}

	// Validate header-based versioning
	if c.HeaderEnabled {
		if c.HeaderName == "" {
			return fmt.Errorf("header versioning enabled but HeaderName is empty: use WithHeaderVersioning(headerName) with non-empty header name (example: \"API-Version\")")
		}
	}

	// Validate query parameter versioning
	if c.QueryEnabled {
		if c.QueryParam == "" {
			return fmt.Errorf("query versioning enabled but QueryParam is empty: use WithQueryVersioning(paramName) with non-empty parameter name (example: \"v\" or \"version\")")
		}
	}

	// Validate that deprecated versions are in ValidVersions (if ValidVersions is set)
	if len(c.ValidVersions) > 0 && len(c.DeprecatedVersions) > 0 {
		for deprecatedVersion := range c.DeprecatedVersions {
			found := false
			for _, validVersion := range c.ValidVersions {
				if validVersion == deprecatedVersion {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("deprecated version %q must be included in ValidVersions: add %q to WithValidVersions(...) call", deprecatedVersion, deprecatedVersion)
			}
		}
	}

	// Validate default version is set
	if c.DefaultVersion == "" {
		return fmt.Errorf("DefaultVersion is empty: use WithDefaultVersion(version) to set a default version (example: \"v1\")")
	}

	return nil
}
