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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_DetectVersion_Header(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithHeaderVersioning("API-Version"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err, "failed to create engine")

	tests := []struct {
		name        string
		headerValue string
		want        string
	}{
		{"header_present", "v2", "v2"},
		{"header_missing", "", "v1"},
		{"header_empty", "", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set("API-Version", tt.headerValue)
			}

			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_DetectVersion_Query(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err, "failed to create engine")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"query_present", "/test?v=v2", "v2"},
		{"query_missing", "/test", "v1"},
		{"query_empty", "/test?v=", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.url, nil)
			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_DetectVersion_Path(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err, "failed to create engine")

	tests := []struct {
		name string
		path string
		want string
	}{
		{"path_v2", "/v2/users", "v2"},
		{"path_v1", "/v1/users", "v1"},
		{"path_no_version", "/users", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_DetectVersion_Validation(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithQueryVersioning("v"),
		WithValidVersions("v1", "v2", "v3"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err, "failed to create engine")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"valid_v2", "/test?v=v2", "v2"},
		{"invalid_v99", "/test?v=v99", "v1"}, // Should fallback to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.url, nil)
			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_StripPathVersion(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithPathVersioning("/v{version}/"),
	)
	require.NoError(t, err, "failed to create engine")

	tests := []struct {
		name    string
		path    string
		version string
		want    string
	}{
		{"strip_v1", "/v1/users", "v1", "/users"},
		{"strip_v2", "/v2/posts/123", "v2", "/posts/123"},
		{"no_version", "/users", "", "/users"},
		{"version_mismatch", "/v1/users", "v2", "/v1/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := engine.StripPathVersion(tt.path, tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_Config(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v2"),
	)
	require.NoError(t, err, "failed to create engine")

	cfg := engine.Config()
	assert.Equal(t, "X-API-Version", cfg.HeaderName)
	assert.Equal(t, "v2", cfg.DefaultVersion)
	assert.True(t, cfg.HeaderEnabled)
}

// TestEngine_AcceptVersioning tests Accept header-based versioning
func TestEngine_AcceptVersioning(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		accept string
		want   string
	}{
		{
			name:   "valid_accept_v2",
			accept: "application/vnd.myapi.v2+json",
			want:   "2", // Note: Accept pattern extracts just the number
		},
		{
			name:   "valid_accept_v1",
			accept: "application/vnd.myapi.v1+json",
			want:   "1",
		},
		{
			name:   "no_accept_header",
			accept: "",
			want:   "v1",
		},
		{
			name:   "invalid_accept",
			accept: "application/json",
			want:   "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}

			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_CustomDetector tests custom version detection
func TestEngine_CustomDetector(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithCustomVersionDetector(func(r *http.Request) string {
			// Custom logic: extract version from X-Custom-Version header
			return r.Header.Get("X-Custom-Version")
		}),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "custom_v2",
			header: "v2",
			want:   "v2",
		},
		{
			name:   "custom_v1",
			header: "v1",
			want:   "v1",
		},
		{
			name:   "no_header",
			header: "",
			want:   "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("X-Custom-Version", tt.header)
			}

			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_SetLifecycleHeaders tests comprehensive lifecycle header management
func TestEngine_SetLifecycleHeaders(t *testing.T) {
	t.Parallel()

	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	docURL := "https://docs.example.com/migration/v1-to-v2"

	tests := []struct {
		name               string
		setupEngine        func() *Engine
		version            string
		route              string
		currentTime        time.Time
		wantSunset         bool
		wantVersionHeader  bool
		wantDeprecation    bool
		wantSunsetHeader   bool
		wantWarning299     bool
		wantLinkHeader     bool
		wantDeprecatedCall bool
	}{
		{
			name: "deprecated_version_with_all_features",
			setupEngine: func() *Engine {
				engine, _ := New(
					WithDeprecatedVersion("v1", sunsetDate),
					WithVersionHeader(),
					WithWarning299(),
					WithDeprecationLink("v1", docURL),
					WithDeprecatedUseCallback(func(version, route string) {
						// Callback will be tested in separate test
					}),
					WithClock(func() time.Time {
						return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
					}),
				)
				return engine
			},
			version:            "v1",
			route:              "/api/users",
			currentTime:        time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			wantSunset:         false,
			wantVersionHeader:  true,
			wantDeprecation:    true,
			wantSunsetHeader:   true,
			wantWarning299:     true,
			wantLinkHeader:     true,
			wantDeprecatedCall: true,
		},
		{
			name: "sunset_version_with_enforcement",
			setupEngine: func() *Engine {
				engine, _ := New(
					WithDeprecatedVersion("v1", sunsetDate),
					WithVersionHeader(),
					WithSunsetEnforcement(),
					WithDeprecationLink("v1", docURL),
					WithClock(func() time.Time {
						return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
					}),
				)
				return engine
			},
			version:           "v1",
			route:             "/api/users",
			currentTime:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantSunset:        true,
			wantVersionHeader: true, // Still send version header to inform client
			wantDeprecation:   false,
			wantSunsetHeader:  true,
			wantWarning299:    false,
			wantLinkHeader:    true,
		},
		{
			name: "non_deprecated_version",
			setupEngine: func() *Engine {
				engine, _ := New(
					WithDeprecatedVersion("v1", sunsetDate),
					WithVersionHeader(),
					WithWarning299(),
				)
				return engine
			},
			version:           "v2",
			route:             "/api/users",
			wantSunset:        false,
			wantVersionHeader: true,
			wantDeprecation:   false,
			wantSunsetHeader:  false,
			wantWarning299:    false,
			wantLinkHeader:    false,
		},
		{
			name: "minimal_configuration",
			setupEngine: func() *Engine {
				engine, _ := New(
					WithDeprecatedVersion("v1", sunsetDate),
				)
				return engine
			},
			version:           "v1",
			route:             "/api/users",
			wantSunset:        false,
			wantVersionHeader: false,
			wantDeprecation:   true,
			wantSunsetHeader:  true,
			wantWarning299:    false,
			wantLinkHeader:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setupEngine()
			w := httptest.NewRecorder()

			isSunset := engine.SetLifecycleHeaders(w, tt.version, tt.route)

			// Check if sunset enforcement returned correct value
			assert.Equal(t, tt.wantSunset, isSunset)

			// Check version header
			if tt.wantVersionHeader {
				assert.Equal(t, tt.version, w.Header().Get("X-API-Version"))
			} else {
				assert.Empty(t, w.Header().Get("X-API-Version"))
			}

			// Check deprecation header
			if tt.wantDeprecation {
				assert.Equal(t, "true", w.Header().Get("Deprecation"))
			} else {
				assert.Empty(t, w.Header().Get("Deprecation"))
			}

			// Check sunset header
			if tt.wantSunsetHeader {
				assert.NotEmpty(t, w.Header().Get("Sunset"))
			} else {
				assert.Empty(t, w.Header().Get("Sunset"))
			}

			// Check Warning: 299 header
			if tt.wantWarning299 {
				warning := w.Header().Get("Warning")
				assert.Contains(t, warning, "299")
				assert.Contains(t, warning, tt.version)
				assert.Contains(t, warning, "deprecated")
			} else {
				assert.Empty(t, w.Header().Get("Warning"))
			}

			// Check Link header
			if tt.wantLinkHeader {
				linkHeader := w.Header().Get("Link")
				assert.NotEmpty(t, linkHeader)
				if !tt.wantSunset {
					assert.Contains(t, linkHeader, "rel=\"deprecation\"")
				}
			} else {
				assert.Empty(t, w.Header().Get("Link"))
			}
		})
	}
}

// TestEngine_SetLifecycleHeaders_CallbackAsync tests that deprecated callback is async
func TestEngine_SetLifecycleHeaders_CallbackAsync(t *testing.T) {
	t.Parallel()

	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	callbackCalled := false
	var callbackVersion, callbackRoute string

	engine, err := New(
		WithDeprecatedVersion("v1", sunsetDate),
		WithDeprecatedUseCallback(func(version, route string) {
			callbackCalled = true
			callbackVersion = version
			callbackRoute = route
		}),
		WithClock(func() time.Time {
			return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
		}),
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	engine.SetLifecycleHeaders(w, "v1", "/api/users")

	// Wait briefly for async callback
	time.Sleep(10 * time.Millisecond)

	assert.True(t, callbackCalled)
	assert.Equal(t, "v1", callbackVersion)
	assert.Equal(t, "/api/users", callbackRoute)
}

// TestEngine_ShouldApplyVersioning tests versioning application logic
func TestEngine_ShouldApplyVersioning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		engine *Engine
		path   string
		want   bool
	}{
		{
			name: "header_based_always_applies",
			engine: func() *Engine {
				e, _ := New(WithHeaderVersioning("API-Version"))
				return e
			}(),
			path: "/users",
			want: true,
		},
		{
			name: "path_based_with_version",
			engine: func() *Engine {
				e, _ := New(WithPathVersioning("/v{version}/"))
				return e
			}(),
			path: "/v1/users",
			want: true,
		},
		{
			name: "path_based_no_version_with_default",
			engine: func() *Engine {
				e, _ := New(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				)
				return e
			}(),
			path: "/users",
			want: true,
		},
		{
			name: "path_based_no_version_no_default",
			engine: func() *Engine {
				e, _ := New(WithPathVersioning("/v{version}/"))
				// Explicitly set DefaultVersion to empty after initialization
				e.config.DefaultVersion = ""
				return e
			}(),
			path: "/users",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.engine.ShouldApplyVersioning(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_ExtractPathSegment tests path segment extraction
func TestEngine_ExtractPathSegment(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithPathVersioning("/v{version}/"),
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		path   string
		want   string
		wantOk bool
	}{
		{
			name:   "extract_v1",
			path:   "/v1/users",
			want:   "v1",
			wantOk: true,
		},
		{
			name:   "extract_v2",
			path:   "/v2/posts/123",
			want:   "v2",
			wantOk: true,
		},
		{
			name:   "no_version",
			path:   "/users",
			want:   "",
			wantOk: false,
		},
		{
			name:   "invalid_version",
			path:   "/v99/users",
			want:   "v99",
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := engine.ExtractPathSegment(tt.path)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

// TestEngine_DetectionPriority tests version detection priority order
func TestEngine_DetectionPriority(t *testing.T) {
	t.Parallel()

	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("API-Version"),
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		path   string
		header string
		query  string
		want   string
	}{
		{
			name:   "path_has_priority",
			path:   "/v2/users",
			header: "v3",
			query:  "?v=v4",
			want:   "v2",
		},
		{
			name:   "header_when_no_path",
			path:   "/users",
			header: "v3",
			query:  "?v=v4",
			want:   "v3",
		},
		{
			name:   "query_when_no_path_or_header",
			path:   "/users?v=v4",
			header: "",
			query:  "?v=v4",
			want:   "v4",
		},
		{
			name:   "default_when_none",
			path:   "/users",
			header: "",
			query:  "",
			want:   "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.header != "" {
				req.Header.Set("API-Version", tt.header)
			}

			got := engine.DetectVersion(req)
			assert.Equal(t, tt.want, got)
		})
	}
}
