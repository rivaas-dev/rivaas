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

	"rivaas.dev/openapi/model"
)

// Version represents an OpenAPI specification version.
type Version string

const (
	// V30 represents OpenAPI 3.0.4.
	V30 Version = "3.0.4"
	// V31 represents OpenAPI 3.1.2.
	V31 Version = "3.1.2"
)

// Config configures spec projection behavior.
type Config struct {
	// Version is the target OpenAPI version.
	Version Version

	// StrictDownlevel causes projection to error (instead of warn) when
	// 3.1-only features are used with a 3.0 target.
	StrictDownlevel bool

	// JSONSchemaEngine is an optional JSON Schema validator.
	// If nil, no validation is performed.
	JSONSchemaEngine JSONSchemaValidator
}

// JSONSchemaValidator validates JSON documents against JSON Schema.
type JSONSchemaValidator interface {
	// ValidateJSON validates docJSON against schemaJSON.
	ValidateJSON(ctx context.Context, schemaJSON, docJSON []byte) error
}

// Project converts the internal model.Spec (IR) to a version-specific
// OpenAPI JSON specification (3.0.4 or 3.1.2).
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
// The returned warnings list contains information about any down-leveling
// that occurred (e.g., info.summary dropped, license.identifier dropped).
//
// If a JSONSchemaValidator is provided in cfg, the generated specification
// is validated against the appropriate OpenAPI meta-schema before returning.
//
// Returns the marshaled JSON bytes, any warnings generated during projection,
// and an error if projection or validation fails.
func Project(spec *model.Spec, cfg Config, schema30, schema31 []byte) ([]byte, []Warning, error) {
	if spec == nil {
		return nil, nil, errors.New("nil spec")
	}

	var out any
	var warns []Warning
	var err error

	switch cfg.Version {
	case V30:
		var spec30 *SpecV30
		spec30, warns, err = projectTo30(spec, cfg)
		if err != nil {
			return nil, warns, err
		}
		out = spec30

		// Validate if engine is provided.
		// Use context.Background() because Project() doesn't take a context parameter.
		// Schema validation is CPU-bound and typically fast, so cancellation is less critical.
		if cfg.JSONSchemaEngine != nil && len(schema30) > 0 {
			b, _ := json.Marshal(spec30)
			if validateErr := cfg.JSONSchemaEngine.ValidateJSON(context.Background(), schema30, b); validateErr != nil {
				return nil, warns, validateErr
			}
		}

	case V31:
		var spec31 *SpecV31
		spec31, warns, err = projectTo31(spec)
		if err != nil {
			return nil, warns, err
		}
		out = spec31

		// Validate if engine is provided.
		// Use context.Background() because Project() doesn't take a context parameter.
		// Schema validation is CPU-bound and typically fast, so cancellation is less critical.
		if cfg.JSONSchemaEngine != nil && len(schema31) > 0 {
			b, _ := json.Marshal(spec31)
			if validateErr := cfg.JSONSchemaEngine.ValidateJSON(context.Background(), schema31, b); validateErr != nil {
				return nil, warns, validateErr
			}
		}

	default:
		return nil, nil, errors.New("unknown version: " + string(cfg.Version))
	}

	// Marshal to JSON
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, warns, err
	}

	return b, warns, nil
}
