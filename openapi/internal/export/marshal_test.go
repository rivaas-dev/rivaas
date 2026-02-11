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

func TestContactV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	c := &ContactV30{
		Name:       "Support",
		URL:        "https://example.com",
		Email:      "support@example.com",
		Extensions: map[string]any{"x-contact-id": "c1"},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Support", m["name"])
	assert.Equal(t, "c1", m["x-contact-id"])
}

func TestLicenseV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	l := &LicenseV30{
		Name:       "MIT",
		URL:        "https://opensource.org/licenses/MIT",
		Extensions: map[string]any{"x-license": "mit"},
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "MIT", m["name"])
	assert.Equal(t, "mit", m["x-license"])
}

func TestServerVariableV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	v := &ServerVariableV30{
		Default:     "api",
		Enum:        []string{"api", "staging"},
		Description: "Environment",
		Extensions:  map[string]any{"x-var": "env"},
	}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "api", m["default"])
	assert.Equal(t, "env", m["x-var"])
}

func TestPathItemV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	p := &PathItemV30{
		Summary:     "Test path",
		Description: "A path",
		Extensions:  map[string]any{"x-path": "p1"},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Test path", m["summary"])
	assert.Equal(t, "p1", m["x-path"])
}

func TestOperationV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OperationV30{
		OperationID: "getTest",
		Summary:     "Get",
		Responses:   map[string]*ResponseV30{"200": {Description: "OK"}},
		Extensions:  map[string]any{"x-op": "op1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "getTest", m["operationId"])
	assert.Equal(t, "op1", m["x-op"])
}

func TestRequestBodyV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	r := &RequestBodyV30{
		Description: "Body",
		Required:    true,
		Extensions:  map[string]any{"x-body": "b1"},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Body", m["description"])
	assert.Equal(t, "b1", m["x-body"])
}

func TestResponseV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	r := &ResponseV30{
		Description: "Success",
		Extensions:  map[string]any{"x-resp": "r1"},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Success", m["description"])
	assert.Equal(t, "r1", m["x-resp"])
}

func TestHeaderV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	h := &HeaderV30{
		Description: "A header",
		Required:    true,
		Extensions:  map[string]any{"x-header": "h1"},
	}
	data, err := json.Marshal(h)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "A header", m["description"])
	assert.Equal(t, "h1", m["x-header"])
}

func TestMediaTypeV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	m := &MediaTypeV30{
		Schema:     &SchemaV30{Type: "string"},
		Example:    "example",
		Extensions: map[string]any{"x-mt": "mt1"},
	}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "mt1", out["x-mt"])
}

func TestExampleV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &ExampleV30{
		Summary:    "Example",
		Value:      "val",
		Extensions: map[string]any{"x-ex": "ex1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Example", m["summary"])
	assert.Equal(t, "ex1", m["x-ex"])
}

func TestEncodingV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &EncodingV30{
		ContentType: "application/json",
		Style:       "form",
		Extensions:  map[string]any{"x-enc": "enc1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "application/json", m["contentType"])
	assert.Equal(t, "enc1", m["x-enc"])
}

func TestLinkV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	l := &LinkV30{
		OperationID: "getUser",
		Description: "Link to getUser",
		Extensions:  map[string]any{"x-link": "link1"},
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "getUser", m["operationId"])
	assert.Equal(t, "link1", m["x-link"])
}

func TestCallbackV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	cb := &CallbackV30{
		PathItems: map[string]*PathItemV30{
			"https://example.com/cb": {
				Post: &OperationV30{
					OperationID: "onEvent",
					Responses:   map[string]*ResponseV30{"200": {Description: "OK"}},
				},
			},
		},
		Extensions: map[string]any{"x-cb": "cb1"},
	}
	data, err := json.Marshal(cb)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["https://example.com/cb"])
	assert.Equal(t, "cb1", m["x-cb"])
}

