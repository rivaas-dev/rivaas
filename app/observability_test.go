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

package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"
)

func TestPathFilter_ExactPaths(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPaths("/health", "/metrics", "/ready")

	assert.True(t, pf.shouldExclude("/health"))
	assert.True(t, pf.shouldExclude("/metrics"))
	assert.True(t, pf.shouldExclude("/ready"))
	assert.False(t, pf.shouldExclude("/api/users"))
	assert.False(t, pf.shouldExclude("/health/check"))
}

func TestPathFilter_Prefixes(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPrefixes("/debug/", "/internal/")

	assert.True(t, pf.shouldExclude("/debug/pprof"))
	assert.True(t, pf.shouldExclude("/internal/status"))
	assert.False(t, pf.shouldExclude("/api/users"))
	assert.False(t, pf.shouldExclude("/debugger"))
}

func TestPathFilter_Defaults(t *testing.T) {
	t.Parallel()

	pf := newPathFilterWithDefaults()

	// Default health paths should be excluded
	assert.True(t, pf.shouldExclude("/health"))
	assert.True(t, pf.shouldExclude("/healthz"))
	assert.True(t, pf.shouldExclude("/ready"))
	assert.True(t, pf.shouldExclude("/readyz"))
	assert.True(t, pf.shouldExclude("/live"))
	assert.True(t, pf.shouldExclude("/livez"))
	assert.True(t, pf.shouldExclude("/metrics"))

	// Default prefix should be excluded
	assert.True(t, pf.shouldExclude("/debug/pprof"))

	// Non-excluded paths
	assert.False(t, pf.shouldExclude("/api/users"))
}

func TestObservabilitySettings_Defaults(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()

	assert.NotNil(t, settings.pathFilter)
	assert.True(t, settings.accessLogging)
	assert.False(t, settings.logErrorsOnly)
	assert.Equal(t, time.Second, settings.slowThreshold)
}

func TestObservabilitySettings_WithExcludePaths(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	WithExcludePaths("/custom1", "/custom2")(settings)

	// Should add to defaults (additive behavior)
	assert.True(t, settings.pathFilter.shouldExclude("/custom1"))
	assert.True(t, settings.pathFilter.shouldExclude("/custom2"))
	assert.True(t, settings.pathFilter.shouldExclude("/health")) // Default preserved
}

func TestObservabilitySettings_WithoutDefaultExclusions(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	WithoutDefaultExclusions()(settings)
	WithExcludePaths("/custom1", "/custom2")(settings)

	// Should only have custom paths, defaults cleared
	assert.True(t, settings.pathFilter.shouldExclude("/custom1"))
	assert.True(t, settings.pathFilter.shouldExclude("/custom2"))
	assert.False(t, settings.pathFilter.shouldExclude("/health")) // Default removed
}

func TestObservabilitySettings_WithExcludePrefixes(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	WithExcludePrefixes("/admin/", "/internal/")(settings)

	assert.True(t, settings.pathFilter.shouldExclude("/admin/users"))
	assert.True(t, settings.pathFilter.shouldExclude("/internal/config"))
	assert.True(t, settings.pathFilter.shouldExclude("/debug/pprof")) // Default preserved
}

func TestObservabilitySettings_WithExcludePatterns(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	WithExcludePatterns(`^/v[0-9]+/internal/.*`)(settings)

	assert.True(t, settings.pathFilter.shouldExclude("/v1/internal/status"))
	assert.True(t, settings.pathFilter.shouldExclude("/v2/internal/config"))
	assert.False(t, settings.pathFilter.shouldExclude("/v1/users"))
}

func TestObservabilitySettings_WithAccessLogging(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	assert.True(t, settings.accessLogging)

	WithAccessLogging(false)(settings)
	assert.False(t, settings.accessLogging)
}

func TestObservabilitySettings_WithLogOnlyErrors(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	assert.False(t, settings.logErrorsOnly)

	WithLogOnlyErrors()(settings)
	assert.True(t, settings.logErrorsOnly)
}

func TestObservabilitySettings_WithSlowThreshold(t *testing.T) {
	t.Parallel()

	settings := defaultObservabilitySettings()
	assert.Equal(t, time.Second, settings.slowThreshold)

	WithSlowThreshold(500 * time.Millisecond)(settings)
	assert.Equal(t, 500*time.Millisecond, settings.slowThreshold)
}

