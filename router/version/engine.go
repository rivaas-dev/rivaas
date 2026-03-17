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
	"slices"
	"strings"
	"time"
)

// Engine manages API versioning, including version detection from requests
// and lifecycle header management.
//
// An Engine must be created with [New] or [MustNew]. Do not call methods on a nil Engine;
// callers (e.g. the router) must guard with a non-nil engine before use.
type Engine struct {
	config *config
}

// New creates a new versioning engine with the given options.
//
// Example:
//
//	engine, err := version.New(
//	    version.WithHeaderDetection("X-API-Version"),
//	    version.WithDefault("v1"),
//	)
func New(opts ...Option) (*Engine, error) {
	cfg, err := newConfig(opts...)
	if err != nil {
		return nil, err
	}

	return &Engine{config: cfg}, nil
}

// MustNew creates a new versioning engine with the given options.
// It panics if configuration is invalid.
// Use in main() or init() where panic on startup is acceptable.
//
// Example:
//
//	engine := version.MustNew(
//	    version.WithHeaderDetection("X-API-Version"),
//	    version.WithDefault("v1"),
//	)
func MustNew(opts ...Option) *Engine {
	e, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("version.MustNew: %v", err))
	}
	return e
}

// DetectVersion detects the API version from the request.
// Checks detectors in order until one returns a version.
// Falls back to default version if none found.
func (e *Engine) DetectVersion(req *http.Request) string {
	if req == nil {
		return e.config.defaultVersion
	}

	// Try each detector in order
	for _, detector := range e.config.detectors {
		if version, found := detector.Detect(req); found {
			validated := e.validateVersion(version)
			if validated != "" {
				e.notifyDetected(validated, detector.Method())
				return validated
			}
		}
	}

	// No version detected
	e.notifyMissing()

	return e.config.defaultVersion
}

// validateVersion checks if a version is valid.
// Returns the version if valid, empty string if invalid.
func (e *Engine) validateVersion(version string) string {
	if version == "" {
		return ""
	}

	cfg := e.config
	if len(cfg.validVersions) == 0 {
		return version // No validation configured
	}

	if slices.Contains(cfg.validVersions, version) {
		return version
	}

	// Invalid version
	e.notifyInvalid(version)

	return ""
}

func (e *Engine) notifyDetected(version, method string) {
	if e.config.observer != nil && e.config.observer.OnDetected != nil {
		e.config.observer.OnDetected(version, method)
	}
}

func (e *Engine) notifyMissing() {
	if e.config.observer != nil && e.config.observer.OnMissing != nil {
		e.config.observer.OnMissing()
	}
}

func (e *Engine) notifyInvalid(version string) {
	if e.config.observer != nil && e.config.observer.OnInvalid != nil {
		e.config.observer.OnInvalid(version)
	}
}

// ShouldApplyVersioning determines if versioning should be applied to this path.
func (e *Engine) ShouldApplyVersioning(path string) bool {
	// If no path detectors, always apply (header/query/accept work everywhere)
	hasPathDetector := false
	for _, d := range e.config.detectors {
		if _, ok := d.(*pathDetector); ok {
			hasPathDetector = true
			break
		}
	}

	if !hasPathDetector {
		return true
	}

	// Check if path matches any path detector
	for _, d := range e.config.detectors {
		if pd, ok := d.(*pathDetector); ok {
			if _, found := pd.extractFromPath(path); found {
				return true
			}
		}
	}

	// No version in path, but we have a default
	return e.config.defaultVersion != ""
}

// ExtractPathSegment extracts the version segment from a path for stripping.
func (e *Engine) ExtractPathSegment(path string) (string, bool) {
	for _, d := range e.config.detectors {
		if pd, ok := d.(*pathDetector); ok {
			if segment, found := pd.ExtractSegment(path); found {
				return segment, true
			}
		}
	}

	return "", false
}

// StripPathVersion removes the version segment from a path.
func (e *Engine) StripPathVersion(path, version string) string {
	for _, d := range e.config.detectors {
		if pd, ok := d.(*pathDetector); ok {
			stripped := pd.StripVersion(path, version)
			if stripped != path {
				return stripped
			}
		}
	}

	return path
}

// SetLifecycleHeaders sets response headers for version lifecycle (deprecation, sunset).
// Returns true if the version has passed its sunset date (caller should return 410 Gone).
func (e *Engine) SetLifecycleHeaders(w http.ResponseWriter, version, route string) bool {
	if w == nil {
		return false
	}

	cfg := e.config

	// Always set version header if enabled
	if cfg.sendVersionHeader && version != "" {
		w.Header().Set("X-API-Version", version)
	}

	// Get lifecycle config for this version
	lc := cfg.getLifecycle(version)
	if lc == nil || !lc.deprecated {
		return false // Not deprecated
	}

	now := cfg.nowTime()

	// Check if version has been sunset
	if cfg.enforceSunset && !lc.sunsetDate.IsZero() && now.After(lc.sunsetDate) {
		// Version is past sunset - set headers and return true
		w.Header().Set("Sunset", lc.sunsetDate.UTC().Format(http.TimeFormat))
		if lc.migrationURL != "" {
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"sunset\"", lc.migrationURL))
		}

		return true
	}

	// Version is deprecated but not yet sunset
	w.Header().Set("Deprecation", "true")
	if !lc.sunsetDate.IsZero() {
		w.Header().Set("Sunset", lc.sunsetDate.UTC().Format(http.TimeFormat))
	}

	// Add Link headers for documentation
	if lc.migrationURL != "" {
		linkHeaders := []string{
			fmt.Sprintf("<%s>; rel=\"deprecation\"", lc.migrationURL),
		}
		if !lc.sunsetDate.IsZero() {
			linkHeaders = append(linkHeaders, fmt.Sprintf("<%s>; rel=\"sunset\"", lc.migrationURL))
		}
		w.Header().Set("Link", strings.Join(linkHeaders, ", "))
	}

	// Add Warning: 299 header if enabled
	if cfg.sendWarning299 {
		warningMsg := fmt.Sprintf("299 - \"API %s is deprecated", version)
		if !lc.sunsetDate.IsZero() {
			warningMsg += " and will be removed on " + lc.sunsetDate.Format(time.RFC3339)
		}
		warningMsg += ". Please upgrade to a supported version.\""
		w.Header().Set("Warning", warningMsg)
	}

	// Call deprecated usage callback
	if cfg.observer != nil && cfg.observer.OnDeprecatedUse != nil {
		cfg.observer.OnDeprecatedUse(version, route)
	}

	return false
}

// DefaultVersion returns the configured default version (e.g. when none is detected).
func (e *Engine) DefaultVersion() string {
	return e.config.defaultVersion
}

// ApplyLifecycle applies lifecycle options for a version.
// Options are merged with any existing lifecycle for that version (e.g. from a previous Version() or Configure() call).
// Used by the router when r.Version("v1", opts...) or VersionRouter.Configure(opts...) is called.
// Returns an error if any lifecycle option is nil.
func (e *Engine) ApplyLifecycle(version string, opts ...LifecycleOption) error {
	if version == "" || len(opts) == 0 {
		return nil
	}
	lc := e.config.getLifecycle(version)
	if lc == nil {
		var err error
		lc, err = applyLifecycleOptions(opts...)
		if err != nil {
			return err
		}
	} else {
		for i, opt := range opts {
			if opt == nil {
				return fmt.Errorf("version: lifecycle option at index %d cannot be nil", i)
			}
			opt(lc)
		}
	}
	e.config.setLifecycle(version, lc)
	return nil
}
