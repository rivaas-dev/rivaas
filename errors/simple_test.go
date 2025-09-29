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

package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimple_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		formatter  *Simple
		err        error
		wantStatus int
	}{
		{
			name:       "simple error",
			formatter:  NewSimple(),
			err:        &testError{message: "something went wrong"},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "error with code",
			formatter:  NewSimple(),
			err:        &testErrorWithCode{message: "validation failed", code: "validation_error"},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "error with status",
			formatter:  NewSimple(),
			err:        &testErrorWithStatus{message: "not found", status: http.StatusNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "error with details",
			formatter:  NewSimple(),
			err:        &testErrorWithDetails{message: "validation failed", details: map[string]any{"field": "error"}},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "custom status resolver",
			formatter: &Simple{
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
			assert.Equal(t, "application/json; charset=utf-8", response.ContentType, "ContentType")

			body, ok := response.Body.(map[string]any)
			require.True(t, ok, "Body is not map[string]any, got %T", response.Body)

			assert.Equal(t, tt.err.Error(), body["error"], "error")

			// Check code if available
			if coded, ok := tt.err.(ErrorCode); ok {
				assert.Equal(t, coded.Code(), body["code"], "code")
			}

			// Check details if available
			if detailed, ok := tt.err.(ErrorDetails); ok {
				assert.NotNil(t, body["details"], "details not found in body")
				// Details should be present
				_ = detailed.Details()
			}
		})
	}
}

func TestSimple_MarshalJSON(t *testing.T) {
	t.Parallel()

	formatter := NewSimple()
	err := &testErrorFull{
		message: "bad request",
		code:    "invalid_input",
		status:  http.StatusBadRequest,
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := formatter.Format(req, err)

	data, marshalErr := json.Marshal(response.Body)
	require.NoError(t, marshalErr, "MarshalJSON failed")

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result), "Unmarshal failed")

	assert.Equal(t, "bad request", result["error"], "error")
	assert.Equal(t, "invalid_input", result["code"], "code")
}
