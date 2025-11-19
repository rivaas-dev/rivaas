package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFastQueryVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawQuery  string
		param     string
		wantValue string
		wantFound bool
	}{
		// Basic cases
		{"empty_query", "", "v", "", false},
		{"empty_param", "v=v1", "", "", false},
		{"simple_match", "v=v1", "v", "v1", true},
		{"long_value", "version=v1.2.3", "version", "v1.2.3", true},

		// Position variations
		{"param_at_start", "v=v1", "v", "v1", true},
		{"param_in_middle", "foo=bar&v=v2&baz=qux", "v", "v2", true},
		{"param_at_end", "foo=bar&baz=qux&v=v3", "v", "v3", true},

		// Boundary cases
		{"param_not_at_boundary", "foobar=v1", "bar", "", false}, // "bar=" is not at boundary
		{"similar_param_names", "fooversion=v1&version=v2", "version", "v2", true},
		{"param_as_substring", "myv=wrong&v=correct", "v", "correct", true},

		// Value extraction
		{"value_until_ampersand", "v=v1&other=value", "v", "v1", true},
		{"value_until_end", "other=value&v=v2", "v", "v2", true},
		{"empty_value", "v=&other=value", "v", "", true}, // Empty value is valid
		{"no_value", "v", "v", "", false},                // No "=" sign

		// Multiple occurrences (should return first)
		{"duplicate_params", "v=v1&foo=bar&v=v2", "v", "v1", true},

		// Special characters in values
		{"dash_in_value", "v=v1-beta", "v", "v1-beta", true},
		{"dot_in_value", "v=1.0.0", "v", "1.0.0", true},
		{"underscore_in_value", "v=v1_stable", "v", "v1_stable", true},

		// Long parameter names
		{"long_param_name", "api_version=v1", "api_version", "v1", true},
		{"long_param_middle", "foo=bar&api_version=v2&baz=qux", "api_version", "v2", true},

		// Edge cases
		{"single_char_param", "a=1", "a", "1", true},
		{"single_char_value", "version=1", "version", "1", true},
		{"equals_no_value_at_end", "foo=bar&v=", "v", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotValue, gotFound := fastQueryVersion(tt.rawQuery, tt.param)
			assert.Equal(t, tt.wantValue, gotValue, "fastQueryVersion(%q, %q) value mismatch", tt.rawQuery, tt.param)
			assert.Equal(t, tt.wantFound, gotFound, "fastQueryVersion(%q, %q) found mismatch", tt.rawQuery, tt.param)
		})
	}
}

// TestFastQueryVersion_ZeroAlloc verifies zero allocations
// Note: Cannot use t.Parallel() with testing.AllocsPerRun
func TestFastQueryVersion_ZeroAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = fastQueryVersion("foo=bar&v=v1&baz=qux", "v")
	})

	assert.Equal(t, float64(0), allocs, "fastQueryVersion allocated %f times, want 0", allocs)
}

