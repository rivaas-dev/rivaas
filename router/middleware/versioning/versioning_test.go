package versioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"rivaas.dev/router"
)

func TestValidateVersions_Errors(t *testing.T) {
	deprecated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Before deprecated

	tests := []struct {
		name     string
		versions []VersionInfo
		wantErr  bool
		errMsg   string
	}{
		{
			name: "empty version",
			versions: []VersionInfo{
				{Version: ""},
			},
			wantErr: true,
		},
		{
			name: "duplicate versions",
			versions: []VersionInfo{
				{Version: "v1"},
				{Version: "v2"},
				{Version: "v1"},
			},
			wantErr: true,
			errMsg:  "duplicate version: v1",
		},
		{
			name: "sunset before deprecated",
			versions: []VersionInfo{
				{
					Version:    "v1",
					Deprecated: &deprecated,
					Sunset:     &sunset,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersions(tt.versions)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVersions_ValidConfigurations(t *testing.T) {
	tests := []struct {
		name     string
		versions []VersionInfo
	}{
		{
			name: "single version",
			versions: []VersionInfo{
				{Version: "v1"},
			},
		},
		{
			name: "multiple versions",
			versions: []VersionInfo{
				{Version: "v1"},
				{Version: "v2"},
				{Version: "v3"},
			},
		},
		{
			name: "deprecated only",
			versions: []VersionInfo{
				{
					Version:    "v1",
					Deprecated: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
		{
			name: "sunset only",
			versions: []VersionInfo{
				{
					Version: "v1",
					Sunset:  timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
		{
			name: "deprecated before sunset",
			versions: []VersionInfo{
				{
					Version:    "v1",
					Deprecated: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Sunset:     timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
		{
			name: "deprecated equals sunset",
			versions: []VersionInfo{
				{
					Version:    "v1",
					Deprecated: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
					Sunset:     timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersions(tt.versions)
			assert.NoError(t, err)
		})
	}
}

func TestWithVersioning_NoVersionDetected(t *testing.T) {
	r := router.MustNew()

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	r.Use(WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: "v1"},
		},
		Now: func() time.Time { return now },
	}))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithVersioning_UnknownVersion(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: "v1"},
		},
		Now: func() time.Time { return now },
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	// Set router to avoid nil pointer in Version() method
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v99") // Unknown version

	handler(c)

	// Should continue to next handler (no version-related headers set for unknown version)
	assert.Empty(t, w.Header().Get("X-API-Version"))
	assert.Empty(t, w.Header().Get("Deprecation"))
}

func TestWithVersioning_Headers(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		versionInfo       VersionInfo
		sendVersionHeader bool
		emitWarning299    bool
		expectHeaders     map[string]string
		checkHeaderExists []string
	}{
		{
			name: "X-API-Version header",
			versionInfo: VersionInfo{
				Version: "v1",
			},
			sendVersionHeader: true,
			expectHeaders: map[string]string{
				"X-API-Version": "v1",
			},
		},
		{
			name: "deprecated version",
			versionInfo: VersionInfo{
				Version:    "v1",
				Deprecated: &deprecated,
				DocsURL:    "https://docs.example.com/v1",
			},
			emitWarning299: true,
			expectHeaders: map[string]string{
				"Link": "<https://docs.example.com/v1>; rel=\"deprecation\"",
			},
			checkHeaderExists: []string{"Deprecation", "Warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithVersioning(Options{
				Versions:          []VersionInfo{tt.versionInfo},
				Now:               func() time.Time { return now },
				SendVersionHeader: tt.sendVersionHeader,
				EmitWarning299:    tt.emitWarning299,
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			r := router.MustNew()
			setRouter(c, r)
			setVersion(c, "v1")

			handler(c)

			for header, expected := range tt.expectHeaders {
				assert.Equal(t, expected, w.Header().Get(header))
			}

			for _, header := range tt.checkHeaderExists {
				assert.NotEmpty(t, w.Header().Get(header), "Expected %s header to be set", header)
			}
		})
	}
}

func TestWithVersioning_SunsetVersion(t *testing.T) {
	sunset := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // After sunset

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version: "v1",
				Sunset:  &sunset,
				DocsURL: "https://docs.example.com/v1",
			},
		},
		Now: func() time.Time { return now },
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	// Should return 410 Gone when past sunset
	assert.Equal(t, http.StatusGone, w.Code)
}

func TestWithVersioning_SunsetWithDeprecation(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // After deprecated, before sunset

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
				Sunset:     &sunset,
				DocsURL:    "https://docs.example.com/v1",
			},
		},
		Now:            func() time.Time { return now },
		EmitWarning299: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	// Check both Deprecation and Sunset headers
	assert.NotEmpty(t, w.Header().Get("Deprecation"))
	assert.NotEmpty(t, w.Header().Get("Sunset"))
	assert.NotEmpty(t, w.Header().Get("Link"))
}

func TestWithVersioning_OnDeprecatedUseCallback(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	callbackCalled := false
	var callbackVersion, callbackRoute string

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
			},
		},
		Now: func() time.Time { return now },
		OnDeprecatedUse: func(_ context.Context, version, route string) {
			callbackCalled = true
			callbackVersion = version
			callbackRoute = route
		},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")
	setRouteTemplate(c, "/test")

	handler(c)

	// Wait a bit for async callback
	time.Sleep(100 * time.Millisecond)

	assert.True(t, callbackCalled, "Expected OnDeprecatedUse callback to be called")
	assert.Equal(t, "v1", callbackVersion)
	assert.Equal(t, "/test", callbackRoute)
}

func TestWithVersioning_NotDeprecatedYet(t *testing.T) {
	deprecated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Before deprecated

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
			},
		},
		Now: func() time.Time { return now },
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	// Should not have deprecation headers
	assert.Empty(t, w.Header().Get("Deprecation"))
}

func TestWithVersioning_Warning299WithSunset(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
				Sunset:     &sunset,
			},
		},
		Now:            func() time.Time { return now },
		EmitWarning299: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	warningHeader := w.Header().Get("Warning")
	assert.NotEmpty(t, warningHeader, "Expected Warning header")

	// Should contain sunset date
	expectedDate := sunset.Format("2006-01-02")
	assert.Contains(t, warningHeader, expectedDate)
}

