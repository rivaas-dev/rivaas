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

package binding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindJSON_BasicTypes tests binding basic JSON data
func TestBindJSON_BasicTypes(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	body := []byte(`{"name":"John","email":"john@example.com","age":30}`)

	var user User
	err := JSONTo(body, &user)

	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

// TestBindJSON_NestedStructs tests binding nested JSON structures
func TestBindJSON_NestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type User struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	body := []byte(`{
		"name":"Alice",
		"address":{"street":"123 Main St","city":"NYC"}
	}`)

	var user User
	err := JSONTo(body, &user)

	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "123 Main St", user.Address.Street)
	assert.Equal(t, "NYC", user.Address.City)
}

// TestBindJSON_Arrays tests binding JSON arrays
func TestBindJSON_Arrays(t *testing.T) {
	t.Parallel()

	type Data struct {
		Tags []string `json:"tags"`
		IDs  []int    `json:"ids"`
	}

	body := []byte(`{"tags":["go","rust","python"],"ids":[1,2,3]}`)

	var data Data
	err := JSONTo(body, &data)

	require.NoError(t, err)
	assert.Equal(t, []string{"go", "rust", "python"}, data.Tags)
	assert.Equal(t, []int{1, 2, 3}, data.IDs)
}

// TestBindJSON_ErrorCases tests JSON binding error scenarios
func TestBindJSON_ErrorCases(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		body    []byte
		wantErr bool
	}{
		{
			name:    "malformed JSON",
			body:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    []byte(``),
			wantErr: true,
		},
		{
			name:    "type mismatch",
			body:    []byte(`{"name":"John","age":"not-a-number"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var user User
			err := JSONTo(tt.body, &user)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBindJSONStrict_UnknownFields tests strict JSON binding
func TestBindJSONStrict_UnknownFields(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name    string
		body    []byte
		wantErr bool
	}{
		{
			name:    "known fields only",
			body:    []byte(`{"name":"John","email":"john@example.com"}`),
			wantErr: false,
		},
		{
			name:    "unknown field present",
			body:    []byte(`{"name":"John","email":"john@example.com","unknown":"field"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var user User
			err := JSONTo(tt.body, &user, WithUnknownFields(UnknownError))

			if tt.wantErr {
				require.Error(t, err)
				var unknownErr *UnknownFieldError
				assert.ErrorAs(t, err, &unknownErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBindJSONInto_Generic tests generic JSON binding helper
func TestBindJSONInto_Generic(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := []byte(`{"name":"Jane","age":25}`)

	user, err := JSON[User](body)

	require.NoError(t, err)
	assert.Equal(t, "Jane", user.Name)
	assert.Equal(t, 25, user.Age)
}