// TestFastHeaderVersion tests the fast header extraction
func TestFastHeaderVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headers    map[string][]string
		headerName string
		want       string
	}{
		{"found", map[string][]string{"API-Version": {"v1"}}, "API-Version", "v1"},
		{"not_found", map[string][]string{"Other": {"value"}}, "API-Version", ""},
		{"empty_header", map[string][]string{}, "API-Version", ""},
		{"multiple_values", map[string][]string{"API-Version": {"v1", "v2"}}, "API-Version", "v1"},
		{"empty_value", map[string][]string{"API-Version": {""}}, "API-Version", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fastHeaderVersion(tt.headers, tt.headerName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFastHeaderVersion_ZeroAlloc verifies zero allocations
// Note: Cannot use t.Parallel() with testing.AllocsPerRun
func TestFastHeaderVersion_ZeroAlloc(t *testing.T) {
	headers := map[string][]string{"API-Version": {"v1"}}

	allocs := testing.AllocsPerRun(100, func() {
		_ = fastHeaderVersion(headers, "API-Version")
	})

	assert.Equal(t, float64(0), allocs, "fastHeaderVersion allocated %f times, want 0", allocs)
}

// TestDetectVersion_Query tests version detection via query parameters
func TestDetectVersion_Query(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupRouter    func() *Router
		setupRequest   func(*http.Request)
		url            string
		wantVersion    string
		wantErrMessage string
	}{
		{
			name: "query_fast_path",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(*http.Request) {},
			url:          "/test?v=v2",
			wantVersion:  "v2",
		},
		{
			name: "query_with_validation",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithQueryVersioning("version"),
					WithValidVersions("v1", "v2", "v3"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(*http.Request) {},
			url:          "/test?version=v2",
			wantVersion:  "v2",
		},
		{
			name: "query_invalid_version",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithQueryVersioning("v"),
					WithValidVersions("v1", "v2"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest:   func(*http.Request) {},
			url:            "/test?v=invalid",
			wantVersion:    "v1",
			wantErrMessage: "should use default version",
		},
		{
			name: "header_priority",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithHeaderVersioning("API-Version"),
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("API-Version", "v2")
			},
			url:            "/test?v=v3",
			wantVersion:    "v2",
			wantErrMessage: "header should take priority over query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := tt.setupRouter()
			req := httptest.NewRequest("GET", tt.url, nil)
			tt.setupRequest(req)

			version := r.detectVersion(req)
			if tt.wantErrMessage != "" {
				assert.Equal(t, tt.wantVersion, version, tt.wantErrMessage)
			} else {
				assert.Equal(t, tt.wantVersion, version)
			}
		})
	}
}

// TestFastPathVersion tests the fast path version extraction
func TestFastPathVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		prefix    string
		wantValue string
		wantFound bool
	}{
		// Basic cases
		{"empty_path", "", "/v", "", false},
		{"empty_prefix", "/v1/users", "", "", false},
		{"simple_match", "/v1/users", "/v", "1", true},
		{"version_at_start", "/v2/posts", "/v", "2", true},
		{"version_with_number", "/v10/data", "/v", "10", true},

		// Path patterns
		{"pattern_with_api_prefix", "/api/v1/users", "/api/v", "1", true},
		{"pattern_with_slash_after_version", "/v1/users/123", "/v", "1", true},
		{"pattern_no_trailing_slash", "/v1", "/v", "1", true},

		// Boundary cases
		{"path_not_matching_prefix", "/api/v1/users", "/v", "", false},
		{"prefix_longer_than_path", "/v", "/version", "", false},
		{"exact_prefix_no_version", "/v", "/v", "", false},
		{"prefix_matches_but_no_segment", "/v/", "/v", "", true}, // Empty version segment

		// Version segment extraction
		{"version_with_underscore", "/v1_0/users", "/v", "1_0", true},
		{"version_with_dash", "/v1-0/users", "/v", "1-0", true},
		{"version_with_dot", "/v1.0/users", "/v", "1.0", true},
		{"long_version_string", "/v1.2.3.4-beta/users", "/v", "1.2.3.4-beta", true},

		// Edge cases
		{"multiple_slashes", "/v1//users", "/v", "1", true},
		{"version_at_end", "/v1", "/v", "1", true},
		{"version_with_trailing_slash", "/v1/", "/v", "1", true},
		{"nested_paths", "/v1/api/users/123", "/v", "1", true},

		// Different prefix patterns
		{"custom_prefix_simple", "/version1/data", "/version", "1", true},
		{"custom_prefix_long", "/api/v1/endpoint", "/api/v", "1", true},
		{"prefix_with_numbers", "/api2/v1/endpoint", "/api2/v", "1", true},

		// Special cases
		{"path_starting_with_version", "/v1", "/v", "1", true},
		{"path_only_version", "/v1", "/v", "1", true},
		{"version_followed_by_query", "/v1/users?id=123", "/v", "1", true}, // URL parsing happens separately
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotValue, gotFound := fastPathVersion(tt.path, tt.prefix)
			assert.Equal(t, tt.wantValue, gotValue, "fastPathVersion(%q, %q) value mismatch", tt.path, tt.prefix)
			assert.Equal(t, tt.wantFound, gotFound, "fastPathVersion(%q, %q) found mismatch", tt.path, tt.prefix)
		})
	}
}