func TestWithVersioning_Warning299WithoutSunset(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
			},
		},
		Now:            func() time.Time { return now },
		EmitWarning299: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	warningHeader := w.Header().Get("Warning")
	assert.NotEmpty(t, warningHeader, "Expected Warning header")

	// Should not contain sunset date
	assert.NotContains(t, warningHeader, "removed on")
}

func TestWithVersioning_DefaultNowFunction(t *testing.T) {
	// Test that default Now function is used when not provided
	// This should not panic
	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: "v1"},
		},
	})

	assert.NotNil(t, handler, "Expected handler to be created")
}

func TestWithVersioning_InvalidVersionConfigPanics(t *testing.T) {
	assert.Panics(t, func() {
		WithVersioning(Options{
			Versions: []VersionInfo{
				{Version: ""}, // Invalid
			},
		})
	}, "Expected panic for invalid version configuration")
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setVersion uses reflection to set the unexported version field for testing
func setVersion(c *router.Context, version string) {
	rv := reflect.ValueOf(c).Elem()
	versionField := rv.FieldByName("version")
	if !versionField.IsValid() {
		panic("version field not found in Context")
	}
	if !versionField.CanSet() {
		// Try using unsafe pointer if CanSet returns false
		versionField = reflect.NewAt(versionField.Type(), unsafe.Pointer(versionField.UnsafeAddr())).Elem()
	}
	versionField.SetString(version)
}

// setRouteTemplate uses reflection to set the unexported routeTemplate field for testing
func setRouteTemplate(c *router.Context, template string) {
	rv := reflect.ValueOf(c).Elem()
	templateField := rv.FieldByName("routeTemplate")
	if !templateField.IsValid() {
		panic("routeTemplate field not found in Context")
	}
	if !templateField.CanSet() {
		// Use unsafe pointer to set unexported field
		templateField = reflect.NewAt(templateField.Type(), unsafe.Pointer(templateField.UnsafeAddr())).Elem()
	}
	templateField.SetString(template)
}

// setRouter uses reflection to set the unexported router field for testing
func setRouter(c *router.Context, r *router.Router) {
	rv := reflect.ValueOf(c).Elem()
	routerField := rv.FieldByName("router")
	if !routerField.IsValid() {
		panic("router field not found in Context")
	}
	if !routerField.CanSet() {
		// Use unsafe pointer to set unexported field
		routerField = reflect.NewAt(routerField.Type(), unsafe.Pointer(routerField.UnsafeAddr())).Elem()
	}
	routerField.Set(reflect.ValueOf(r))
}
