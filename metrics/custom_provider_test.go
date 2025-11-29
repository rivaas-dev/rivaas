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

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestWithCustomMeterProvider tests that users can provide their own meter provider
func TestWithCustomMeterProvider(t *testing.T) {
	t.Parallel()

	// Create a custom meter provider
	exporter, err := stdoutmetric.New()
	require.NoError(t, err)

	reader := sdkmetric.NewPeriodicReader(exporter)
	customProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Create recorder with custom provider
	recorder, err := New(
		WithMeterProvider(customProvider),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)
	assert.NotNil(t, recorder)

	// Verify custom provider is used
	assert.True(t, recorder.customMeterProvider)
	assert.Equal(t, customProvider, recorder.meterProvider)

	// Verify metrics work (errors are returned but we ignore them for this test)
	_ = recorder.IncrementCounter(context.Background(), "test_counter")
	_ = recorder.RecordHistogram(context.Background(), "test_metric", 1.5)
	_ = recorder.SetGauge(context.Background(), "test_gauge", 42)

	// Shutdown should NOT shut down the custom provider (user manages it)
	err = recorder.Shutdown(context.Background())
	assert.NoError(t, err)

	// User should shutdown their own provider
	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestCustomProviderIgnoresBuiltInProvider tests that custom provider ignores built-in options
func TestCustomProviderIgnoresBuiltInProvider(t *testing.T) {
	t.Parallel()

	exporter, err := stdoutmetric.New()
	require.NoError(t, err)

	reader := sdkmetric.NewPeriodicReader(exporter)
	customProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Create recorder with both custom provider and built-in provider option
	// Built-in provider option should be ignored
	recorder, err := New(
		WithMeterProvider(customProvider),
		WithPrometheus(":9090", "/metrics"), // This should be ignored
		WithServiceName("test-service"),
	)
	require.NoError(t, err)

	// Verify custom provider is used, not Prometheus
	assert.True(t, recorder.customMeterProvider)
	assert.Nil(t, recorder.prometheusHandler) // Prometheus handler shouldn't be initialized
	assert.Nil(t, recorder.metricsServer)     // Prometheus server shouldn't be started

	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestNilCustomMeterProvider tests error handling for nil custom provider
func TestNilCustomMeterProvider(t *testing.T) {
	t.Parallel()

	recorder := newDefaultRecorder()
	recorder.customMeterProvider = true
	recorder.meterProvider = nil

	err := recorder.initializeProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// TestGlobalMeterProviderOption tests the WithGlobalMeterProvider option
func TestGlobalMeterProviderOption(t *testing.T) {
	t.Parallel()

	recorder := newDefaultRecorder()
	assert.False(t, recorder.registerGlobal) // Default is false

	// Apply option
	WithGlobalMeterProvider()(recorder)
	assert.True(t, recorder.registerGlobal)
}

// TestCustomProviderWithGlobalRegistration tests combining custom provider with global registration
func TestCustomProviderWithGlobalRegistration(t *testing.T) {
	t.Parallel()

	exporter, err := stdoutmetric.New()
	require.NoError(t, err)

	reader := sdkmetric.NewPeriodicReader(exporter)
	customProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Even with custom provider, user can request global registration
	recorder, err := New(
		WithMeterProvider(customProvider),
		WithGlobalMeterProvider(), // User can still request global registration
		WithServiceName("test-service"),
	)
	require.NoError(t, err)

	assert.True(t, recorder.customMeterProvider)
	assert.True(t, recorder.registerGlobal)

	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}
