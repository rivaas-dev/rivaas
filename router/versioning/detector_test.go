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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractQueryVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawQuery  string
		param     string
		want      string
		wantFound bool
	}{
		{
			name:      "simple_query",
			rawQuery:  "v=v1",
			param:     "v",
			want:      "v1",
			wantFound: true,
		},
		{
			name:      "query_with_other_params",
			rawQuery:  "foo=bar&v=v2&baz=qux",
			param:     "v",
			want:      "v2",
			wantFound: true,
		},
		{
			name:      "long_param_name",
			rawQuery:  "version=v1",
			param:     "version",
			want:      "v1",
			wantFound: true,
		},
		{
			name:      "param_not_found",
			rawQuery:  "value=v1",
			param:     "v",
			want:      "",
			wantFound: false,
		},
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
			name:      "param_at_end_no_value",
			rawQuery:  "foo=bar&v=",
			param:     "v",
			want:      "",
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

func TestExtractHeaderVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headers    http.Header
		headerName string
		want       string
	}{
		{
			name: "header_present",
			headers: http.Header{
				"Api-Version": []string{"v2"},
			},
			headerName: "Api-Version",
			want:       "v2",
		},
		{
			name: "header_with_multiple_values",
			headers: http.Header{
				"Api-Version": []string{"v2", "v1"},
			},
			headerName: "Api-Version",
			want:       "v2", // Should return first value
		},
		{
			name:       "header_missing",
			headers:    http.Header{},
			headerName: "Api-Version",
			want:       "",
		},
		{
			name: "header_empty",
			headers: http.Header{
				"Api-Version": []string{""},
			},
			headerName: "Api-Version",
			want:       "",
		},
		{
			name: "different_header",
			headers: http.Header{
				"X-Custom": []string{"v1"},
			},
			headerName: "Api-Version",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractHeaderVersion(tt.headers, tt.headerName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractPathVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		prefix    string
		want      string
		wantFound bool
	}{
		{
			name:      "simple_version",
			path:      "/v1/users",
			prefix:    "/v",
			want:      "1",
			wantFound: true,
		},
		{
			name:      "version_with_path",
			path:      "/v2/posts/123",
			prefix:    "/v",
			want:      "2",
			wantFound: true,
		},
		{
			name:      "no_version",
			path:      "/users",
			prefix:    "/v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "wrong_prefix",
			path:      "/api/v1/users",
			prefix:    "/v",
			want:      "",
			wantFound: false,
		},
		{
			name:      "api_prefix",
			path:      "/api/v1/users",
			prefix:    "/api/v",
			want:      "1",
			wantFound: true,
		},
		{
			name:      "version_at_end",
			path:      "/v1",
			prefix:    "/v",
			want:      "1",
			wantFound: true,
		},
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

func TestExtractAcceptVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		accept    string
		pattern   string
		want      string
		wantFound bool
	}{
		{
			name:      "simple_accept",
			accept:    "application/vnd.myapi.v2+json",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "2",
			wantFound: true,
		},
		{
			name:      "accept_with_multiple_values",
			accept:    "application/vnd.myapi.v1+json, text/html",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "1",
			wantFound: true,
		},
		{
			name:      "no_matching_accept",
			accept:    "application/json",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "",
			wantFound: false,
		},
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
			name:      "accept_with_params",
			accept:    "application/vnd.myapi.v2+json; charset=utf-8",
			pattern:   "application/vnd.myapi.v{version}+json",
			want:      "2",
			wantFound: true,
		},
		{
			name:      "pattern_without_suffix",
			accept:    "application/vnd.myapi.v2",
			pattern:   "application/vnd.myapi.v{version}",
			want:      "2",
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