func TestWithObservability_Integration(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithObservability(
			WithExcludePaths("/custom-health"),
			WithExcludePrefixes("/admin/"),
			WithLogOnlyErrors(),
			WithSlowThreshold(500*time.Millisecond),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	// Verify the settings were applied
	assert.NotNil(t, app.config.observability)
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/health"))
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/custom-health"))
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/admin/users"))
	assert.True(t, app.config.observability.logErrorsOnly)
	assert.Equal(t, 500*time.Millisecond, app.config.observability.slowThreshold)
}

func TestWithObservability_Components(t *testing.T) {
	t.Parallel()

	t.Run("metrics only", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
			),
		)
		require.NoError(t, err)
		assert.NotNil(t, app.metrics)
		assert.Nil(t, app.tracing)
	})

	t.Run("tracing only", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		assert.Nil(t, app.metrics)
		assert.NotNil(t, app.tracing)
	})

	t.Run("all components with shared settings", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithLogging(logging.WithJSONHandler()),
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
				WithTracing(tracing.WithNoop()),
				WithExcludePaths("/custom"),
				WithLogOnlyErrors(),
			),
		)
		require.NoError(t, err)
		assert.NotNil(t, app.logging)
		assert.NotNil(t, app.metrics)
		assert.NotNil(t, app.tracing)
		assert.True(t, app.config.observability.pathFilter.shouldExclude("/custom"))
		assert.True(t, app.config.observability.logErrorsOnly)
	})
}

func TestObservabilityRecorder_ExcludesHealthPaths(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
	)

	// Register health endpoint
	app.GET("/health", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	// Register API endpoint
	app.GET("/api/users", func(c *Context) {
		c.String(http.StatusOK, "users")
	})

	// Request to /health (should be excluded from observability)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Request to /api/users (should not be excluded)
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWithMetricsOnMainRouter(t *testing.T) {
	t.Parallel()

	t.Run("mounts metrics on main router", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(), // WithMetricsOnMainRouter provides the provider
				WithMetricsOnMainRouter("/metrics"),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app.metrics)

		// Metrics endpoint should be available on main router
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		// Should return prometheus format (contains # HELP or # TYPE)
		body := rec.Body.String()
		assert.True(t, strings.Contains(body, "# HELP") || strings.Contains(body, "# TYPE"),
			"expected prometheus format metrics")
	})

	t.Run("auto-disables separate server", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(), // WithMetricsOnMainRouter provides the provider
				WithMetricsOnMainRouter("/metrics"),
			),
		)
		require.NoError(t, err)

		// Separate metrics server should be disabled
		assert.Empty(t, app.GetMetricsServerAddress())
	})

	t.Run("custom path", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(), // WithMetricsOnMainRouter provides the provider
				WithMetricsOnMainRouter("/custom-metrics"),
			),
		)
		require.NoError(t, err)

		// Metrics should be at custom path
		req := httptest.NewRequest(http.MethodGet, "/custom-metrics", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("default path when empty", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(),               // WithMetricsOnMainRouter provides the provider
				WithMetricsOnMainRouter(""), // Empty should default to /metrics
			),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestWithMetricsSeparateServer(t *testing.T) {
	t.Parallel()

	t.Run("configures separate server", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(), // WithMetricsSeparateServer provides the provider
				WithMetricsSeparateServer(":19091", "/metrics"),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app.metrics)

		// Should have a separate server (not empty means server is running)
		assert.NotEmpty(t, app.GetMetricsServerAddress())
	})

	t.Run("error when both OnMainRouter and SeparateServer specified", func(t *testing.T) {
		t.Parallel()

		_, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(),
				WithMetricsOnMainRouter("/metrics"),
				WithMetricsSeparateServer(":19092", "/metrics"),
			),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("error when both SeparateServer and OnMainRouter specified", func(t *testing.T) {
		t.Parallel()

		_, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithMetrics(),
				WithMetricsSeparateServer(":19093", "/metrics"),
				WithMetricsOnMainRouter("/metrics"),
			),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})
}

func TestWithObservability_MultipleValidationErrors(t *testing.T) {
	t.Parallel()

	// Test that ALL observability validation errors are returned, not just the first one
	t.Run("all invalid regex patterns reported together", func(t *testing.T) {
		t.Parallel()

		_, err := New(
			WithServiceName("test-service"),
			WithObservability(
				WithExcludePatterns(
					`[invalid`,   // Invalid regex: unclosed bracket
					`(?invalid)`, // Invalid regex: unknown group flag
					`.*valid.*`,  // Valid pattern (should not cause error)
					`(unclosed`,  // Invalid regex: unclosed paren
				),
			),
		)
		require.Error(t, err)

		// Verify it's a ValidationError containing multiple errors
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.GreaterOrEqual(t, len(ve.Errors), 3, "should have at least 3 observability errors")

		// Check that all invalid patterns are reported
		errorStr := err.Error()
		assert.Contains(t, errorStr, "[invalid")
		assert.Contains(t, errorStr, "(?invalid)")
		assert.Contains(t, errorStr, "(unclosed")
	})

	t.Run("observability errors combined with other config errors", func(t *testing.T) {
		t.Parallel()

		_, err := New(
			WithServiceName(""), // Empty service name error
			WithObservability(
				WithExcludePatterns(`[invalid`), // Invalid regex error
			),
		)
		require.Error(t, err)

		// Should contain both errors
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.GreaterOrEqual(t, len(ve.Errors), 2, "should have at least 2 errors (serviceName + observability)")

		// Both errors should be present
		errorStr := err.Error()
		assert.Contains(t, errorStr, "serviceName")
		assert.Contains(t, errorStr, "[invalid")
	})
}
