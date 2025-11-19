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
)

func TestRFC9457_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		formatter  *RFC9457
		err        error
		wantStatus int
		wantType   string
	}{
		{
			name:       "simple error",
			formatter:  NewRFC9457("https://api.example.com/problems"),
			err:        &testError{message: "something went wrong"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "about:blank",
		},
		{
			name:       "error with code",
			formatter:  NewRFC9457("https://api.example.com/problems"),
			err:        &testErrorWithCode{message: "validation failed", code: "validation_error"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "https://api.example.com/problems/validation_error",
		},
		{
			name:       "error with status",
			formatter:  NewRFC9457("https://api.example.com/problems"),
			err:        &testErrorWithStatus{message: "not found", status: http.StatusNotFound},
			wantStatus: http.StatusNotFound,
			wantType:   "about:blank",
		},
		{
			name:       "error with code and status",
			formatter:  NewRFC9457("https://api.example.com/problems"),
			err:        &testErrorFull{message: "bad request", code: "invalid_input", status: http.StatusBadRequest},
			wantStatus: http.StatusBadRequest,
			wantType:   "https://api.example.com/problems/invalid_input",
		},
		{
			name: "custom type resolver",
			formatter: &RFC9457{
				BaseURL: "https://api.example.com/problems",
				TypeResolver: func(err error) string {
					return "https://api.example.com/problems/custom-type"
				},
			},
			err:        &testError{message: "test"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "https://api.example.com/problems/custom-type",
		},
		{
			name: "custom status resolver",
			formatter: &RFC9457{
				BaseURL: "https://api.example.com/problems",
				StatusResolver: func(err error) int {
					return http.StatusTeapot
				},
			},
			err:        &testError{message: "test"},
			wantStatus: http.StatusTeapot,
			wantType:   "about:blank",
		},
		{
			name:       "no base URL",
			formatter:  NewRFC9457(""),
			err:        &testErrorWithCode{message: "test", code: "test_code"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "test_code",
		},
		{
			name: "disabled error ID",
			formatter: &RFC9457{
				BaseURL:        "https://api.example.com/problems",
				DisableErrorID: true,
			},
			err:        &testError{message: "test"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "about:blank",
		},
		{
			name: "custom error ID generator",
			formatter: &RFC9457{
				BaseURL: "https://api.example.com/problems",
				ErrorIDGenerator: func() string {
					return "custom-id-123"
				},
			},
			err:        &testError{message: "test"},
			wantStatus: http.StatusInternalServerError,
			wantType:   "about:blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			response := tt.formatter.Format(req, tt.err)

			if response.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", response.Status, tt.wantStatus)
			}

			if response.ContentType != "application/problem+json; charset=utf-8" {
				t.Errorf("ContentType = %q, want %q", response.ContentType, "application/problem+json; charset=utf-8")
			}

			body, ok := response.Body.(ProblemDetail)
			if !ok {
				t.Fatalf("Body is not ProblemDetail, got %T", response.Body)
			}

			if body.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", body.Type, tt.wantType)
			}

			if body.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", body.Status, tt.wantStatus)
			}

			if body.Detail != tt.err.Error() {
				t.Errorf("Detail = %q, want %q", body.Detail, tt.err.Error())
			}

			// Check error_id unless disabled
			if !tt.formatter.DisableErrorID {
				if _, ok := body.Extensions["error_id"]; !ok {
					t.Error("error_id not found in extensions")
				}
			} else {
				if _, ok := body.Extensions["error_id"]; ok {
					t.Error("error_id should not be present when disabled")
				}
			}

			// Check custom error ID generator
			if tt.formatter.ErrorIDGenerator != nil {
				if id, ok := body.Extensions["error_id"].(string); ok {
					if id != "custom-id-123" {
						t.Errorf("error_id = %q, want %q", id, "custom-id-123")
					}
				}
			}
		})
	}
}

func TestRFC9457_Format_WithDetails(t *testing.T) {
	t.Parallel()

	formatter := NewRFC9457("https://api.example.com/problems")
	err := &testErrorWithDetails{
		message: "validation failed",
		details: map[string]any{
			"field1": "error1",
			"field2": "error2",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := formatter.Format(req, err)

	body, ok := response.Body.(ProblemDetail)
	if !ok {
		t.Fatalf("Body is not ProblemDetail, got %T", response.Body)
	}

	if errors, ok := body.Extensions["errors"].(map[string]any); ok {
		if errors["field1"] != "error1" {
			t.Errorf("errors[field1] = %v, want %q", errors["field1"], "error1")
		}
	} else {
		t.Error("errors not found in extensions or wrong type")
	}
}

func TestRFC9457_MarshalJSON(t *testing.T) {
	t.Parallel()

	p := ProblemDetail{
		Type:     "https://api.example.com/problems/validation_error",
		Title:    "Bad Request",
		Status:   400,
		Detail:   "Validation failed",
		Instance: "/api/users",
		Extensions: map[string]any{
			"error_id": "err-123",
			"code":     "validation_error",
			"errors":   []string{"field1 is required"},
		},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["type"] != p.Type {
		t.Errorf("type = %v, want %q", result["type"], p.Type)
	}

	if result["error_id"] != "err-123" {
		t.Errorf("error_id = %v, want %q", result["error_id"], "err-123")
	}

	// Reserved fields should not be overwritten
	if result["type"] == "overwritten" {
		t.Error("reserved field 'type' was overwritten")
	}
}
