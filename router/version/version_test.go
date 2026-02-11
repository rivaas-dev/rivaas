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

package version

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	t.Run("minimal config", func(t *testing.T) {
		t.Parallel()
		cfg, err := NewConfig(WithDefault("v1"))
		require.NoError(t, err)
		assert.Equal(t, "v1", cfg.DefaultVersion())
	})

	t.Run("with header detection", func(t *testing.T) {
		t.Parallel()
		cfg, err := NewConfig(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
		)
		require.NoError(t, err)
		assert.Len(t, cfg.Detectors(), 1)
	})

	t.Run("with multiple detectors", func(t *testing.T) {
		t.Parallel()
		cfg, err := NewConfig(
			WithPathDetection("/v{version}/"),
			WithHeaderDetection("X-API-Version"),
			WithQueryDetection("v"),
			WithDefault("v1"),
		)
		require.NoError(t, err)
		assert.Len(t, cfg.Detectors(), 3)
	})

	t.Run("with valid versions", func(t *testing.T) {
		t.Parallel()
		cfg, err := NewConfig(
			WithDefault("v1"),
			WithValidVersions("v1", "v2", "v3"),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"v1", "v2", "v3"}, cfg.ValidVersions())
	})

	t.Run("with response headers", func(t *testing.T) {
		t.Parallel()
		cfg, err := NewConfig(
			WithDefault("v1"),
			WithResponseHeaders(),
			WithWarning299(),
		)
		require.NoError(t, err)
		assert.True(t, cfg.SendVersionHeader())
		assert.True(t, cfg.SendWarning299())
	})

	t.Run("empty default version fails", func(t *testing.T) {
		t.Parallel()
		_, err := NewConfig(WithDefault(""))
		assert.Error(t, err)
	})

	t.Run("invalid path pattern fails", func(t *testing.T) {
		t.Parallel()
		_, err := NewConfig(
			WithPathDetection("/users"), // Missing {version}
			WithDefault("v1"),
		)
		assert.Error(t, err)
	})
}

func TestEngineDetectVersion(t *testing.T) {
	t.Parallel()

	t.Run("header detection", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("X-API-Version", "v2")

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v2", ver)
	})

	t.Run("header detection with validation", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
			WithValidVersions("v1", "v2"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("X-API-Version", "v99")

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v1", ver) // Fallback to default
	})

	t.Run("query detection", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithQueryDetection("v"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users?v=v2", nil)
		ver := engine.DetectVersion(req)
		assert.Equal(t, "v2", ver)
	})

	t.Run("path detection", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithPathDetection("/v{version}/"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
		ver := engine.DetectVersion(req)
		assert.Equal(t, "v2", ver)
	})

	t.Run("accept detection", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithAcceptDetection("application/vnd.myapi.{version}+json"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("Accept", "application/vnd.myapi.v2+json")

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v2", ver)
	})

	t.Run("custom detection", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithCustomDetection(func(r *http.Request) string {
				if r.Host == "v2.example.com" {
					return "v2"
				}

				return ""
			}),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Host = "v2.example.com"

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v2", ver)
	})

	t.Run("detection priority", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithPathDetection("/v{version}/"),
			WithHeaderDetection("X-API-Version"),
			WithQueryDetection("v"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		// Path takes priority over header
		req := httptest.NewRequest(http.MethodGet, "/v3/users", nil)
		req.Header.Set("X-API-Version", "v2")

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v3", ver)
	})

	t.Run("fallback to default", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		// No version header set

		ver := engine.DetectVersion(req)
		assert.Equal(t, "v1", ver)
	})
}

