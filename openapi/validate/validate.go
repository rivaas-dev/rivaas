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
	"errors"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ErrNoValidator indicates no validator is configured.
var ErrNoValidator = errors.New("no JSON Schema validator configured")

// Engine provides OpenAPI specification validation.
type Engine struct {
	compiler *jsonschema.Compiler
}

// New creates a new validation engine.
//
// The engine uses santhosh-tekuri/jsonschema which supports both
// JSON Schema draft-04 (for OpenAPI 3.0) and draft-2020-12 (for OpenAPI 3.1).
func New() *Engine {
	return &Engine{
		compiler: jsonschema.NewCompiler(),
	}
}

// ValidateJSON validates docJSON against schemaJSON.
//
// This method compiles the schema and validates the document.
// Returns an error if validation fails.
func (e *Engine) ValidateJSON(ctx context.Context, schemaJSON, docJSON []byte) error {
	if e.compiler == nil {
		return ErrNoValidator
	}

	// Compile schema
	schema, err := e.compiler.Compile("schema.json")
	if err != nil {
		// If compilation fails, try adding the schema as a resource first
		if err := e.compiler.AddResource("schema.json", bytes.NewReader(schemaJSON)); err != nil {
			return err
		}
		schema, err = e.compiler.Compile("schema.json")
		if err != nil {
			return err
		}
	}

	// Validate document
	if err := schema.Validate(bytes.NewReader(docJSON)); err != nil {
		return err
	}

	return nil
}

// ValidateOpenAPI validates an OpenAPI specification against its meta-schema.
//
// Parameters:
//   - version: "3.0.4" or "3.1.2"
//   - docJSON: The OpenAPI specification JSON bytes
//   - schema30: The OpenAPI 3.0.4 meta-schema JSON bytes
//   - schema31: The OpenAPI 3.1.2 meta-schema JSON bytes
//
// Returns an error if validation fails.
func (e *Engine) ValidateOpenAPI(ctx context.Context, version string, docJSON, schema30, schema31 []byte) error {
	if e.compiler == nil {
		return ErrNoValidator
	}

	var schemaJSON []byte
	switch version {
	case "3.0.4":
		schemaJSON = schema30
	case "3.1.2":
		schemaJSON = schema31
	default:
		return errors.New("unknown OpenAPI version: " + version)
	}

	return e.ValidateJSON(ctx, schemaJSON, docJSON)
}
