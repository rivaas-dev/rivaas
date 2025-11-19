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

package export_test

import (
	"context"
	"encoding/json"
	"fmt"

	"rivaas.dev/openapi/export"
	"rivaas.dev/openapi/model"
)

// ExampleProject demonstrates basic usage of Project to convert a spec to OpenAPI 3.0.
func ExampleProject() {
	spec := &model.Spec{
		Info: model.Info{
			Title:   "Example API",
			Version: "1.0.0",
		},
		Paths: map[string]*model.PathItem{
			"/users": {
				Get: &model.Operation{
					Summary: "List users",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
	}

	cfg := export.Config{
		Version: export.V30,
	}

	jsonBytes, warnings, err := export.Project(spec, cfg, nil, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("OpenAPI version: %s\n", result["openapi"])
	fmt.Printf("Warnings: %d\n", len(warnings))
	// Output:
	// OpenAPI version: 3.0.4
	// Warnings: 0
}

// ExampleProject_withValidator demonstrates Project with JSON Schema validation.
func ExampleProject_withValidator() {
	spec := &model.Spec{
		Info: model.Info{
			Title:   "Example API",
			Version: "1.0.0",
		},
		Paths: map[string]*model.PathItem{
			"/users": {
				Get: &model.Operation{
					Summary: "List users",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
	}

	validator := &mockValidator{}

	cfg := export.Config{
		Version:          export.V30,
		JSONSchemaEngine: validator,
	}

	// Empty schema for validation (in real usage, this would be the OpenAPI meta-schema)
	schema30 := []byte(`{}`)

	jsonBytes, warnings, err := export.Project(spec, cfg, schema30, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated spec length: %d bytes\n", len(jsonBytes))
	fmt.Printf("Warnings: %d\n", len(warnings))
	// Output:
	// Generated spec length: 289 bytes
	// Warnings: 0
}

// ExampleProject_withWarnings demonstrates Project generating warnings for 3.1-only features.
func ExampleProject_withWarnings() {
	spec := &model.Spec{
		Info: model.Info{
			Title:   "Example API",
			Version: "1.0.0",
			Summary: "API summary", // 3.1-only feature
		},
		Paths: map[string]*model.PathItem{
			"/users": {
				Get: &model.Operation{
					Summary: "List users",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
		Webhooks: map[string]*model.PathItem{
			"userCreated": {}, // 3.1-only feature
		},
	}

	cfg := export.Config{
		Version: export.V30,
	}

	jsonBytes, warnings, err := export.Project(spec, cfg, nil, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("OpenAPI version: %s\n", result["openapi"])
	fmt.Printf("Warnings: %d\n", len(warnings))
	for _, w := range warnings {
		fmt.Printf("Warning: %s - %s\n", w.Code, w.Message)
	}
	// Output:
	// OpenAPI version: 3.0.4
	// Warnings: 2
	// Warning: DOWNLEVEL_INFO_SUMMARY - info.summary is 3.1-only; dropped
	// Warning: DOWNLEVEL_WEBHOOKS - webhooks are 3.1-only; dropped
}

// ExampleProject_v31 demonstrates Project converting to OpenAPI 3.1.
func ExampleProject_v31() {
	spec := &model.Spec{
		Info: model.Info{
			Title:   "Example API",
			Version: "1.0.0",
			Summary: "API summary", // Supported in 3.1
		},
		Paths: map[string]*model.PathItem{
			"/users": {
				Get: &model.Operation{
					Summary: "List users",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
		Webhooks: map[string]*model.PathItem{
			"userCreated": {}, // Supported in 3.1
		},
	}

	cfg := export.Config{
		Version: export.V31,
	}

	jsonBytes, warnings, err := export.Project(spec, cfg, nil, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("OpenAPI version: %s\n", result["openapi"])
	fmt.Printf("Warnings: %d\n", len(warnings))
	// Output:
	// OpenAPI version: 3.1.2
	// Warnings: 0
}

// ExampleVersion demonstrates the Version constants.
func ExampleVersion() {
	fmt.Printf("OpenAPI 3.0: %s\n", export.V30)
	fmt.Printf("OpenAPI 3.1: %s\n", export.V31)
	// Output:
	// OpenAPI 3.0: 3.0.4
	// OpenAPI 3.1: 3.1.2
}

// ExampleWarning demonstrates Warning structure and codes.
func ExampleWarning() {
	warning := export.Warning{
		Code:    export.DOWNLEVEL_CONST_TO_ENUM,
		Path:    "#/components/schemas/User",
		Message: "const keyword not supported in 3.0; converted to enum",
	}

	fmt.Printf("Code: %s\n", warning.Code)
	fmt.Printf("Path: %s\n", warning.Path)
	fmt.Printf("Message: %s\n", warning.Message)
	// Output:
	// Code: DOWNLEVEL_CONST_TO_ENUM
	// Path: #/components/schemas/User
	// Message: const keyword not supported in 3.0; converted to enum
}

// mockValidator is a test implementation of JSONSchemaValidator.
type mockValidator struct{}

func (m *mockValidator) ValidateJSON(ctx context.Context, schemaJSON, docJSON []byte) error {
	// In a real implementation, this would validate docJSON against schemaJSON
	return nil
}