func TestEngineObserver(t *testing.T) {
	t.Parallel()

	t.Run("on detected callback", func(t *testing.T) {
		t.Parallel()
		var detectedVersion, detectedMethod string

		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
			WithObserver(
				OnDetected(func(v, m string) {
					detectedVersion = v
					detectedMethod = m
				}),
			),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("X-API-Version", "v2")
		engine.DetectVersion(req)

		assert.Equal(t, "v2", detectedVersion)
		assert.Equal(t, "header", detectedMethod)
	})

	t.Run("on missing callback", func(t *testing.T) {
		t.Parallel()
		missingCalled := false

		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
			WithObserver(
				OnMissing(func() {
					missingCalled = true
				}),
			),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		engine.DetectVersion(req)

		assert.True(t, missingCalled)
	})

	t.Run("on invalid callback", func(t *testing.T) {
		t.Parallel()
		var invalidVersion string

		engine, err := New(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
			WithValidVersions("v1", "v2"),
			WithObserver(
				OnInvalid(func(v string) {
					invalidVersion = v
				}),
			),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("X-API-Version", "v99")
		engine.DetectVersion(req)

		assert.Equal(t, "v99", invalidVersion)
	})
}

func TestLifecycleOptions(t *testing.T) {
	t.Parallel()

	t.Run("deprecated", func(t *testing.T) {
		t.Parallel()
		lc := ApplyLifecycleOptions(Deprecated())
		assert.True(t, lc.Deprecated)
		assert.False(t, lc.DeprecatedSince.IsZero())
	})

	t.Run("deprecated since", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		lc := ApplyLifecycleOptions(DeprecatedSince(date))
		assert.True(t, lc.Deprecated)
		assert.Equal(t, date, lc.DeprecatedSince)
	})

	t.Run("sunset", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		lc := ApplyLifecycleOptions(Sunset(date))
		assert.Equal(t, date, lc.SunsetDate)
	})

	t.Run("migration docs", func(t *testing.T) {
		t.Parallel()
		lc := ApplyLifecycleOptions(MigrationDocs("https://docs.example.com"))
		assert.Equal(t, "https://docs.example.com", lc.MigrationURL)
	})

	t.Run("successor version", func(t *testing.T) {
		t.Parallel()
		lc := ApplyLifecycleOptions(SuccessorVersion("v2"))
		assert.Equal(t, "v2", lc.Successor)
	})

	t.Run("combined options", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		lc := ApplyLifecycleOptions(
			Deprecated(),
			Sunset(date),
			MigrationDocs("https://docs.example.com/migrate"),
			SuccessorVersion("v2"),
		)

		assert.True(t, lc.Deprecated)
		assert.Equal(t, date, lc.SunsetDate)
		assert.Equal(t, "https://docs.example.com/migrate", lc.MigrationURL)
		assert.Equal(t, "v2", lc.Successor)
	})
}

func TestEngineSetLifecycleHeaders(t *testing.T) {
	t.Parallel()

	t.Run("version header", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithDefault("v1"),
			WithResponseHeaders(),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		isSunset := engine.SetLifecycleHeaders(w, "v1", "/users")

		assert.False(t, isSunset)
		assert.Equal(t, "v1", w.Header().Get("X-API-Version"))
	})

	t.Run("deprecated version headers", func(t *testing.T) {
		t.Parallel()
		sunsetDate := time.Now().Add(30 * 24 * time.Hour)

		engine, err := New(
			WithDefault("v1"),
		)
		require.NoError(t, err)

		// Set lifecycle
		lc := ApplyLifecycleOptions(
			Deprecated(),
			Sunset(sunsetDate),
		)
		engine.Config().SetLifecycle("v1", lc)

		w := httptest.NewRecorder()
		isSunset := engine.SetLifecycleHeaders(w, "v1", "/users")

		assert.False(t, isSunset)
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.NotEmpty(t, w.Header().Get("Sunset"))
	})

	t.Run("sunset enforcement", func(t *testing.T) {
		t.Parallel()
		pastDate := time.Now().Add(-30 * 24 * time.Hour)

		engine, err := New(
			WithDefault("v1"),
			WithSunsetEnforcement(),
		)
		require.NoError(t, err)

		// Set lifecycle with past sunset
		lc := ApplyLifecycleOptions(
			Deprecated(),
			Sunset(pastDate),
		)
		engine.Config().SetLifecycle("v1", lc)

		w := httptest.NewRecorder()
		isSunset := engine.SetLifecycleHeaders(w, "v1", "/users")

		assert.True(t, isSunset)
	})
}

func TestPathStripping(t *testing.T) {
	t.Parallel()

	t.Run("strip path version", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithPathDetection("/v{version}/"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		stripped := engine.StripPathVersion("/v2/users", "v2")
		assert.Equal(t, "/users", stripped)
	})

	t.Run("extract path segment", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithPathDetection("/v{version}/"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		segment, found := engine.ExtractPathSegment("/v2/users")
		assert.True(t, found)
		assert.Equal(t, "v2", segment)
	})

	t.Run("no version in path", func(t *testing.T) {
		t.Parallel()
		engine, err := New(
			WithPathDetection("/v{version}/"),
			WithDefault("v1"),
		)
		require.NoError(t, err)

		segment, found := engine.ExtractPathSegment("/users")
		assert.False(t, found)
		assert.Empty(t, segment)
	})
}

func TestConfig_EnforceSunset(t *testing.T) {
	t.Parallel()
	cfg, err := NewConfig(WithDefault("v1"), WithSunsetEnforcement())
	require.NoError(t, err)
	assert.True(t, cfg.EnforceSunset())
}

func TestConfig_Observer(t *testing.T) {
	t.Parallel()
	cfg, err := NewConfig(
		WithDefault("v1"),
		WithObserver(OnDetected(func(_, _ string) {}), OnMissing(func() {}), OnInvalid(func(_ string) {})),
	)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Observer())
}

func TestEngine_ShouldApplyVersioning(t *testing.T) {
	t.Parallel()
	engine, err := New(WithDefault("v1"))
	require.NoError(t, err)
	// No path detector -> always apply when default is set
	assert.True(t, engine.ShouldApplyVersioning("/any/path"))
	// Nil engine
	assert.False(t, (*Engine)(nil).ShouldApplyVersioning("/path"))
}
