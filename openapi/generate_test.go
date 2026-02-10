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

package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_Generate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		api      *API
		ops      []Operation
		validate func(t *testing.T, spec map[string]any)
	}

	tests := []testCase{
		{
			name: "minimal API produces valid spec",
			api:  MustNew(WithTitle("Minimal API", "1.0.0")),
			ops: []Operation{
				GET("/health", WithSummary("Health check")),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				assert.Equal(t, "3.0.4", spec["openapi"])
				info, ok := spec["info"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Minimal API", info["title"])
				assert.Equal(t, "1.0.0", info["version"])
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				_, hasHealth := paths["/health"]
				assert.True(t, hasHealth)
			},
		},
		{
			name: "with servers and tags",
			api: MustNew(
				WithTitle("API", "1.0.0"),
				WithServer("https://api.example.com", "Production"),
				WithTag("users", "User operations"),
			),
			ops: []Operation{
				GET("/users", WithSummary("List users"), WithTags("users")),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				servers, ok := spec["servers"].([]any)
				require.True(t, ok)
				require.Len(t, servers, 1)
				srv, ok := servers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "https://api.example.com", srv["url"])
				tags, ok := spec["tags"].([]any)
				require.True(t, ok)
				require.Len(t, tags, 1)
				tag, ok := tags[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "users", tag["name"])
			},
		},
		{
			name: "with security schemes and default security",
			api: MustNew(
				WithTitle("API", "1.0.0"),
				WithBearerAuth("bearerAuth", "JWT"),
				WithDefaultSecurity("bearerAuth"),
			),
			ops: []Operation{
				GET("/protected", WithSummary("Protected"), WithSecurity("bearerAuth")),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				components, ok := spec["components"].(map[string]any)
				require.True(t, ok)
				schemes, ok := components["securitySchemes"].(map[string]any)
				require.True(t, ok)
				_, hasBearer := schemes["bearerAuth"]
				assert.True(t, hasBearer)
				security, ok := spec["security"].([]any)
				require.True(t, ok)
				require.Len(t, security, 1)
			},
		},
		{
			name: "with external docs",
			api: MustNew(
				WithTitle("API", "1.0.0"),
				WithExternalDocs("https://example.com/docs", "API documentation"),
			),
			ops: []Operation{
				GET("/", WithSummary("Root")),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				extDocs, ok := spec["externalDocs"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "https://example.com/docs", extDocs["url"])
				assert.Equal(t, "API documentation", extDocs["description"])
			},
		},
		{
			name: "operation with request and response types",
			api:  MustNew(WithTitle("API", "1.0.0")),
			ops: []Operation{
				POST("/users",
					WithSummary("Create user"),
					WithRequest(struct {
						Name string `json:"name"`
					}{}),
					WithResponse(http.StatusCreated, struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{}),
				),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				pathItem, ok := paths["/users"].(map[string]any)
				require.True(t, ok)
				postOp, ok := pathItem["post"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Create user", postOp["summary"])
				_, hasRequestBody := postOp["requestBody"]
				assert.True(t, hasRequestBody)
				responses, ok := postOp["responses"].(map[string]any)
				require.True(t, ok)
				_, has201 := responses["201"]
				assert.True(t, has201)
			},
		},
		{
			name: "version 3.1 produces 3.1.2 spec",
			api:  MustNew(WithTitle("API", "1.0.0"), WithVersion(V31x)),
			ops: []Operation{
				GET("/health", WithSummary("Health")),
			},
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				assert.Equal(t, "3.1.2", spec["openapi"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			result, err := tt.api.Generate(ctx, tt.ops...)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotEmpty(t, result.JSON)

			var spec map[string]any
			err = json.Unmarshal(result.JSON, &spec)
			require.NoError(t, err)

			tt.validate(t, spec)
		})
	}
}

func TestAPI_Generate_ErrorCase(t *testing.T) {
	t.Parallel()

	t.Run("empty operations with default version returns error", func(t *testing.T) {
		t.Parallel()

		api := MustNew(WithTitle("API", "1.0.0"))
		ctx := context.Background()

		result, err := api.Generate(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "paths")
	})
}
