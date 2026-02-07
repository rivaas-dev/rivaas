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

package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActiveRequestsReturnsToZero verifies that the http_requests_active gauge
// returns to 0 after all requests complete, and does not go negative.
// This tests the fix for the bug where increment and decrement used different attribute sets.
func TestActiveRequestsReturnsToZero(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "active-requests-test")

	ctx := t.Context()

	// Simulate multiple requests
	for range 5 {
		m := recorder.BeginRequest(ctx)
		require.NotNil(t, m, "BeginRequest should return metrics")

		// Simulate request completion
		recorder.Finish(ctx, m, 200, 100, "/test")
	}

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Parse http_requests_active values
	lines := strings.Split(body, "\n")
	var activeRequestsValues []string
	for _, line := range lines {
		if strings.HasPrefix(line, "http_requests_active") && !strings.HasPrefix(line, "#") {
			activeRequestsValues = append(activeRequestsValues, line)
		}
	}

	// Verify http_requests_active exists and has value 0
	// There should be exactly one series (with no route/status labels)
	require.Len(t, activeRequestsValues, 1, "Expected exactly one http_requests_active series")

	// The value should be 0 (not negative, not split across multiple series)
	assert.Contains(t, activeRequestsValues[0], " 0", "Active requests should be 0 after all requests complete")
	assert.NotContains(t, activeRequestsValues[0], "http_route", "Active requests should not have route labels")
	assert.NotContains(t, activeRequestsValues[0], "http_status_code", "Active requests should not have status code labels")
}

// TestActiveRequestsDuringInflight verifies that http_requests_active accurately
// tracks in-flight requests.
func TestActiveRequestsDuringInflight(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "inflight-test")

	ctx := t.Context()

	// Start 3 requests without finishing them
	m1 := recorder.BeginRequest(ctx)
	m2 := recorder.BeginRequest(ctx)
	m3 := recorder.BeginRequest(ctx)

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should show 3 active requests
	assert.Contains(t, body, "http_requests_active")
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "http_requests_active") && !strings.HasPrefix(line, "#") {
			assert.Contains(t, line, " 3", "Should show 3 active requests")
		}
	}

	// Finish all requests
	recorder.Finish(ctx, m1, 200, 100, "/api/users")
	recorder.Finish(ctx, m2, 404, 50, "/api/missing")
	recorder.Finish(ctx, m3, 500, 200, "/api/error")

	// Get metrics again
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body = w.Body.String()

	// Should be back to 0
	lines = strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "http_requests_active") && !strings.HasPrefix(line, "#") {
			assert.Contains(t, line, " 0", "Active requests should return to 0")
		}
	}
}

// TestWithoutScopeInfo verifies that the WithoutScopeInfo option removes
// otel_scope_* labels from Prometheus output.
func TestWithoutScopeInfo(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "no-scope-info",
		WithoutScopeInfo(),
	)

	ctx := t.Context()

	// Record a request
	m := recorder.BeginRequest(ctx)
	recorder.Finish(ctx, m, 200, 100, "/test")

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify otel_scope_* labels are absent
	assert.NotContains(t, body, "otel_scope_name", "Should not contain otel_scope_name label")
	assert.NotContains(t, body, "otel_scope_version", "Should not contain otel_scope_version label")
	assert.NotContains(t, body, "otel_scope_schema_url", "Should not contain otel_scope_schema_url label")

	// But metrics should still be present
	assert.Contains(t, body, "http_requests_total", "Should still contain metrics")
}

// TestWithScopeInfo verifies that by default, otel_scope_* labels are included.
func TestWithScopeInfo(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "with-scope-info")
	// Note: NOT using WithoutScopeInfo(), so scope info should be present

	ctx := t.Context()

	// Record a request
	m := recorder.BeginRequest(ctx)
	recorder.Finish(ctx, m, 200, 100, "/test")

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify otel_scope_name is present (default behavior)
	assert.Contains(t, body, "otel_scope_name", "Should contain otel_scope_name label by default")
	assert.Contains(t, body, "rivaas.dev/metrics", "Should contain the instrumentation scope name")
}

// TestWithoutTargetInfo verifies that the WithoutTargetInfo option removes
// the target_info metric from Prometheus output.
func TestWithoutTargetInfo(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "no-target-info",
		WithoutTargetInfo(),
	)

	ctx := t.Context()

	// Record a request
	m := recorder.BeginRequest(ctx)
	recorder.Finish(ctx, m, 200, 100, "/test")

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify target_info metric is absent
	assert.NotContains(t, body, "target_info", "Should not contain target_info metric")

	// But other metrics should still be present
	assert.Contains(t, body, "http_requests_total", "Should still contain HTTP metrics")
}

// TestWithTargetInfo verifies that by default, target_info metric is included.
func TestWithTargetInfo(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "with-target-info",
		WithServiceVersion("v1.2.3"),
	)
	// Note: NOT using WithoutTargetInfo(), so target_info should be present

	ctx := t.Context()

	// Record a request
	m := recorder.BeginRequest(ctx)
	recorder.Finish(ctx, m, 200, 100, "/test")

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify target_info is present with resource attributes
	assert.Contains(t, body, "target_info", "Should contain target_info metric by default")
	assert.Contains(t, body, "with-target-info", "Should contain service name in target_info")
	assert.Contains(t, body, "v1.2.3", "Should contain service version in target_info")
}

// TestServiceAttributesNotOnMetrics verifies that service.name and service.version
// are NOT added as labels on individual metric points (they should only be in target_info).
func TestServiceAttributesNotOnMetrics(t *testing.T) {
	t.Parallel()

	recorder := TestingRecorderWithPrometheus(t, "service-attrs-test",
		WithServiceVersion("v2.0.0"),
	)

	ctx := t.Context()

	// Record a request
	m := recorder.BeginRequest(ctx)
	recorder.Finish(ctx, m, 200, 100, "/api/test")

	// Get metrics output
	handler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	lines := strings.Split(body, "\n")

	// Find http_requests_total metric lines (not comments, not target_info)
	var requestsLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "http_requests_total{") {
			requestsLines = append(requestsLines, line)
		}
	}

	require.NotEmpty(t, requestsLines, "Should have http_requests_total metrics")

	// Verify service_name and service_version are NOT on http_requests_total
	for _, line := range requestsLines {
		assert.NotContains(t, line, "service_name=", "service_name should not be a label on http_requests_total")
		assert.NotContains(t, line, "service_version=", "service_version should not be a label on http_requests_total")
	}

	// But target_info should have them
	var targetInfoLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "target_info{") {
			targetInfoLines = append(targetInfoLines, line)
		}
	}

	require.NotEmpty(t, targetInfoLines, "Should have target_info metric")
	assert.Contains(t, targetInfoLines[0], "service_name=", "target_info should have service_name")
	assert.Contains(t, targetInfoLines[0], "service-attrs-test", "target_info should have correct service name")
	assert.Contains(t, targetInfoLines[0], "service_version=", "target_info should have service_version")
}
