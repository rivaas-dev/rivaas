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

// Config holds the versioning engine configuration.
// This is configured via functional options passed to router.WithVersioning().
type Config struct {
	// Detection strategies (checked in order)
	detectors []Detector

	// Default version when none is detected
	defaultVersion string

	// Version validation
	validVersions []string

	// Global behavior options
	sendVersionHeader bool // Add X-API-Version to responses
	sendWarning299    bool // Add Warning: 299 for deprecated versions
	enforceSunset     bool // Return 410 Gone after sunset date

	// Per-version lifecycle configurations
	versionLifecycles map[string]*LifecycleConfig

	// Observer for version detection events
	observer *Observer

	// Clock function for testing
	now func() time.Time
}

// LifecycleConfig holds lifecycle configuration for a specific version.
// This is configured via lifecycle options passed to r.Version().
type LifecycleConfig struct {
	Deprecated      bool
	DeprecatedSince time.Time
	SunsetDate      time.Time
	MigrationURL    string
	Successor       string
}

// Detector defines the interface for version detection strategies.
type Detector interface {
	// Detect attempts to extract a version from the request.
	// Returns the detected version and true if found, empty string and false otherwise.
	Detect(req *http.Request) (version string, found bool)

	// Method returns the detection method name for observability.
	Method() string
}

// Observer holds callbacks for version detection events.
type Observer struct {
	// OnDetected is called when a version is successfully detected.
	OnDetected func(version, method string)

	// OnMissing is called when no version is detected (using default).
	OnMissing func()

	// OnInvalid is called when a detected version fails validation.
	OnInvalid func(attempted string)

	// OnDeprecatedUse is called when a deprecated version is accessed.
	OnDeprecatedUse func(version, route string)
}

// Option configures the versioning engine.
type Option func(*Config) error

// NewConfig creates a new Config with the given options.
func NewConfig(opts ...Option) (*Config, error) {
	cfg := &Config{
		defaultVersion:    "v1", // Sensible default
		versionLifecycles: make(map[string]*LifecycleConfig),
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validate checks the configuration for errors.
func (c *Config) validate() error {
	if c.defaultVersion == "" {
		return fmt.Errorf("%w: use version.WithDefault(\"v1\")", ErrDefaultRequired)
	}

	// Validate that path detectors have proper patterns
	for _, d := range c.detectors {
		if pd, ok := d.(*pathDetector); ok {
			if !strings.Contains(pd.pattern, "{version}") {
				return fmt.Errorf("%w: path pattern %q", ErrMissingVersionPlaceholder, pd.pattern)
			}
		}
	}

	return nil
}

// DefaultVersion returns the configured default version.
func (c *Config) DefaultVersion() string {
	return c.defaultVersion
}

// ValidVersions returns the list of valid versions, or nil if not configured.
func (c *Config) ValidVersions() []string {
	return c.validVersions
}

// SendVersionHeader returns whether to send X-API-Version response header.
func (c *Config) SendVersionHeader() bool {
	return c.sendVersionHeader
}

// SendWarning299 returns whether to send Warning: 299 for deprecated versions.
func (c *Config) SendWarning299() bool {
	return c.sendWarning299
}

// EnforceSunset returns whether to return 410 Gone for sunset versions.
func (c *Config) EnforceSunset() bool {
	return c.enforceSunset
}

// GetLifecycle returns the lifecycle config for a version, or nil if not configured.
func (c *Config) GetLifecycle(version string) *LifecycleConfig {
	if c.versionLifecycles == nil {
		return nil
	}
	return c.versionLifecycles[version]
}

// SetLifecycle sets the lifecycle config for a version.
func (c *Config) SetLifecycle(version string, lc *LifecycleConfig) {
	if c.versionLifecycles == nil {
		c.versionLifecycles = make(map[string]*LifecycleConfig)
	}
	c.versionLifecycles[version] = lc
}

// Now returns the current time (injectable for testing).
func (c *Config) Now() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// Observer returns the configured observer.
func (c *Config) Observer() *Observer {
	return c.observer
}

// Detectors returns the configured detectors.
func (c *Config) Detectors() []Detector {
	return c.detectors
}
