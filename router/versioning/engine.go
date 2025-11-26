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
	"slices"
	"strings"
	"time"
)

// Engine manages API versioning, including version detection from requests
// and deprecation header management.
type Engine struct {
	config *Config
}

// New creates a new versioning engine with the given options.
//
// Example:
//
//	engine, err := versioning.New(
//	    versioning.WithHeaderVersioning("API-Version"),
//	    versioning.WithDefaultVersion("v1"),
//	    versioning.WithValidVersions("v1", "v2", "v3"),
//	)
func New(opts ...Option) (*Engine, error) {
	cfg := &Config{
		DefaultVersion: "v1", // Sensible default
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}

	// Validate configuration after all options are applied
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &Engine{
		config: cfg,
	}, nil
}

// DetectVersion performs version detection.
// Checks are ordered for the most common cases first.
//
// Detection order (by priority):
// 1. Custom detector (highest priority, if set)
// 2. Path-based (most explicit and RESTful)
// 3. Header-based (common in production APIs)
// 4. Accept-based (content negotiation, RFC 7231)
// 5. Query-based (convenient for testing)
// 6. Default version (fallback)
func (e *Engine) DetectVersion(req *http.Request) string {
	// Defensive checks: handle nil cases gracefully
	if e == nil || e.config == nil {
		return "v1" // Safe fallback
	}
	if req == nil {
		return e.config.DefaultVersion
	}

	cfg := e.config

	// Custom detector (highest priority)
	if cfg.CustomDetector != nil {
		if version := cfg.CustomDetector(req); version != "" {
			if validated := e.validateVersion(version); validated != "" {
				e.notifyVersionDetected(validated, "custom")
				return validated
			}
		}
	}

	// Path-based detection (most explicit and RESTful)
	if cfg.PathEnabled {
		if segment, ok := extractPathVersion(req.URL.Path, cfg.PathPrefix); ok && segment != "" {
			// Try multiple version formats to match registered routes
			// e.g., "/v1" could match routes registered as "v1" or "1"
			// Check candidates inline
			if strings.HasSuffix(cfg.PathPrefix, "v") {
				// Try "v" + segment first (e.g., "v1")
				if validated := e.validateVersion("v" + segment); validated != "" {
					e.notifyVersionDetected(validated, "path")
					return validated
				}
			}
			// Try segment as-is (e.g., "1")
			if validated := e.validateVersion(segment); validated != "" {
				e.notifyVersionDetected(validated, "path")
				return validated
			}
		}
	}

	// Header-based detection (common in production APIs)
	if cfg.HeaderEnabled {
		if header := req.Header.Get(cfg.HeaderName); header != "" {
			if validated := e.validateVersion(header); validated != "" {
				e.notifyVersionDetected(validated, "header")
				return validated
			}
		}
	}

	// Accept-header based content negotiation (RFC 7231)
	// Supports vendor-specific media types like "application/vnd.myapi.v2+json"
	if cfg.AcceptEnabled {
		if accept := req.Header.Get("Accept"); accept != "" {
			if version, ok := extractAcceptVersion(accept, cfg.AcceptPattern); ok && version != "" {
				if validated := e.validateVersion(version); validated != "" {
					e.notifyVersionDetected(validated, "accept")
					return validated
				}
			}
		}
	}

	// Query parameter-based detection (convenient for testing)
	if cfg.QueryEnabled {
		if version, ok := extractQueryVersion(req.URL.RawQuery, cfg.QueryParam); ok && version != "" {
			if validated := e.validateVersion(version); validated != "" {
				e.notifyVersionDetected(validated, "query")
				return validated
			}
		}
	}

	// No version detected, use default
	e.notifyVersionMissing()

	return cfg.DefaultVersion
}

