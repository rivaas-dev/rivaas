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
	"fmt"
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
		cfg, err := newConfig(WithDefault("v1"))
		require.NoError(t, err)
		assert.Equal(t, "v1", cfg.defaultVersion)
	})

	t.Run("with header detection", func(t *testing.T) {
		t.Parallel()
		cfg, err := newConfig(
			WithHeaderDetection("X-API-Version"),
			WithDefault("v1"),
		)
		require.NoError(t, err)
		assert.Len(t, cfg.detectors, 1)
	})

	t.Run("with multiple detectors", func(t *testing.T) {
		t.Parallel()
		cfg, err := newConfig(
			WithPathDetection("/v{version}/"),
			WithHeaderDetection("X-API-Version"),
			WithQueryDetection("v"),
			WithDefault("v1"),
		)
		require.NoError(t, err)
		assert.Len(t, cfg.detectors, 3)
	})

	t.Run("with valid versions", func(t *testing.T) {
		t.Parallel()
		cfg, err := newConfig(
			WithDefault("v1"),
			WithValidVersions("v1", "v2", "v3"),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"v1", "v2", "v3"}, cfg.validVersions)
	})

	t.Run("with response headers", func(t *testing.T) {
		t.Parallel()
		cfg, err := newConfig(
			WithDefault("v1"),
			WithResponseHeaders(),
			WithWarning299(),
		)
		require.NoError(t, err)
		assert.True(t, cfg.sendVersionHeader)
		assert.True(t, cfg.sendWarning299)
	})

	t.Run("empty default version fails", func(t *testing.T) {
		t.Parallel()
		_, err := newConfig(WithDefault(""))
		assert.Error(t, err)
	})

	t.Run("invalid path pattern fails", func(t *testing.T) {
		t.Parallel()
		_, err := newConfig(
			WithPathDetection("/users"), // Missing {version}
			WithDefault("v1"),
		)
		assert.Error(t, err)
	})
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	t.Run("returns engine with valid options", func(t *testing.T) {
		t.Parallel()
		engine := MustNew(WithDefault("v1"))
		require.NotNil(t, engine)
		assert.Equal(t, "v1", engine.DefaultVersion())
	})

	t.Run("panics on invalid config", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			MustNew(WithDefault(""))
		})
	})
}

func TestNew_NilOptionFails(t *testing.T) {
	t.Parallel()

	engine, err := New(WithDefault("v1"), nil)
	require.Error(t, err)
	require.Nil(t, engine)
	assert.Contains(t, err.Error(), "cannot be nil")
	assert.Contains(t, err.Error(), "option at index 1")
}

func TestMustNew_NilOptionPanics(t *testing.T) {
	t.Parallel()

	var panicMsg string
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicMsg = fmt.Sprint(r)
			}
		}()
		MustNew(WithDefault("v1"), nil)
	}()
	require.NotEmpty(t, panicMsg, "MustNew with nil option should panic")
	assert.Contains(t, panicMsg, "cannot be nil")
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
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated()))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
	})

	t.Run("deprecated since", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", DeprecatedSince(date)))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
	})

	t.Run("sunset", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated(), Sunset(date)))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Equal(t, date.UTC().Format(http.TimeFormat), w.Header().Get("Sunset"))
	})

	t.Run("migration docs", func(t *testing.T) {
		t.Parallel()
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated(), MigrationDocs("https://docs.example.com")))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Contains(t, w.Header().Get("Link"), "https://docs.example.com")
	})

	t.Run("successor version", func(t *testing.T) {
		t.Parallel()
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated(), SuccessorVersion("v2")))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
	})

	t.Run("combined options", func(t *testing.T) {
		t.Parallel()
		date := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1",
			Deprecated(),
			Sunset(date),
			MigrationDocs("https://docs.example.com/migrate"),
			SuccessorVersion("v2"),
		))

		w := httptest.NewRecorder()
		engine.SetLifecycleHeaders(w, "v1", "/users")
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.Equal(t, date.UTC().Format(http.TimeFormat), w.Header().Get("Sunset"))
		assert.Contains(t, w.Header().Get("Link"), "https://docs.example.com/migrate")
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

		engine, err := New(WithDefault("v1"))
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated(), Sunset(sunsetDate)))

		w := httptest.NewRecorder()
		isSunset := engine.SetLifecycleHeaders(w, "v1", "/users")

		assert.False(t, isSunset)
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.NotEmpty(t, w.Header().Get("Sunset"))
	})

	t.Run("sunset enforcement", func(t *testing.T) {
		t.Parallel()
		pastDate := time.Now().Add(-30 * 24 * time.Hour)

		engine, err := New(WithDefault("v1"), WithSunsetEnforcement())
		require.NoError(t, err)
		require.NoError(t, engine.ApplyLifecycle("v1", Deprecated(), Sunset(pastDate)))

		w := httptest.NewRecorder()
		isSunset := engine.SetLifecycleHeaders(w, "v1", "/users")

		assert.True(t, isSunset)
	})
}

func TestApplyLifecycle_NilOptionReturnsError(t *testing.T) {
	t.Parallel()
	engine, err := New(WithDefault("v1"))
	require.NoError(t, err)

	err = engine.ApplyLifecycle("v1", Deprecated(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lifecycle option")
	assert.Contains(t, err.Error(), "cannot be nil")
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
	cfg, err := newConfig(WithDefault("v1"), WithSunsetEnforcement())
	require.NoError(t, err)
	assert.True(t, cfg.enforceSunset)
}

func TestConfig_Observer(t *testing.T) {
	t.Parallel()
	cfg, err := newConfig(
		WithDefault("v1"),
		WithObserver(OnDetected(func(_, _ string) {}), OnMissing(func() {}), OnInvalid(func(_ string) {})),
	)
	require.NoError(t, err)
	assert.NotNil(t, cfg.observer)
}

func TestNew_WithObserver_NilOptionReturnsError(t *testing.T) {
	t.Parallel()
	_, err := New(WithDefault("v1"), WithObserver(OnDetected(func(_, _ string) {}), nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "observer option")
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestEngine_ShouldApplyVersioning(t *testing.T) {
	t.Parallel()
	engine, err := New(WithDefault("v1"))
	require.NoError(t, err)
	// No path detector -> always apply when default is set
	assert.True(t, engine.ShouldApplyVersioning("/any/path"))
}

func TestEngine_NilReceiverPanics(t *testing.T) {
	t.Parallel()
	var nilEngine *Engine
	require.Panics(t, func() { nilEngine.ShouldApplyVersioning("/path") })
	require.Panics(t, func() { nilEngine.DefaultVersion() })
	require.Panics(t, func() { nilEngine.DetectVersion(nil) })
}
