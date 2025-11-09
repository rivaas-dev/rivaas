package versioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"rivaas.dev/router"
)

func TestValidateVersions_EmptyVersion(t *testing.T) {
	versions := []VersionInfo{
		{Version: ""},
	}

	err := ValidateVersions(versions)
	if err == nil {
		t.Error("Expected error for empty version string")
	}
}

func TestValidateVersions_DuplicateVersions(t *testing.T) {
	versions := []VersionInfo{
		{Version: "v1"},
		{Version: "v2"},
		{Version: "v1"}, // Duplicate
	}

	err := ValidateVersions(versions)
	if err == nil {
		t.Error("Expected error for duplicate versions")
	}

	if err.Error() != "duplicate version: v1" {
		t.Errorf("Expected duplicate version error, got %v", err)
	}
}

func TestValidateVersions_SunsetBeforeDeprecated(t *testing.T) {
	deprecated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Before deprecated

	versions := []VersionInfo{
		{
			Version:    "v1",
			Deprecated: &deprecated,
			Sunset:     &sunset,
		},
	}

	err := ValidateVersions(versions)
	if err == nil {
		t.Error("Expected error for sunset before deprecated")
	}

	if err.Error() == "" {
		t.Error("Expected detailed error message")
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
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
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
	if w.Header().Get("X-API-Version") != "" {
		t.Errorf("Expected no X-API-Version header for unknown version, got %s", w.Header().Get("X-API-Version"))
	}
	if w.Header().Get("Deprecation") != "" {
		t.Errorf("Expected no Deprecation header for unknown version, got %s", w.Header().Get("Deprecation"))
	}
}

func TestWithVersioning_XAPIVersionHeader(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: "v1"},
		},
		Now:               func() time.Time { return now },
		SendVersionHeader: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	r := router.MustNew()
	setRouter(c, r)
	setVersion(c, "v1")

	handler(c)

	if w.Header().Get("X-API-Version") != "v1" {
		t.Errorf("Expected X-API-Version: v1, got %s", w.Header().Get("X-API-Version"))
	}
}

func TestWithVersioning_DeprecatedVersion(t *testing.T) {
	deprecated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // After deprecated

	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{
				Version:    "v1",
				Deprecated: &deprecated,
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

	// Check Deprecation header
	depHeader := w.Header().Get("Deprecation")
	if depHeader == "" {
		t.Error("Expected Deprecation header to be set")
	}

	// Check Link header with deprecation relation
	linkHeader := w.Header().Get("Link")
	if linkHeader != "<https://docs.example.com/v1>; rel=\"deprecation\"" {
		t.Errorf("Expected Link header with deprecation, got %s", linkHeader)
	}

	// Check Warning header
	warningHeader := w.Header().Get("Warning")
	if warningHeader == "" {
		t.Error("Expected Warning header to be set")
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
	if w.Code != http.StatusGone {
		t.Errorf("Expected 410 Gone, got %d", w.Code)
	}

	// When past sunset, middleware returns early (before setting Sunset header)
	// This is expected behavior - the 410 response indicates the version is gone
	// The Sunset header would be set if the version wasn't past sunset yet
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
	depHeader := w.Header().Get("Deprecation")
	if depHeader == "" {
		t.Error("Expected Deprecation header")
	}

	sunsetHeader := w.Header().Get("Sunset")
	if sunsetHeader == "" {
		t.Error("Expected Sunset header")
	}

	// Check Link header contains both relations
	linkHeader := w.Header().Get("Link")
	if linkHeader == "" {
		t.Error("Expected Link header")
	}
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

	if !callbackCalled {
		t.Error("Expected OnDeprecatedUse callback to be called")
	}

	if callbackVersion != "v1" {
		t.Errorf("Expected callback version v1, got %s", callbackVersion)
	}

	if callbackRoute != "/test" {
		t.Errorf("Expected callback route /test, got %s", callbackRoute)
	}
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
	depHeader := w.Header().Get("Deprecation")
	if depHeader != "" {
		t.Errorf("Expected no Deprecation header, got %s", depHeader)
	}
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
	if warningHeader == "" {
		t.Error("Expected Warning header")
	}

	// Should contain sunset date
	expectedDate := sunset.Format("2006-01-02")
	if warningHeader == "" || !contains(warningHeader, expectedDate) {
		t.Errorf("Expected Warning header to contain sunset date %s, got %s", expectedDate, warningHeader)
	}
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
	if warningHeader == "" {
		t.Error("Expected Warning header")
	}

	// Should not contain sunset date
	if contains(warningHeader, "removed on") {
		t.Errorf("Warning header should not contain sunset date when not configured, got %s", warningHeader)
	}
}

func TestWithVersioning_DefaultNowFunction(t *testing.T) {
	// Test that default Now function is used when not provided
	// This should not panic
	handler := WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: "v1"},
		},
	})

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestWithVersioning_InvalidVersionConfigPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid version configuration")
		}
	}()

	WithVersioning(Options{
		Versions: []VersionInfo{
			{Version: ""}, // Invalid
		},
	})
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
