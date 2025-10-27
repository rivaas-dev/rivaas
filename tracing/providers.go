package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// initNoopProvider creates a no-op tracer provider.
func (c *Config) initNoopProvider() error {
	// If user provided a custom tracer provider, skip initialization
	if c.customTracerProvider {
		c.logDebug("Using custom user-provided tracer provider")
		if c.tracer == nil {
			c.tracer = c.tracerProvider.Tracer("github.com/rivaas-dev/rivaas/tracing")
		}
		return nil
	}

	// Create a tracer provider with no exporter
	res := createResource(c.serviceName, c.serviceVersion)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
	)

	c.tracerProvider = tp
	c.tracer = tp.Tracer("github.com/rivaas-dev/rivaas/tracing")

	// Set global tracer provider
	c.logDebug("Setting global OpenTelemetry tracer provider", "provider", "noop")
	otel.SetTracerProvider(tp)

	return nil
}

// initStdoutProvider initializes the stdout trace exporter.
func (c *Config) initStdoutProvider() error {
	// If user provided a custom tracer provider, skip initialization
	if c.customTracerProvider {
		c.logDebug("Using custom user-provided tracer provider")
		if c.tracer == nil {
			c.tracer = c.tracerProvider.Tracer("github.com/rivaas-dev/rivaas/tracing")
		}
		return nil
	}

	// Create stdout exporter with pretty printing
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	// Create resource with service information
	res := createResource(c.serviceName, c.serviceVersion)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	c.tracerProvider = tp
	c.tracer = tp.Tracer("github.com/rivaas-dev/rivaas/tracing")

	// Set global tracer provider
	c.logDebug("Setting global OpenTelemetry tracer provider", "provider", "stdout")
	otel.SetTracerProvider(tp)

	c.logInfo("Tracing initialized", "provider", "stdout", "service", c.serviceName)
	return nil
}

// initOTLPProvider initializes the OTLP trace exporter.
func (c *Config) initOTLPProvider() error {
	// If user provided a custom tracer provider, skip initialization
	if c.customTracerProvider {
		c.logDebug("Using custom user-provided tracer provider")
		if c.tracer == nil {
			c.tracer = c.tracerProvider.Tracer("github.com/rivaas-dev/rivaas/tracing")
		}
		return nil
	}

	// Build OTLP options
	opts := []otlptracegrpc.Option{}

	if c.otlpEndpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(c.otlpEndpoint))
	}

	if c.otlpInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(context.Background(), opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res := createResource(c.serviceName, c.serviceVersion)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	c.tracerProvider = tp
	c.tracer = tp.Tracer("github.com/rivaas-dev/rivaas/tracing")

	// Set global tracer provider
	c.logDebug("Setting global OpenTelemetry tracer provider", "provider", "otlp")
	otel.SetTracerProvider(tp)

	c.logInfo("Tracing initialized", "provider", "otlp", "endpoint", c.otlpEndpoint, "service", c.serviceName)
	return nil
}

// createResource creates an OpenTelemetry resource with service information.
func createResource(serviceName, serviceVersion string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
	)
}
