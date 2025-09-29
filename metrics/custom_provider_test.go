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

	// Create config with custom provider
	config, err := New(
		WithMeterProvider(customProvider),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify custom provider is used
	assert.True(t, config.customMeterProvider)
	assert.Equal(t, customProvider, config.meterProvider)

	// Verify metrics work
	config.IncrementCounter(context.Background(), "test_counter")
	config.RecordMetric(context.Background(), "test_metric", 1.5)
	config.SetGauge(context.Background(), "test_gauge", 42)

	// Shutdown should NOT shut down the custom provider (user manages it)
	err = config.Shutdown(context.Background())
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

	// Create config with both custom provider and built-in provider option
	// Built-in provider option should be ignored
	config, err := New(
		WithMeterProvider(customProvider),
		WithProvider(PrometheusProvider), // This should be ignored
		WithServiceName("test-service"),
	)
	require.NoError(t, err)

	// Verify custom provider is used, not Prometheus
	assert.True(t, config.customMeterProvider)
	assert.Nil(t, config.prometheusHandler) // Prometheus handler shouldn't be initialized
	assert.Nil(t, config.metricsServer)     // Prometheus server shouldn't be started

	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestNilCustomMeterProvider tests error handling for nil custom provider
func TestNilCustomMeterProvider(t *testing.T) {
	t.Parallel()

	config := newDefaultConfig()
	config.customMeterProvider = true
	config.meterProvider = nil

	err := config.initializeProvider()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "custom meter provider is nil")
}

// TestMultipleIndependentConfigurations demonstrates how to use custom providers
// for multiple independent metrics configurations without global state conflicts
func TestMultipleIndependentConfigurations(t *testing.T) {
	t.Parallel()

	// Create first metrics configuration with custom provider
	exporter1, err := stdoutmetric.New()
	require.NoError(t, err)

	provider1 := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter1)),
	)

	config1, err := New(
		WithMeterProvider(provider1),
		WithServiceName("service-1"),
	)
	require.NoError(t, err)

	// Create second metrics configuration with its own custom provider
	exporter2, err := stdoutmetric.New()
	require.NoError(t, err)

	provider2 := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter2)),
	)

	config2, err := New(
		WithMeterProvider(provider2),
		WithServiceName("service-2"),
	)
	require.NoError(t, err)

	// Both configurations should work independently
	config1.IncrementCounter(context.Background(), "service1_counter")
	config2.IncrementCounter(context.Background(), "service2_counter")

	// Cleanup
	assert.NoError(t, config1.Shutdown(context.Background()))
	assert.NoError(t, config2.Shutdown(context.Background()))
	assert.NoError(t, provider1.Shutdown(context.Background()))
	assert.NoError(t, provider2.Shutdown(context.Background()))
}