// StripPathVersion removes the version segment from the path when path-based versioning is enabled.
// This is called before route matching to ensure routes registered without version prefix can match.
//
// The function handles the case where the pattern includes characters before {version}.
// For example, pattern "/v{version}/" extracts "1" as version from "/v1/users", but we need
// to strip "/v1/" to match routes registered as "/users".
//
// Examples:
//   - "/v1/users" with prefix "/v" and detected version "v1" → "/users"
//   - "/api/v2/posts" with prefix "/api/v" and detected version "v2" → "/api/posts"
//   - "/users" (no version) → "/users" (unchanged)
func (e *Engine) StripPathVersion(path, version string) string {
	// Defensive checks
	if e == nil || e.config == nil {
		return path // Return path unchanged if engine is invalid
	}
	if path == "" {
		return path
	}

	cfg := e.config
	if !cfg.PathEnabled || version == "" {
		return path // No path-based versioning or no version detected
	}

	// Check if path starts with prefix
	if !strings.HasPrefix(path, cfg.PathPrefix) {
		return path // Path doesn't match prefix pattern
	}

	// Calculate version segment start (after prefix)
	versionStart := len(cfg.PathPrefix)
	if versionStart >= len(path) {
		return path // Invalid: prefix extends beyond path
	}

	// Find where version segment ends (next "/" or end of path)
	remaining := path[versionStart:]
	versionEnd := strings.IndexByte(remaining, '/')

	var versionSegment string
	if versionEnd == -1 {
		// Version is at end of path (e.g., "/v1")
		versionSegment = remaining
	} else {
		versionSegment = remaining[:versionEnd]
	}

	// Check if the version segment matches detected version
	// The detected version might be "v1" (from pattern "/v{version}/") or just "1"
	// We check multiple ways to match:
	// 1. Segment directly matches version (e.g., segment "v1" == version "v1")
	// 2. Prefix+segment matches version (e.g., "/v" + "1" != "v1", but we handle this below)
	// 3. Segment matches version without prefix (e.g., segment "1" == version "v1"[1:])
	fullSegment := cfg.PathPrefix + versionSegment
	versionMatches := versionSegment == version || fullSegment == version

	// Special case: if prefix ends with "v" and version is "v" + segment
	if !versionMatches && strings.HasSuffix(cfg.PathPrefix, "v") && len(version) > 1 && version[0] == 'v' {
		if versionSegment == version[1:] {
			versionMatches = true
		}
	}

	if !versionMatches {
		return path // Version doesn't match, don't strip
	}

	// Calculate where to start the stripped path
	var strippedStart int
	if versionEnd == -1 {
		// Version at end of path, strip everything (versionMatches already checked above)
		return "/" // Return root path
	}

	// Strip prefix + version segment
	// Example: "/v1/users" with prefix "/v", segment "1" → strip "/v1" → "/users"
	// versionEnd is index of "/" within remaining, so absolute position is versionStart + versionEnd
	// We want to start from that "/", so use versionStart + versionEnd
	strippedStart = versionStart + versionEnd
	if strippedStart >= len(path) {
		return "/" // Path becomes root after stripping
	}

	return path[strippedStart:]
}

// ShouldApplyVersioning determines whether version-specific routing should be applied.
// Returns false if:
// - Path-based versioning is enabled but no version in path and no default configured
//
// This helps routers decide whether to perform version detection and routing.
func (e *Engine) ShouldApplyVersioning(path string) bool {
	// Defensive checks
	if e == nil || e.config == nil {
		return false // Safe default: don't apply versioning if engine is invalid
	}

	// If not path-based versioning, always use it (header/query/accept)
	if !e.config.PathEnabled {
		return true
	}

	// Path-based: check if version is in path
	_, hasVersionInPath := extractPathVersion(path, e.config.PathPrefix)
	if hasVersionInPath {
		return true
	}

	// No version in path, but we have a default - use versioning
	if e.config.DefaultVersion != "" {
		return true
	}

	// Path-based versioning enabled, no version in path, no default - skip versioning
	return false
}

// ExtractPathSegment extracts the actual version segment from a path,
// regardless of whether it's valid. This is useful for stripping invalid
// versions before falling back to default routing.
//
// Returns the segment to strip (e.g., "v99" from "/v99/users") and whether
// a segment was found.
func (e *Engine) ExtractPathSegment(path string) (string, bool) {
	// Defensive checks
	if e == nil || e.config == nil {
		return "", false
	}
	if path == "" {
		return "", false
	}

	if !e.config.PathEnabled {
		return "", false
	}

	segment, ok := extractPathVersion(path, e.config.PathPrefix)
	if !ok || segment == "" {
		return "", false
	}

	// Build the full segment as it appears in the path
	if strings.HasSuffix(e.config.PathPrefix, "v") {
		return "v" + segment, true
	}
	return segment, true
}

