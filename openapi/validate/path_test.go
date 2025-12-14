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

package validate_test

import (
	"errors"
	"testing"

	"rivaas.dev/openapi/validate"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		// Valid paths - no parameters
		{
			name:    "simple path",
			path:    "/users",
			wantErr: nil,
		},
		{
			name:    "nested path",
			path:    "/api/v1/users",
			wantErr: nil,
		},
		{
			name:    "root path",
			path:    "/",
			wantErr: nil,
		},

		// Valid paths - router-style :param syntax
		{
			name:    "single colon parameter",
			path:    "/users/:id",
			wantErr: nil,
		},
		{
			name:    "multiple colon parameters",
			path:    "/users/:userId/posts/:postId",
			wantErr: nil,
		},
		{
			name:    "colon parameter with underscore",
			path:    "/users/:user_id",
			wantErr: nil,
		},
		{
			name:    "colon parameter with dash",
			path:    "/users/:user-id",
			wantErr: nil,
		},
		{
			name:    "colon parameter with dot",
			path:    "/api/:version.format",
			wantErr: nil,
		},

		// Valid paths - OpenAPI-style {param} syntax
		{
			name:    "single brace parameter",
			path:    "/users/{id}",
			wantErr: nil,
		},
		{
			name:    "multiple brace parameters",
			path:    "/users/{userId}/posts/{postId}",
			wantErr: nil,
		},
		{
			name:    "brace parameter with underscore",
			path:    "/users/{user_id}",
			wantErr: nil,
		},
		{
			name:    "brace parameter with dash",
			path:    "/users/{user-id}",
			wantErr: nil,
		},
		{
			name:    "brace parameter with dot",
			path:    "/api/{version.format}",
			wantErr: nil,
		},

		// Valid paths - mixed segments (not mixed syntax in same parameter)
		{
			name:    "mixed parameter styles in different segments",
			path:    "/users/:userId/posts/{postId}",
			wantErr: nil,
		},

		// Invalid paths - empty and missing slash
		{
			name:    "empty path",
			path:    "",
			wantErr: validate.ErrPathEmpty,
		},
		{
			name:    "no leading slash",
			path:    "users",
			wantErr: validate.ErrPathNoLeadingSlash,
		},
		{
			name:    "no leading slash with parameter",
			path:    "users/:id",
			wantErr: validate.ErrPathNoLeadingSlash,
		},

		// Invalid paths - duplicate parameters
		{
			name:    "duplicate colon parameters",
			path:    "/users/:id/posts/:id",
			wantErr: validate.ErrPathDuplicateParameter,
		},
		{
			name:    "duplicate brace parameters",
			path:    "/users/{id}/posts/{id}",
			wantErr: validate.ErrPathDuplicateParameter,
		},
		{
			name:    "duplicate mixed syntax parameters",
			path:    "/users/:id/posts/{id}",
			wantErr: validate.ErrPathDuplicateParameter,
		},

		// Invalid paths - malformed colon parameters
		{
			name:    "empty colon parameter",
			path:    "/users/:/posts",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "colon parameter with space",
			path:    "/users/:user id",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "colon parameter with special chars",
			path:    "/users/:user@id",
			wantErr: validate.ErrPathInvalidParameter,
		},

		// Invalid paths - malformed brace parameters
		{
			name:    "empty brace parameter",
			path:    "/users/{}/posts",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "unclosed brace",
			path:    "/users/{id",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "unopened brace",
			path:    "/users/id}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "brace parameter with space",
			path:    "/users/{user id}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "brace parameter with special chars",
			path:    "/users/{user@id}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "brace parameter with slash",
			path:    "/users/{user/id}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "nested braces",
			path:    "/users/{{id}}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "brace in middle of segment",
			path:    "/users/prefix{id}",
			wantErr: validate.ErrPathInvalidParameter,
		},
		{
			name:    "brace at end of segment",
			path:    "/users/id{suffix}",
			wantErr: validate.ErrPathInvalidParameter,
		},

		// Edge cases
		{
			name:    "trailing slash",
			path:    "/users/",
			wantErr: nil,
		},
		{
			name:    "multiple slashes",
			path:    "/users//posts",
			wantErr: nil, // Empty segments are skipped
		},
		{
			name:    "very long path",
			path:    "/api/v1/organizations/:orgId/projects/:projectId/deployments/:deployId/logs",
			wantErr: nil,
		},
		{
			name:    "numeric parameter name",
			path:    "/users/{123}",
			wantErr: nil,
		},
		{
			name:    "mixed alphanumeric parameter",
			path:    "/users/{user123}",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.ValidatePath(tt.path)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q) unexpected error: %v", tt.path, err)
				}
				return
			}

			if err == nil {
				t.Errorf("ValidatePath(%q) expected error %v, got nil", tt.path, tt.wantErr)
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePath(%q) error = %v, want %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath_ErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantContain string
	}{
		{
			name:        "duplicate parameter error message",
			path:        "/users/:id/posts/:id",
			wantContain: "id",
		},
		{
			name:        "invalid parameter name error message",
			path:        "/users/{user@id}",
			wantContain: "must match pattern",
		},
		{
			name:        "mismatched braces error message",
			path:        "/users/{id",
			wantContain: "mismatched braces",
		},
		{
			name:        "parameter with slash error message",
			path:        "/users/{user/id}",
			wantContain: "mismatched braces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.ValidatePath(tt.path)
			if err == nil {
				t.Errorf("ValidatePath(%q) expected error, got nil", tt.path)
				return
			}

			errMsg := err.Error()
			if !containsString(errMsg, tt.wantContain) {
				t.Errorf("ValidatePath(%q) error message %q does not contain %q", tt.path, errMsg, tt.wantContain)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexString(s, substr) >= 0))
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Benchmark path validation
func BenchmarkValidatePath(b *testing.B) {
	paths := []string{
		"/users",
		"/users/:id",
		"/users/{id}",
		"/users/:userId/posts/:postId",
		"/users/{userId}/posts/{postId}",
		"/api/v1/organizations/:orgId/projects/:projectId/deployments/:deployId/logs",
	}

	for _, path := range paths {
		b.Run(path, func(b *testing.B) {
			for b.Loop() {
				//nolint:errcheck // Benchmark focuses on performance, not error handling
				_ = validate.ValidatePath(path)
			}
		})
	}
}
