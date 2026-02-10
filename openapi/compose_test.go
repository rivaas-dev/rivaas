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

func TestWithOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		op       Operation
		validate func(t *testing.T, spec map[string]any)
	}{
		{
			name: "single option applies correctly",
			op: GET("/health",
				WithOptions(WithSummary("Health check")),
			),
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				pathItem, ok := paths["/health"].(map[string]any)
				require.True(t, ok)
				getOp, ok := pathItem["get"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Health check", getOp["summary"])
			},
		},
		{
			name: "multiple options apply in order",
			op: GET("/users",
				WithOptions(
					WithSummary("List users"),
					WithTags("users"),
					WithResponse(http.StatusOK, []struct{}{}),
				),
			),
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				pathItem, ok := paths["/users"].(map[string]any)
				require.True(t, ok)
				getOp, ok := pathItem["get"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "List users", getOp["summary"])
				tags, ok := getOp["tags"].([]any)
				require.True(t, ok)
				require.Len(t, tags, 1)
				assert.Equal(t, "users", tags[0])
				responses, ok := getOp["responses"].(map[string]any)
				require.True(t, ok)
				_, has200 := responses["200"]
				assert.True(t, has200)
			},
		},
		{
			name: "later option overrides earlier",
			op: GET("/item",
				WithOptions(
					WithSummary("First summary"),
					WithSummary("Final summary"),
				),
			),
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				pathItem, ok := paths["/item"].(map[string]any)
				require.True(t, ok)
				getOp, ok := pathItem["get"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Final summary", getOp["summary"])
			},
		},
		{
			name: "empty options is valid",
			op:   GET("/root", WithOptions()),
			validate: func(t *testing.T, spec map[string]any) {
				t.Helper()
				paths, ok := spec["paths"].(map[string]any)
				require.True(t, ok)
				pathItem, ok := paths["/root"].(map[string]any)
				require.True(t, ok)
				_, hasGet := pathItem["get"]
				assert.True(t, hasGet)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := MustNew(WithTitle("Test API", "1.0.0"))
			ctx := context.Background()

			result, err := api.Generate(ctx, tt.op)
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
