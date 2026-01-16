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

package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONAPI_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		formatter  *JSONAPI
		err        error
		wantStatus int
	}{
		{
			name:       "simple error",
			formatter:  NewJSONAPI(),
			err:        &testError{message: "something went wrong"},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "error with code",
			formatter:  NewJSONAPI(),
			err:        &testErrorWithCode{message: "validation failed", code: "validation_error"},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "error with status",
			formatter:  NewJSONAPI(),
			err:        &testErrorWithStatus{message: "not found", status: http.StatusNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "custom status resolver",
			formatter: &JSONAPI{
				StatusResolver: func(err error) int {
					return http.StatusTeapot
				},
			},
			err:        &testError{message: "test"},
			wantStatus: http.StatusTeapot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			response := tt.formatter.Format(req, tt.err)

			assert.Equal(t, tt.wantStatus, response.Status, "Status")
			assert.Equal(t, "application/vnd.api+json; charset=utf-8", response.ContentType, "ContentType")

			body, ok := response.Body.(jsonAPIErrorResponse)
			require.True(t, ok, "Body is not jsonAPIErrorResponse, got %T", response.Body)

			assert.NotEmpty(t, body.Errors, "Errors slice is empty")

			firstErr := body.Errors[0]
			assert.Equal(t, tt.err.Error(), firstErr.Detail, "Detail")
			assert.Equal(t, strconv.Itoa(tt.wantStatus), firstErr.Status, "Status")
		})
	}
}

func TestJSONAPI_Format_WithDetails(t *testing.T) {
	t.Parallel()

	formatter := NewJSONAPI()
	err := &testErrorWithDetailsSlice{
		message: "validation failed",
		details: []map[string]any{
			{
				"path":    "email",
				"code":    "required",
				"message": "email is required",
			},
			{
				"path":    "items.0.price",
				"code":    "min",
				"message": "price must be positive",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := formatter.Format(req, err)

	body, ok := response.Body.(jsonAPIErrorResponse)
	require.True(t, ok, "Body is not jsonAPIErrorResponse, got %T", response.Body)

	require.Len(t, body.Errors, 2, "Errors length")

	// Check first error
	firstErr := body.Errors[0]
	require.NotNil(t, firstErr.Source, "Source is nil for first error")
	assert.Equal(t, "/data/attributes/email", firstErr.Source.Pointer, "Source.Pointer")
	assert.Equal(t, "required", firstErr.Code, "Code")

	// Check second error
	secondErr := body.Errors[1]
	require.NotNil(t, secondErr.Source, "Source is nil for second error")
	assert.Equal(t, "/data/attributes/items/0/price", secondErr.Source.Pointer, "Source.Pointer")
}

func TestConvertPathToPointer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple field",
			path: "email",
			want: "/data/attributes/email",
		},
		{
			name: "nested field",
			path: "user.name",
			want: "/data/attributes/user/name",
		},
		{
			name: "array index",
			path: "items.0.price",
			want: "/data/attributes/items/0/price",
		},
		{
			name: "deeply nested",
			path: "user.address.city.zip",
			want: "/data/attributes/user/address/city/zip",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := convertPathToPointer(tt.path)
			assert.Equal(t, tt.want, got, "convertPathToPointer(%q)", tt.path)
		})
	}
}

func TestJSONAPI_MarshalJSON(t *testing.T) {
	t.Parallel()

	response := jsonAPIErrorResponse{
		Errors: []jsonAPIError{
			{
				ID:     "err-123",
				Status: "400",
				Code:   "validation_error",
				Title:  "Bad Request",
				Detail: "Validation failed",
				Source: &jsonAPISource{
					Pointer: "/data/attributes/email",
				},
			},
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err, "MarshalJSON failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result), "Unmarshal failed")

	errors, ok := result["errors"].([]any)
	require.True(t, ok, "errors is not []interface{}, got %T", result["errors"])

	assert.Len(t, errors, 1, "errors length")
}