// TestFastPathVersion_ZeroAlloc verifies zero allocations
// Note: Cannot use t.Parallel() with testing.AllocsPerRun
func TestFastPathVersion_ZeroAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = fastPathVersion("/v1/users", "/v")
	})

	assert.Equal(t, float64(0), allocs, "fastPathVersion allocated %f times, want 0", allocs)
}

// TestDetectVersion_Path tests path version detection (internal implementation)
func TestDetectVersion_Path(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupRouter    func() *Router
		setupRequest   func(*http.Request)
		url            string
		wantVersion    string
		wantErrMessage string
	}{
		{
			name: "path_fast_path",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/v{version}/"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(*http.Request) {},
			url:          "/v2/users",
			wantVersion:  "v2",
		},
		{
			name: "path_with_validation",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/v{version}/"),
					WithValidVersions("v1", "v2", "v3"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(*http.Request) {},
			url:          "/v2/users",
			wantVersion:  "v2",
		},
		{
			name: "path_invalid_version",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/v{version}/"),
					WithValidVersions("v1", "v2"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest:   func(*http.Request) {},
			url:            "/v99/users",
			wantVersion:    "v1",
			wantErrMessage: "should use default version",
		},
		{
			name: "path_priority_over_header",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/v{version}/"),
					WithHeaderVersioning("API-Version"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("API-Version", "v3")
			},
			url:            "/v2/users",
			wantVersion:    "v2",
			wantErrMessage: "path should take priority over header",
		},
		{
			name: "path_priority_over_query",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/v{version}/"),
					WithQueryVersioning("v"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest:   func(*http.Request) {},
			url:            "/v2/users?v=v3",
			wantVersion:    "v2",
			wantErrMessage: "path should take priority over query",
		},
		{
			name: "path_with_api_prefix",
			setupRouter: func() *Router {
				return MustNew(WithVersioning(
					WithPathVersioning("/api/v{version}/"),
					WithDefaultVersion("v1"),
				))
			},
			setupRequest: func(*http.Request) {},
			url:          "/api/v2/users",
			wantVersion:  "v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := tt.setupRouter()
			req := httptest.NewRequest("GET", tt.url, nil)
			tt.setupRequest(req)

			version := r.detectVersion(req)
			if tt.wantErrMessage != "" {
				assert.Equal(t, tt.wantVersion, version, tt.wantErrMessage)
			} else {
				assert.Equal(t, tt.wantVersion, version)
			}
		})
	}
}

// ============================================================================
// Accept Header Fast Path Tests
// ============================================================================

func TestFastAcceptVersion(t *testing.T) {
	// Note: Cannot use t.Parallel() at top level because subtests use AllocsPerRun
	t.Run("fastAcceptVersion_basic", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			accept  string
			pattern string
			want    string
			wantOk  bool
		}{
			{
				name:    "simple_match",
				accept:  "application/vnd.myapi.v2+json",
				pattern: "application/vnd.myapi.{version}+json",
				want:    "v2",
				wantOk:  true,
			},
			{
				name:    "with_multiple_types",
				accept:  "text/html, application/vnd.myapi.v3+json, application/json",
				pattern: "application/vnd.myapi.{version}+json",
				want:    "v3",
				wantOk:  true,
			},
			{
				name:    "no_match",
				accept:  "application/json",
				pattern: "application/vnd.myapi.{version}+json",
				want:    "",
				wantOk:  false,
			},
			{
				name:    "empty_accept",
				accept:  "",
				pattern: "application/vnd.myapi.{version}+json",
				want:    "",
				wantOk:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, ok := fastAcceptVersion(tt.accept, tt.pattern)
				assert.Equal(t, tt.wantOk, ok)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("fastAcceptVersion_zero_alloc", func(t *testing.T) {
		// Note: Cannot use t.Parallel() with testing.AllocsPerRun
		accept := "application/vnd.myapi.v2+json, text/html"
		pattern := "application/vnd.myapi.{version}+json"

		allocs := testing.AllocsPerRun(100, func() {
			_, _ = fastAcceptVersion(accept, pattern)
		})

		// Should have minimal allocations (string split may allocate)
		assert.LessOrEqual(t, allocs, float64(2), "fastAcceptVersion allocated %f times, want <= 2", allocs)
	})
}

// ============================================================================
// Observability Hooks Tests (Internal Implementation)
// ============================================================================

func TestObservabilityHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupRouter     func() (*Router, *string, *string, *int, *int, *string, *int)
		setupRequest    func(*http.Request)
		wantVersion     string
		wantDetectedVer string
		wantDetectedMet string
		wantCallCount   int
		wantMissingCnt  int
		wantInvalidVer  string
		wantInvalidCnt  int
	}{
		{
			name: "on_version_detected",
			setupRouter: func() (*Router, *string, *string, *int, *int, *string, *int) {
				var detectedVersion string
				var detectedMethod string
				callCount := 0
				var invalidVersion string
				invalidCount := 0
				missingCount := 0

				r := MustNew(WithVersioning(
					WithHeaderVersioning("X-API-Version"),
					WithDefaultVersion("v1"),
					WithVersionObserver(
						func(version string, method string) {
							detectedVersion = version
							detectedMethod = method
							callCount++
						},
						nil,
						nil,
					),
				))
				return r, &detectedVersion, &detectedMethod, &callCount, &missingCount, &invalidVersion, &invalidCount
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Version", "v2")
			},
			wantDetectedVer: "v2",
			wantDetectedMet: "header",
			wantCallCount:   1,
		},
		{
			name: "on_version_missing",
			setupRouter: func() (*Router, *string, *string, *int, *int, *string, *int) {
				var detectedVersion string
				var detectedMethod string
				callCount := 0
				missingCount := 0
				var invalidVersion string
				invalidCount := 0

				r := MustNew(WithVersioning(
					WithHeaderVersioning("X-API-Version"),
					WithDefaultVersion("v1"),
					WithVersionObserver(
						nil,
						func() {
							missingCount++
						},
						nil,
					),
				))
				return r, &detectedVersion, &detectedMethod, &callCount, &missingCount, &invalidVersion, &invalidCount
			},
			setupRequest: func(*http.Request) {
				// No X-API-Version header set
			},
			wantVersion:    "v1",
			wantMissingCnt: 1,
		},
		{
			name: "on_version_invalid",
			setupRouter: func() (*Router, *string, *string, *int, *int, *string, *int) {
				var detectedVersion string
				var detectedMethod string
				callCount := 0
				invalidCount := 0
				var invalidVersion string
				missingCount := 0

				r := MustNew(WithVersioning(
					WithHeaderVersioning("X-API-Version"),
					WithValidVersions("v1", "v2"),
					WithDefaultVersion("v1"),
					WithVersionObserver(
						nil,
						nil,
						func(attempted string) {
							invalidVersion = attempted
							invalidCount++
						},
					),
				))
				return r, &detectedVersion, &detectedMethod, &callCount, &missingCount, &invalidVersion, &invalidCount
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Version", "v99") // Invalid version
			},
			wantVersion:    "v1",
			wantInvalidVer: "v99",
			wantInvalidCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, detectedVersion, detectedMethod, callCount, missingCount, invalidVersion, invalidCount := tt.setupRouter()
			req := httptest.NewRequest("GET", "/users", nil)
			tt.setupRequest(req)

			version := r.detectVersion(req)

			if tt.wantVersion != "" {
				assert.Equal(t, tt.wantVersion, version)
			}
			if tt.wantDetectedVer != "" {
				assert.Equal(t, tt.wantDetectedVer, *detectedVersion)
			}
			if tt.wantDetectedMet != "" {
				assert.Equal(t, tt.wantDetectedMet, *detectedMethod)
			}
			if tt.wantCallCount > 0 {
				assert.Equal(t, tt.wantCallCount, *callCount)
			}
			if tt.wantMissingCnt > 0 {
				assert.Equal(t, tt.wantMissingCnt, *missingCount)
			}
			if tt.wantInvalidVer != "" {
				assert.Equal(t, tt.wantInvalidVer, *invalidVersion)
			}
			if tt.wantInvalidCnt > 0 {
				assert.Equal(t, tt.wantInvalidCnt, *invalidCount)
			}
		})
	}

	t.Run("multiple_detection_methods", func(t *testing.T) {
		t.Parallel()
		methods := []string{}

		r := MustNew(WithVersioning(
			WithPathVersioning("/v{version}/"),
			WithHeaderVersioning("X-API-Version"),
			WithAcceptVersioning("application/vnd.api.{version}+json"),
			WithQueryVersioning("v"),
			WithDefaultVersion("v1"),
			WithVersionObserver(
				func(_ string, method string) {
					methods = append(methods, method)
				},
				nil,
				nil,
			),
		))

		// Test path detection
		req1 := httptest.NewRequest("GET", "/v2/users", nil)
		_ = r.detectVersion(req1)
		assert.Contains(t, methods, "path")

		// Test header detection
		methods = []string{}
		req2 := httptest.NewRequest("GET", "/users", nil)
		req2.Header.Set("X-API-Version", "v2")
		_ = r.detectVersion(req2)
		assert.Contains(t, methods, "header")

		// Test accept detection
		methods = []string{}
		req3 := httptest.NewRequest("GET", "/users", nil)
		req3.Header.Set("Accept", "application/vnd.api.v2+json")
		_ = r.detectVersion(req3)
		assert.Contains(t, methods, "accept")

		// Test query detection
		methods = []string{}
		req4 := httptest.NewRequest("GET", "/users?v=v2", nil)
		_ = r.detectVersion(req4)
		assert.Contains(t, methods, "query")
	})
}