func TestComponentsV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	c := &ComponentsV30{
		Schemas:    map[string]*SchemaV30{"User": {Type: "object"}},
		Extensions: map[string]any{"x-comp": "comp1"},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["schemas"])
	assert.Equal(t, "comp1", m["x-comp"])
}

func TestSecuritySchemeV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	s := &SecuritySchemeV30{
		Type:        "http",
		Scheme:      "bearer",
		Description: "Bearer auth",
		Extensions:  map[string]any{"x-ss": "ss1"},
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "http", m["type"])
	assert.Equal(t, "ss1", m["x-ss"])
}

func TestOAuthFlowsV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OAuthFlowsV30{
		Implicit:   &OAuthFlowV30{AuthorizationURL: "https://auth.example.com", Scopes: map[string]string{"read": "Read"}},
		Extensions: map[string]any{"x-flows": "f1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["implicit"])
	assert.Equal(t, "f1", m["x-flows"])
}

func TestOAuthFlowV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OAuthFlowV30{
		AuthorizationURL: "https://auth.example.com",
		TokenURL:         "https://token.example.com",
		Scopes:           map[string]string{"read": "Read"},
		Extensions:       map[string]any{"x-flow": "flow1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://auth.example.com", m["authorizationUrl"])
	assert.Equal(t, "flow1", m["x-flow"])
}

func TestTagV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	tag := &TagV30{
		Name:        "users",
		Description: "User operations",
		Extensions:  map[string]any{"x-tag": "tag1"},
	}
	data, err := json.Marshal(tag)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "users", m["name"])
	assert.Equal(t, "tag1", m["x-tag"])
}

func TestExternalDocsV30_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &ExternalDocsV30{
		URL:         "https://docs.example.com",
		Description: "External docs",
		Extensions:  map[string]any{"x-docs": "docs1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://docs.example.com", m["url"])
	assert.Equal(t, "docs1", m["x-docs"])
}

func TestInfoV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	info := &InfoV31{
		Title:      "Test API",
		Version:    "1.0.0",
		Summary:    "Summary",
		Extensions: map[string]any{"x-api": "api1"},
	}
	data, err := json.Marshal(info)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Test API", m["title"])
	assert.Equal(t, "api1", m["x-api"])
}

func TestContactV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	c := &ContactV31{
		Name:       "Support",
		Extensions: map[string]any{"x-contact": "c1"},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Support", m["name"])
	assert.Equal(t, "c1", m["x-contact"])
}

func TestLicenseV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	l := &LicenseV31{
		Name:       "MIT",
		Identifier: "MIT",
		Extensions: map[string]any{"x-lic": "lic1"},
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "MIT", m["name"])
	assert.Equal(t, "lic1", m["x-lic"])
}

func TestServerV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	s := &ServerV31{
		URL:         "https://api.example.com",
		Description: "API server",
		Extensions:  map[string]any{"x-server": "s1"},
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://api.example.com", m["url"])
	assert.Equal(t, "s1", m["x-server"])
}

func TestServerVariableV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	v := &ServerVariableV31{
		Default:    "api",
		Enum:       []string{"api", "staging"},
		Extensions: map[string]any{"x-var": "v1"},
	}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "api", m["default"])
	assert.Equal(t, "v1", m["x-var"])
}

func TestPathItemV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	p := &PathItemV31{
		Summary:     "Path",
		Description: "A path",
		Extensions:  map[string]any{"x-path": "p1"},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Path", m["summary"])
	assert.Equal(t, "p1", m["x-path"])
}

func TestOperationV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OperationV31{
		OperationID: "getItem",
		Summary:     "Get item",
		Responses:   map[string]*ResponseV31{"200": {Description: "OK"}},
		Extensions:  map[string]any{"x-op": "op1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "getItem", m["operationId"])
	assert.Equal(t, "op1", m["x-op"])
}

func TestParameterV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	p := &ParameterV31{
		Name:       "id",
		In:         "path",
		Required:   true,
		Extensions: map[string]any{"x-param": "param1"},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "id", m["name"])
	assert.Equal(t, "param1", m["x-param"])
}

