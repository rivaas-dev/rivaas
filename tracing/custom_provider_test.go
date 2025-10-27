package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// TestWithCustomTracerProvider tests that users can provide their own tracer provider
func TestWithCustomTracerProvider(t *testing.T) {
	// Create a custom tracer provider
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("test-service"),
	)

	customProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Create config with custom provider
	config, err := New(
		WithTracerProvider(customProvider),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify custom provider is used
	assert.True(t, config.customTracerProvider)
	assert.Equal(t, customProvider, config.tracerProvider)

	// Verify tracing works
	ctx, span := config.StartSpan(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	config.FinishSpan(span, 200)

	// Shutdown should NOT shut down the custom provider (user manages it)
	err = config.Shutdown(context.Background())
	assert.NoError(t, err)

	// User should shutdown their own provider
	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestCustomProviderIgnoresBuiltInProvider tests that custom provider ignores built-in options
func TestCustomProviderIgnoresBuiltInProvider(t *testing.T) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("test-service"),
	)

	customProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Create config with both custom provider and built-in provider option
	// Built-in provider option should be ignored
	config, err := New(
		WithTracerProvider(customProvider),
		WithProvider(OTLPProvider), // This should be ignored
		WithServiceName("test-service"),
	)
	require.NoError(t, err)

	// Verify custom provider is used
	assert.True(t, config.customTracerProvider)

	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestMultipleIndependentTracingConfigurations demonstrates how to use custom providers
// for multiple independent tracing configurations without global state conflicts
func TestMultipleIndependentTracingConfigurations(t *testing.T) {
	// Create first tracing configuration with custom provider
	exporter1, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)

	provider1 := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter1),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("service-1"),
		)),
	)

	config1, err := New(
		WithTracerProvider(provider1),
		WithServiceName("service-1"),
	)
	require.NoError(t, err)

	// Create second tracing configuration with its own custom provider
	exporter2, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)

	provider2 := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter2),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("service-2"),
		)),
	)

	config2, err := New(
		WithTracerProvider(provider2),
		WithServiceName("service-2"),
	)
	require.NoError(t, err)

	// Both configurations should work independently
	ctx1, span1 := config1.StartSpan(context.Background(), "operation-1")
	assert.NotNil(t, ctx1)
	config1.FinishSpan(span1, 200)

	ctx2, span2 := config2.StartSpan(context.Background(), "operation-2")
	assert.NotNil(t, ctx2)
	config2.FinishSpan(span2, 200)

	// Cleanup - shutdown configs first (they won't shutdown the custom providers)
	assert.NoError(t, config1.Shutdown(context.Background()))
	assert.NoError(t, config2.Shutdown(context.Background()))

	// Then shutdown the custom providers
	assert.NoError(t, provider1.Shutdown(context.Background()))
	assert.NoError(t, provider2.Shutdown(context.Background()))
}

// TestWithTracerProviderAndCustomTracer tests using both custom provider and tracer
func TestWithTracerProviderAndCustomTracer(t *testing.T) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)

	customProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)

	customTracer := customProvider.Tracer("custom-tracer")

	config, err := New(
		WithTracerProvider(customProvider),
		WithCustomTracer(customTracer),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)

	// Verify both are set
	assert.True(t, config.customTracerProvider)
	assert.Equal(t, customTracer, config.tracer)

	err = customProvider.Shutdown(context.Background())
	assert.NoError(t, err)
}