// SetLifecycleHeaders adds comprehensive API lifecycle headers following RFC 8594 and RFC 9457.
// This should be called after version detection with the ResponseWriter available.
//
// Sets headers based on configuration:
// - X-API-Version: The detected version (if SendVersionHeader enabled)
// - Deprecation: HTTP-date when version was deprecated (RFC 8594)
// - Sunset: HTTP-date when version will be removed (RFC 8594)
// - Link: Documentation URLs with rel=deprecation and rel=sunset
// - Warning: 299 with deprecation message (if EmitWarning299 enabled)
//
// Returns true if the version has passed its sunset date (caller should return 410 Gone).
func (e *Engine) SetLifecycleHeaders(w http.ResponseWriter, version string, route string) bool {
	// Defensive checks
	if e == nil || e.config == nil {
		return false // Safe default: don't enforce sunset if engine is invalid
	}
	if w == nil {
		return false // Can't set headers without ResponseWriter
	}

	cfg := e.config

	// Always set version header if enabled
	if cfg.SendVersionHeader && version != "" {
		w.Header().Set("X-API-Version", version)
	}

	// Early return if no deprecated versions configured
	if len(cfg.DeprecatedVersions) == 0 {
		return false
	}

	sunsetDate, deprecated := cfg.DeprecatedVersions[version]
	if !deprecated {
		return false // Not a deprecated version
	}

	// Get current time (injectable for testing)
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now() }
	}
	currentTime := now()

	// Check if version has been sunset (past removal date)
	if cfg.EnforceSunset && currentTime.After(sunsetDate) {
		// Version is past sunset - caller should return 410 Gone
		// Still set headers to provide context
		w.Header().Set("Sunset", sunsetDate.UTC().Format(http.TimeFormat))
		if docURL, ok := cfg.DeprecationLinks[version]; ok {
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"sunset\"", docURL))
		}
		return true
	}

	// Version is deprecated but not yet sunset
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", sunsetDate.UTC().Format(http.TimeFormat))

	// Add Link headers for documentation
	if docURL, ok := cfg.DeprecationLinks[version]; ok {
		linkHeaders := []string{
			fmt.Sprintf("<%s>; rel=\"deprecation\"", docURL),
			fmt.Sprintf("<%s>; rel=\"sunset\"", docURL),
		}
		w.Header().Set("Link", strings.Join(linkHeaders, ", "))
	}

	// Add Warning: 299 header if enabled
	if cfg.EmitWarning299 {
		warningMsg := fmt.Sprintf("299 - \"API %s is deprecated and will be removed on %s. Please upgrade to a supported version.\"",
			version, sunsetDate.Format(time.RFC3339))
		w.Header().Set("Warning", warningMsg)
	}

	// Call deprecated usage callback synchronously
	// NOTE: This blocks the request handler. For I/O operations, use async patterns
	// (buffered channels, worker pools) to avoid blocking request processing.
	if cfg.OnDeprecatedUse != nil {
		cfg.OnDeprecatedUse(version, route)
	}

	return false
}

// validateVersion checks if a version is valid and returns it, or empty string if invalid.
// This helper reduces code duplication in DetectVersion.
func (e *Engine) validateVersion(version string) string {
	if version == "" {
		return ""
	}

	// Defensive check
	if e == nil || e.config == nil {
		return version // Return as-is if engine is invalid (better than empty)
	}

	cfg := e.config

	// Early return if no validation configured
	if len(cfg.ValidVersions) == 0 {
		return version
	}

	// Validation enabled: check if version is in allowed list
	if slices.Contains(cfg.ValidVersions, version) {
		return version
	}

	// Invalid version - notify observer if configured
	if cfg.Observer != nil && cfg.Observer.OnInvalid != nil {
		cfg.Observer.OnInvalid(version)
	}
	return ""
}

// notifyVersionDetected calls the observer callback when a version is detected.
func (e *Engine) notifyVersionDetected(version, method string) {
	if e == nil || e.config == nil {
		return // Safe: no-op if engine is invalid
	}
	cfg := e.config
	if cfg.Observer != nil && cfg.Observer.OnDetected != nil {
		cfg.Observer.OnDetected(version, method)
	}
}

// notifyVersionMissing calls the observer callback when no version is detected.
func (e *Engine) notifyVersionMissing() {
	if e == nil || e.config == nil {
		return // Safe: no-op if engine is invalid
	}
	cfg := e.config
	if cfg.Observer != nil && cfg.Observer.OnMissing != nil {
		cfg.Observer.OnMissing()
	}
}

// Config returns the underlying configuration (for inspection/testing).
func (e *Engine) Config() *Config {
	return e.config
}