func TestExampleV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &ExampleV31{
		Summary:    "Example",
		Value:      "value",
		Extensions: map[string]any{"x-ex": "ex1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Example", m["summary"])
	assert.Equal(t, "ex1", m["x-ex"])
}

func TestRequestBodyV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	r := &RequestBodyV31{
		Description: "Request body",
		Required:    true,
		Extensions:  map[string]any{"x-rb": "rb1"},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Request body", m["description"])
	assert.Equal(t, "rb1", m["x-rb"])
}

func TestResponseV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	r := &ResponseV31{
		Description: "Success",
		Extensions:  map[string]any{"x-resp": "r1"},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Success", m["description"])
	assert.Equal(t, "r1", m["x-resp"])
}

func TestHeaderV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	h := &HeaderV31{
		Description: "Header",
		Required:    true,
		Extensions:  map[string]any{"x-header": "h1"},
	}
	data, err := json.Marshal(h)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "Header", m["description"])
	assert.Equal(t, "h1", m["x-header"])
}

func TestMediaTypeV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	m := &MediaTypeV31{
		Schema:     &SchemaV31{Type: "string"},
		Example:    "example",
		Extensions: map[string]any{"x-mt": "mt1"},
	}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "mt1", out["x-mt"])
}

func TestEncodingV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &EncodingV31{
		ContentType: "application/json",
		Style:       "form",
		Extensions:  map[string]any{"x-enc": "enc1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "application/json", m["contentType"])
	assert.Equal(t, "enc1", m["x-enc"])
}

func TestLinkV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	l := &LinkV31{
		OperationID: "getUser",
		Description: "Link",
		Extensions:  map[string]any{"x-link": "link1"},
	}
	data, err := json.Marshal(l)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "getUser", m["operationId"])
	assert.Equal(t, "link1", m["x-link"])
}

func TestComponentsV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	c := &ComponentsV31{
		Schemas:    map[string]*SchemaV31{"User": {Type: "object"}},
		PathItems:  map[string]*PathItemV31{"reusable": {Summary: "Reusable"}},
		Extensions: map[string]any{"x-comp": "comp1"},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["schemas"])
	assert.Equal(t, "comp1", m["x-comp"])
}

func TestSecuritySchemeV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	s := &SecuritySchemeV31{
		Type:        "http",
		Scheme:      "bearer",
		Description: "Bearer",
		Extensions:  map[string]any{"x-ss": "ss1"},
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "http", m["type"])
	assert.Equal(t, "ss1", m["x-ss"])
}

func TestOAuthFlowsV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OAuthFlowsV31{
		AuthorizationCode: &OAuthFlowV31{
			AuthorizationURL: "https://auth.example.com",
			TokenURL:         "https://token.example.com",
			Scopes:           map[string]string{"read": "Read"},
		},
		Extensions: map[string]any{"x-flows": "f1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["authorizationCode"])
	assert.Equal(t, "f1", m["x-flows"])
}

func TestOAuthFlowV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	o := &OAuthFlowV31{
		AuthorizationURL: "https://auth.example.com",
		TokenURL:         "https://token.example.com",
		Scopes:           map[string]string{"read": "Read"},
		Extensions:       map[string]any{"x-flow": "flow1"},
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://auth.example.com", m["authorizationUrl"])
	assert.Equal(t, "flow1", m["x-flow"])
}

func TestTagV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	tag := &TagV31{
		Name:        "users",
		Description: "User ops",
		Extensions:  map[string]any{"x-tag": "tag1"},
	}
	data, err := json.Marshal(tag)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "users", m["name"])
	assert.Equal(t, "tag1", m["x-tag"])
}

func TestExternalDocsV31_MarshalJSON_WithExtensions(t *testing.T) {
	t.Parallel()

	e := &ExternalDocsV31{
		URL:         "https://docs.example.com",
		Description: "Docs",
		Extensions:  map[string]any{"x-docs": "docs1"},
	}
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://docs.example.com", m["url"])
	assert.Equal(t, "docs1", m["x-docs"])
}
