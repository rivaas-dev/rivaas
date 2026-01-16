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

//go:build !integration

package export

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	spec := &SpecV30{
		OpenAPI: "3.0.4",
		Info: &InfoV30{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Extensions: map[string]any{
			"x-custom-field": "value",
			"x-version":      2,
		},
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify base fields
	assert.Equal(t, "3.0.4", m["openapi"])
	assert.NotNil(t, m["info"])

	// Verify extensions are inlined
	assert.Equal(t, "value", m["x-custom-field"])
	assert.Equal(t, float64(2), m["x-version"]) //nolint:testifylint // exact integer comparison
}

func TestSpecV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	spec := &SpecV31{
		OpenAPI:           "3.1.2",
		JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/2024-11-10",
		Info: &InfoV31{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Extensions: map[string]any{
			"x-custom-field": "value",
			"x-array":        []string{"a", "b"},
		},
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify base fields
	assert.Equal(t, "3.1.2", m["openapi"])
	assert.NotNil(t, m["info"])

	// Verify extensions are inlined
	assert.Equal(t, "value", m["x-custom-field"])
	assert.Equal(t, []any{"a", "b"}, m["x-array"])
}

func TestInfoV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	info := &InfoV30{
		Title:   "Test API",
		Version: "1.0.0",
		Extensions: map[string]any{
			"x-api-id": "api-123",
		},
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "Test API", m["title"])
	assert.Equal(t, "1.0.0", m["version"])
	assert.Equal(t, "api-123", m["x-api-id"])
}

func TestServerV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	server := &ServerV30{
		URL:         "https://api.example.com",
		Description: "Production server",
		Extensions: map[string]any{
			"x-region": "us-east-1",
		},
	}

	data, err := json.Marshal(server)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "https://api.example.com", m["url"])
	assert.Equal(t, "Production server", m["description"])
	assert.Equal(t, "us-east-1", m["x-region"])
}

func TestParameterV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	param := &ParameterV30{
		Name:     "id",
		In:       "path",
		Required: true,
		Extensions: map[string]any{
			"x-deprecated": true,
		},
	}

	data, err := json.Marshal(param)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "id", m["name"])
	assert.Equal(t, "path", m["in"])
	assert.Equal(t, true, m["required"])
	assert.Equal(t, true, m["x-deprecated"])
}

func TestCallbackV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	callback := &CallbackV31{
		PathItems: map[string]*PathItemV31{
			"https://example.com/webhook": {
				Post: &OperationV31{
					OperationID: "webhook",
				},
			},
		},
		Extensions: map[string]any{
			"x-callback-id": "cb-123",
		},
	}

	data, err := json.Marshal(callback)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify path item is at top level
	assert.NotNil(t, m["https://example.com/webhook"])
	// Verify extension is inlined
	assert.Equal(t, "cb-123", m["x-callback-id"])
}

func TestSchemaV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	schema := &SchemaV30{
		Type:        "string",
		Description: "A string field",
		Extensions: map[string]any{
			"x-format": "email",
		},
	}

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "string", m["type"])
	assert.Equal(t, "A string field", m["description"])
	assert.Equal(t, "email", m["x-format"])
}

func TestSchemaV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	schema := &SchemaV31{
		Type:        "string",
		Description: "A string field",
		Extensions: map[string]any{
			"x-format": "email",
		},
	}

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "string", m["type"])
	assert.Equal(t, "A string field", m["description"])
	assert.Equal(t, "email", m["x-format"])
}

func TestNestedExtensions(t *testing.T) {
	t.Parallel()

	// Test that extensions work in nested structures
	info := &InfoV30{
		Title:   "Test API",
		Version: "1.0.0",
		Contact: &ContactV30{
			Name: "Support",
			Extensions: map[string]any{
				"x-contact-id": "contact-123",
			},
		},
		License: &LicenseV30{
			Name: "MIT",
			Extensions: map[string]any{
				"x-license-id": "license-456",
			},
		},
		Extensions: map[string]any{
			"x-api-id": "api-789",
		},
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Verify root-level extension
	assert.Equal(t, "api-789", m["x-api-id"])

	// Verify nested contact extension
	contact, ok := m["contact"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "contact-123", contact["x-contact-id"])

	// Verify nested license extension
	license, ok := m["license"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "license-456", license["x-license-id"])
}