// ============================================================================
// Validation Helper Tests
// ============================================================================

func TestValidateVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cfg             *VersioningConfig
		input           string
		wantResult      string
		wantInvalidCall bool
		wantInvalidArg  string
	}{
		{
			name:       "no_validation_configured",
			cfg:        &VersioningConfig{},
			input:      "anyversion",
			wantResult: "anyversion",
		},
		{
			name: "valid_version",
			cfg: &VersioningConfig{
				ValidVersions: []string{"v1", "v2", "v3"},
			},
			input:      "v2",
			wantResult: "v2",
		},
		{
			name: "invalid_version",
			cfg: &VersioningConfig{
				ValidVersions: []string{"v1", "v2", "v3"},
			},
			input:           "v99",
			wantResult:      "",
			wantInvalidCall: true,
			wantInvalidArg:  "v99",
		},
		{
			name: "empty_version",
			cfg: &VersioningConfig{
				ValidVersions: []string{"v1", "v2"},
			},
			input:      "",
			wantResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup callback if needed for invalid version test
			var invalidAttempted string
			if tt.wantInvalidCall {
				tt.cfg.OnVersionInvalid = func(attempted string) {
					invalidAttempted = attempted
				}
			}

			got := tt.cfg.validateVersion(tt.input)

			assert.Equal(t, tt.wantResult, got)
			if tt.wantInvalidCall {
				assert.Equal(t, tt.wantInvalidArg, invalidAttempted)
			}
		})
	}
}

// ============================================================================
// Performance Regression Tests
// ============================================================================

func TestVersioningPerformance(t *testing.T) {
	// Note: Cannot use t.Parallel() because subtest uses AllocsPerRun
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	t.Run("detection_overhead", func(t *testing.T) {
		// Note: Cannot use t.Parallel() with testing.AllocsPerRun
		r := MustNew(WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		))

		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("X-API-Version", "v2")

		// Measure allocations
		allocs := testing.AllocsPerRun(100, func() {
			_ = r.detectVersion(req)
		})

		// Should have minimal allocations
		assert.LessOrEqual(t, allocs, float64(1), "detectVersion allocated %f times, want <= 1", allocs)
	})
}
