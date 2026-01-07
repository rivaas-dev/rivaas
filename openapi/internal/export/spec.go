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

package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
	"rivaas.dev/openapi/validate"
)

// Version represents an OpenAPI specification version.
type Version string

const (
	// V30 represents OpenAPI 3.0.4.
	V30 Version = "3.0.4"
	// V31 represents OpenAPI 3.1.2.
	V31 Version = "3.1.2"
)

// Validator validates generated OpenAPI specifications.
type Validator interface {
	// Validate validates an OpenAPI specification JSON against its meta-schema.
	Validate(ctx context.Context, specJSON []byte, version validate.Version) error
}

// Config configures spec projection behavior.
type Config struct {
	// Version is the target OpenAPI version.
	Version Version

	// StrictDownlevel causes projection to error (instead of warn) when
	// 3.1-only features are used with a 3.0 target.
	StrictDownlevel bool

	// Validator is an optional validator for the generated specification.
	// If nil, no validation is performed.
	Validator Validator
}

// Result contains the output of spec projection.
type Result struct {
	// JSON is the marshaled OpenAPI specification as JSON bytes.
	JSON []byte

	// YAML is the marshaled OpenAPI specification as YAML bytes.
	YAML []byte

	// Warnings contains any warnings generated during projection.
	// Warnings are generated when 3.1-only features are used with a 3.0 target.
	Warnings diag.Warnings
}

// Project converts the internal model.Spec (IR) to a version-specific
// OpenAPI JSON and YAML specification (3.0.4 or 3.1.2).
//
// This function handles version-specific differences:
//   - Schema definitions (nullable vs type union, exclusive bounds)
//   - Keywords (const support, webhooks, mutualTLS)
//   - Reserved extension prefixes (x-oai-, x-oas- in 3.1.x)
//
// Features that exist only in 3.1.x are either:
//   - Dropped with warnings when projecting to 3.0.4
//   - Cause errors if StrictDownlevel is enabled
//
// The returned Result contains the marshaled JSON/YAML and any warnings generated
// during projection (e.g., info.summary dropped, license.identifier dropped).
//
// If a Validator is provided in cfg, the generated specification is validated
// against the appropriate OpenAPI meta-schema before returning.
func Project(ctx context.Context, spec *model.Spec, cfg Config) (Result, error) {
	if spec == nil {
		return Result{}, errors.New("nil spec")
	}

	var out any
	var warns diag.Warnings
	var err error

	switch cfg.Version {
	case V30:
		var spec30 *SpecV30
		spec30, warns, err = projectTo30(spec, cfg)
		if err != nil {
			return Result{Warnings: warns}, err
		}
		out = spec30

	case V31:
		var spec31 *SpecV31
		spec31, warns, err = projectTo31(spec)
		if err != nil {
			return Result{Warnings: warns}, err
		}
		out = spec31

	default:
		return Result{}, fmt.Errorf("unknown version: %s", cfg.Version)
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return Result{Warnings: warns}, fmt.Errorf("failed to marshal spec to JSON: %w", err)
	}

	// Validate if validator provided
	if cfg.Validator != nil {
		validatorVersion := validate.V30
		if cfg.Version == V31 {
			validatorVersion = validate.V31
		}
		if err = cfg.Validator.Validate(ctx, jsonBytes, validatorVersion); err != nil {
			return Result{Warnings: warns}, fmt.Errorf("spec validation failed: %w", err)
		}
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(out)
	if err != nil {
		return Result{Warnings: warns}, fmt.Errorf("failed to marshal spec to YAML: %w", err)
	}

	return Result{
		JSON:     jsonBytes,
		YAML:     yamlBytes,
		Warnings: warns,
	}, nil
}
