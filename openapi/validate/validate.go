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
package validate

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
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

// Validator validates OpenAPI specifications against their meta-schemas.
// Thread-safe. Compiles schemas once and caches them for reuse.
type Validator struct {
	compiler *jsonschema.Compiler
	schemas  map[Version]*jsonschema.Schema
	mu       sync.RWMutex
}

// New creates a new Validator with embedded OpenAPI meta-schemas.
//
// The validator uses santhosh-tekuri/jsonschema which supports both
// JSON Schema draft-04 (for OpenAPI 3.0) and draft-2020-12 (for OpenAPI 3.1).
func New() *Validator {
	return &Validator{
		compiler: jsonschema.NewCompiler(),
		schemas:  make(map[Version]*jsonschema.Schema),
	}
}

// Validate validates an OpenAPI specification JSON against the meta-schema for the given version.
//
// The schema is compiled on first use and cached for subsequent validations.
// This method is thread-safe.
//
// Example:
//
//	validator := validate.New()
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

// getOrCompile returns a cached compiled schema or compiles it on first access.
func (v *Validator) getOrCompile(version Version) (*jsonschema.Schema, error) {
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
