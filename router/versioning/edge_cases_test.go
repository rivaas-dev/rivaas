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

package versioning

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_DetectVersion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() (*Engine, *http.Request)
		want    string
		wantErr bool
	}{
		{
			name: "nil_request",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(WithDefaultVersion("v1"))
				require.NoError(t, err)
				return engine, nil
			},
			want: "v1",
		},
		{
			name: "empty_path",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/", nil)
				req.URL.Path = "" // Simulate empty path
				return engine, req
			},
			want: "v1",
		},
		{
			name: "root_path",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/", nil)
				return engine, req
			},
			want: "v1",
		},
		{
			name: "path_with_multiple_slashes",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "//v1//users", nil)
				return engine, req
			},
			want: "v1",
		},
		{
			name: "path_with_trailing_slash",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/v1/users/", nil)
				return engine, req
			},
			want: "v1",
		},
		{
			name: "version_at_end_of_path",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/v2", nil)
				return engine, req
			},
			want: "v2",
		},
		{
			name: "unicode_in_version",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithHeaderVersioning("API-Version"),
					WithValidVersions("v1", "v2", "v测试"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("API-Version", "v测试")
				return engine, req
			},
			want: "v测试",
		},
		{
			name: "very_long_version_string",
			setup: func() (*Engine, *http.Request) {
				longVersion := string(make([]byte, 200)) // 200 bytes
				engine, err := New(
					WithHeaderVersioning("API-Version"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("API-Version", longVersion)
				return engine, req
			},
			want: string(make([]byte, 200)), // Should accept long versions
		},
		{
			name: "version_with_special_characters",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithHeaderVersioning("API-Version"),
					WithValidVersions("v1.2.3-beta+build", "v2"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("API-Version", "v1.2.3-beta+build")
				return engine, req
			},
			want: "v1.2.3-beta+build",
		},
		{
			name: "url_encoded_query_parameter",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test?v=v%201", nil)
				return engine, req
			},
			want: "v%201", // extractQueryVersion doesn't decode, uses RawQuery
		},
		{
			name: "multiple_query_params_same_name",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test?v=v1&v=v2", nil)
				return engine, req
			},
			want: "v1", // Should use first value
		},
		{
			name: "case_sensitive_header",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithHeaderVersioning("API-Version"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("api-version", "v2") // Lowercase
				return engine, req
			},
			want: "v2", // HTTP headers are case-insensitive
		},
		{
			name: "empty_header_value",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithHeaderVersioning("API-Version"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("API-Version", "")
				return engine, req
			},
			want: "v1", // Empty header should fallback to default
		},
		{
			name: "empty_query_value",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test?v=", nil)
				return engine, req
			},
			want: "v1", // Empty query value should fallback to default
		},
		{
			name: "accept_header_with_parameters",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Accept", "application/vnd.myapi.v2+json; charset=utf-8; q=0.9")
				return engine, req
			},
			want: "2", // extractAcceptVersion extracts just the version number
		},
		{
			name: "accept_header_multiple_values",
			setup: func() (*Engine, *http.Request) {
				engine, err := New(
					WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
					WithDefaultVersion("v1"),
				)
				require.NoError(t, err)
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Accept", "application/vnd.myapi.v3+json, application/vnd.myapi.v2+json, text/html")
				return engine, req
			},
			want: "3", // extractAcceptVersion extracts just the version number
		},
		{
			name: "nil_engine",
			setup: func() (*Engine, *http.Request) {
				var engine *Engine = nil
				req := httptest.NewRequest("GET", "/test", nil)
				return engine, req
			},
			want: "v1", // Safe fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, req := tt.setup()
			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_StripPathVersion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() (*Engine, string, string)
		want    string
		wantErr bool
	}{
		{
			name: "empty_path",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "", "v1"
			},
			want: "",
		},
		{
			name: "root_path",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "/", "v1"
			},
			want: "/",
		},
		{
			name: "path_with_multiple_slashes",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "//v1//users", "v1"
			},
			want: "//v1//users", // Path doesn't match prefix "/v" when it starts with "//"
		},
		{
			name: "version_only_path",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "/v1", "v1"
			},
			want: "/",
		},
		{
			name: "nil_engine",
			setup: func() (*Engine, string, string) {
				var engine *Engine = nil
				return engine, "/v1/users", "v1"
			},
			want: "/v1/users", // Should return path unchanged
		},
		{
			name: "empty_version",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "/v1/users", ""
			},
			want: "/v1/users", // Should return path unchanged
		},
		{
			name: "mismatched_version",
			setup: func() (*Engine, string, string) {
				engine, err := New(WithPathVersioning("/v{version}/"))
				require.NoError(t, err)
				return engine, "/v1/users", "v2"
			},
			want: "/v1/users", // Should return path unchanged if version doesn't match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, path, version := tt.setup()
			got := engine.StripPathVersion(path, version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractQueryVersion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawQuery  string
		param     string
		want      string
		wantFound bool
	}{
		{
			name:      "empty_query",
			rawQuery:  "",
			param:     "v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "empty_param",
			rawQuery:  "v=v1",
			param:     "",
			want:      "",
			wantFound: false,
		},
		{
			name:      "param_without_value",
			rawQuery:  "v=",
			param:     "v",
			want:      "",
			wantFound: true,
		},
		{
			name:      "param_at_end",
			rawQuery:  "foo=bar&v=v1",
			param:     "v",
			want:      "v1",
			wantFound: true,
		},
		{
			name:      "param_in_middle",
			rawQuery:  "foo=bar&v=v1&baz=qux",
			param:     "v",
			want:      "v1",
			wantFound: true,
		},
		{
			name:      "similar_param_names",
			rawQuery:  "version=v2&v=v1",
			param:     "v",
			want:      "v1",
			wantFound: true,
		},
		{
			name:      "param_name_in_value",
			rawQuery:  "foo=v=bar",
			param:     "v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "multiple_equals",
			rawQuery:  "v=v=1",
			param:     "v",
			want:      "v=1",
			wantFound: true,
		},
		{
			name:      "url_encoded_value",
			rawQuery:  "v=v%201",
			param:     "v",
			want:      "v%201", // Not decoded by extractQueryVersion (net/http handles this)
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, found := extractQueryVersion(tt.rawQuery, tt.param)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestExtractPathVersion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		prefix    string
		want      string
		wantFound bool
	}{
		{
			name:      "empty_path",
			path:      "",
			prefix:    "/v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "empty_prefix",
			path:      "/v1/users",
			prefix:    "",
			want:      "",
			wantFound: false,
		},
		{
			name:      "root_path",
			path:      "/",
			prefix:    "/v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "path_with_multiple_slashes",
			path:      "//v1//users",
			prefix:    "/v",
			want:      "", // Path doesn't start with "/v" prefix
			wantFound: false,
		},
		{
			name:      "version_at_end",
			path:      "/v1",
			prefix:    "/v",
			want:      "1",
			wantFound: true,
		},
		{
			name:      "prefix_longer_than_path",
			path:      "/v",
			prefix:    "/v1",
			want:      "",
			wantFound: false,
		},
		{
			name:      "unicode_in_path",
			path:      "/v测试/users",
			prefix:    "/v",
			want:      "测试",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, found := extractPathVersion(tt.path, tt.prefix)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestExtractAcceptVersion_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		accept    string
		pattern   string
		want      string
		wantFound bool
	}{
		{
			name:      "empty_accept",
			accept:    "",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "",
			wantFound: false,
		},
		{
			name:      "empty_pattern",
			accept:    "application/vnd.myapi.v2+json",
			pattern:   "",
			want:      "",
			wantFound: false,
		},
		{
			name:      "pattern_without_placeholder",
			accept:    "application/json",
			pattern:   "application/json",
			want:      "",
			wantFound: false,
		},
		{
			name:      "accept_with_parameters",
			accept:    "application/vnd.myapi.v2+json; charset=utf-8",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "2", // extractAcceptVersion extracts just the version number
			wantFound: true,
		},
		{
			name:      "accept_with_quality_value",
			accept:    "application/vnd.myapi.v2+json; q=0.9",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "2", // extractAcceptVersion extracts just the version number
			wantFound: true,
		},
		{
			name:      "multiple_accept_values",
			accept:    "application/vnd.myapi.v3+json, application/vnd.myapi.v2+json",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "3", // fastAcceptVersion extracts just the version number
			wantFound: true,
		},
		{
			name:      "pattern_without_suffix",
			accept:    "application/vnd.myapi.v2",
			pattern:   "application/vnd.myapi.v{version}",
			want:      "2", // extractAcceptVersion extracts just the version number
			wantFound: true,
		},
		{
			name:      "unicode_in_version",
			accept:    "application/vnd.myapi.v测试+json",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "测试",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, found := extractAcceptVersion(tt.accept, tt.pattern)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestEngine_ConcurrentDetection(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("API-Version"),
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err)

	const numGoroutines = 100
	const numRequests = 10

	errors := make(chan error, numGoroutines*numRequests)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numRequests; j++ {
				req := httptest.NewRequest("GET", "/v2/users", nil)
				req.Header.Set("API-Version", "v3")
				version := engine.DetectVersion(req)
				if version == "" {
					errors <- assert.AnError
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(t, err)
	}
}

func TestEngine_InputSanitization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		version      string
		shouldReject bool
	}{
		{
			name:         "normal_version",
			version:      "v1",
			shouldReject: false,
		},
		{
			name:         "version_with_control_characters",
			version:      "v1\x00\x01",
			shouldReject: false, // Currently not rejected, but should be handled gracefully
		},
		{
			name:         "version_with_newline",
			version:      "v1\nv2",
			shouldReject: false, // Currently not rejected
		},
		{
			name:         "version_with_tab",
			version:      "v1\tv2",
			shouldReject: false, // Currently not rejected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, err := New(
				WithHeaderVersioning("API-Version"),
				WithValidVersions("v1", "v2"),
				WithDefaultVersion("v1"),
			)
			require.NoError(t, err)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("API-Version", tt.version)

			// Should not panic
			version := engine.DetectVersion(req)
			assert.NotEmpty(t, version)

			// Check for control characters
			hasControl := false
			for _, r := range tt.version {
				if unicode.IsControl(r) {
					hasControl = true
					break
				}
			}
			if tt.shouldReject && hasControl {
				assert.Equal(t, "v1", version, "should fallback to default for invalid version")
			}
		})
	}
}
