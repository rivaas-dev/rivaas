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

// Package validate provides OpenAPI specification validation functionality.
// It validates OpenAPI specs against the official JSON Schema definitions.
//
// Create a validator with [New] or [MustNew]. Options are optional; use
// [WithVersions] to restrict which OpenAPI versions are accepted. Use
// [Validator.Validate] when you know the version, or [Validator.ValidateAuto]
// to detect the version from the spec and validate in one call.
package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"rivaas.dev/openapi/internal/metaschema"
)

// Version represents OpenAPI specification version.
type Version string

const (
	// V30 represents OpenAPI 3.0.x.
	V30 Version = "3.0"
	// V31 represents OpenAPI 3.1.x.
	V31 Version = "3.1"
)

var (
	// validResponseCodePattern matches OpenAPI response codes: 1XX-5XX, default, or wildcards like 4XX
	// Per spec: ^[1-5](?:\d{2}|XX)$ or "default"
	validResponseCodePattern = regexp.MustCompile(`^(default|[1-5](\d{2}|XX))$`)

	// validComponentNamePattern matches OpenAPI component names: ^[a-zA-Z0-9.\-_]+$
	validComponentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9.\-_]+$`)
)

// config holds construction-time validator configuration.
// Options mutate config; New builds the Validator from it.
type config struct {
	// allowedVersions restricts which OpenAPI versions the validator accepts.
	// Nil or empty means allow all supported versions (V30, V31).
	allowedVersions map[Version]struct{}
}

// Option configures the validator using the functional options pattern.
// Options are applied in order. No options are required; defaults work for typical use.
type Option func(*config)

func defaultConfig() *config {
	return &config{}
}

// WithVersions restricts the validator to the given OpenAPI versions.
// Only specs for these versions will be compiled and accepted. Pass no arguments
// or both V30 and V31 to allow all supported versions (default behavior).
// Example: WithVersions(V31) for OpenAPI 3.1 only.
func WithVersions(versions ...Version) Option {
	return func(c *config) {
		if len(versions) == 0 {
			c.allowedVersions = nil
			return
		}
		c.allowedVersions = make(map[Version]struct{}, len(versions))
		for _, v := range versions {
			if v == V30 || v == V31 {
				c.allowedVersions[v] = struct{}{}
			}
		}
	}
}

// Validator validates OpenAPI specifications against their meta-schemas.
// Thread-safe. Compiles schemas once and caches them for reuse.
type Validator struct {
	compiler        *jsonschema.Compiler
	schemas         map[Version]*jsonschema.Schema
	allowedVersions map[Version]struct{} // nil or empty = allow all supported
	mu              sync.RWMutex
}

// New creates a new Validator with the given options.
// Construction currently cannot fail; the error return is for API consistency with other Rivaas packages.
//
// The validator uses santhosh-tekuri/jsonschema which supports both
// JSON Schema draft-04 (for OpenAPI 3.0) and draft-2020-12 (for OpenAPI 3.1).
func New(opts ...Option) (*Validator, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	v := &Validator{
		compiler: jsonschema.NewCompiler(),
		schemas:  make(map[Version]*jsonschema.Schema),
	}
	if len(cfg.allowedVersions) > 0 {
		v.allowedVersions = make(map[Version]struct{}, len(cfg.allowedVersions))
		for ver := range cfg.allowedVersions {
			v.allowedVersions[ver] = struct{}{}
		}
	}
	return v, nil
}

// MustNew creates a new Validator with the given options.
// Panics if construction fails. Use in main() or init() where panic on startup is acceptable.
func MustNew(opts ...Option) *Validator {
	v, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("validate.MustNew: %v", err))
	}
	return v
}

// Validate validates an OpenAPI specification JSON against the meta-schema for the given version.
//
// The schema is compiled on first use and cached for subsequent validations.
// This method is thread-safe.
//
// Example:
//
//	validator := validate.MustNew()
//	if err := validator.Validate(ctx, specJSON, validate.V31); err != nil {
//	    log.Fatalf("Invalid OpenAPI spec: %v", err)
//	}
func (v *Validator) Validate(ctx context.Context, specJSON []byte, version Version) error {
	schema, err := v.getOrCompile(version)
	if err != nil {
		return err
	}
	return schema.Validate(bytes.NewReader(specJSON))
}

// ValidateAuto detects the OpenAPI version from the spec's "openapi" field and validates
// the specification against the corresponding meta-schema. Use this for a single-call
// "validate this spec" flow when the version is not known in advance.
// Returns an error if the spec is invalid JSON, missing the "openapi" field, or uses
// an unsupported version (only 3.0.x and 3.1.x are supported).
func (v *Validator) ValidateAuto(ctx context.Context, specJSON []byte) error {
	var minimal struct {
		OpenAPI string `json:"openapi"`
	}
	if err := json.Unmarshal(specJSON, &minimal); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	s := strings.TrimSpace(minimal.OpenAPI)
	if s == "" {
		return fmt.Errorf("missing \"openapi\" field in specification")
	}
	var version Version
	switch {
	case strings.HasPrefix(s, "3.0"):
		version = V30
	case strings.HasPrefix(s, "3.1"):
		version = V31
	default:
		return fmt.Errorf("unsupported openapi version %q (use 3.0.x or 3.1.x)", s)
	}
	return v.Validate(ctx, specJSON, version)
}

// getOrCompile returns a cached compiled schema or compiles it on first access.
func (v *Validator) getOrCompile(version Version) (*jsonschema.Schema, error) {
	if len(v.allowedVersions) > 0 {
		if _, ok := v.allowedVersions[version]; !ok {
			allowed := make([]string, 0, len(v.allowedVersions))
			for ver := range v.allowedVersions {
				allowed = append(allowed, string(ver))
			}
			return nil, fmt.Errorf("openapi version %s not allowed by this validator (allowed: %s)", version, strings.Join(allowed, ", "))
		}
	}

	// Fast path: read lock for cache hit
	v.mu.RLock()
	if s, ok := v.schemas[version]; ok {
		v.mu.RUnlock()
		return s, nil
	}
	v.mu.RUnlock()

	// Slow path: write lock to compile and cache
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have compiled it)
	if s, ok := v.schemas[version]; ok {
		return s, nil
	}

	var schemaJSON []byte
	switch version {
	case V30:
		schemaJSON = metaschema.OAS30
	case V31:
		schemaJSON = metaschema.OAS31
	default:
		return nil, fmt.Errorf("unsupported OpenAPI version: %s (use V30 or V31)", version)
	}

	// Compile and cache schema
	resourceName := fmt.Sprintf("openapi-%s.json", version)
	if err := v.compiler.AddResource(resourceName, bytes.NewReader(schemaJSON)); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	schema, err := v.compiler.Compile(resourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	v.schemas[version] = schema
	return schema, nil
}

// ValidateResponseCode validates an OpenAPI response code.
// Valid codes: "default", "200", "404", "4XX", "5XX", etc.
// Returns an error if the code is invalid.
func ValidateResponseCode(code string) error {
	if !validResponseCodePattern.MatchString(code) {
		return fmt.Errorf("invalid response code %q: must be 'default' or match pattern [1-5](XX|\\d{2})", code)
	}
	return nil
}

// ValidateComponentName validates an OpenAPI component name.
// Valid names must match: ^[a-zA-Z0-9.\-_]+$
// Returns an error if the name is invalid.
func ValidateComponentName(name string) error {
	if name == "" {
		return fmt.Errorf("component name cannot be empty")
	}
	if !validComponentNamePattern.MatchString(name) {
		return fmt.Errorf("invalid component name %q: must match pattern [a-zA-Z0-9.\\-_]+", name)
	}
	return nil
}
