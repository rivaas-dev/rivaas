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

package tracing

import (
	"context"
	"fmt"
	"strings"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// initializeProvider initializes the tracing provider based on configuration.
// This is used for non-OTLP providers that don't require network connections.
func (t *Tracer) initializeProvider() error {
	switch t.provider {
	case NoopProvider:
		return t.initNoopProvider()
	case StdoutProvider:
		return t.initStdoutProvider()
	case OTLPProvider, OTLPHTTPProvider:
		// OTLP providers should use initializeProviderWithContext
		return errors.New("OTLP providers require context; use Start(ctx)")
	default:
		return fmt.Errorf("unsupported tracing provider: %s", t.provider)
	}
}

// initializeProviderWithContext initializes OTLP providers with a context.
// The context is used for network connection establishment.
func (t *Tracer) initializeProviderWithContext(ctx context.Context) error {
	switch t.provider {
	case OTLPProvider:
		return t.initOTLPProvider(ctx)
	case OTLPHTTPProvider:
		return t.initOTLPHTTPProvider(ctx)
	default:
		return fmt.Errorf("provider %s does not require context initialization", t.provider)
	}
}

// initNoopProvider creates a no-op tracer provider.
func (t *Tracer) initNoopProvider() error {
	// If user provided a custom tracer provider, use it
	if t.customTracerProvider {
		t.emitDebug("Using custom user-provided tracer provider")
		if t.tracer == nil {
			t.tracer = t.tracerProvider.Tracer("rivaas.dev/tracing")
		}
		if t.registerGlobal {
			t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "noop")
			otel.SetTracerProvider(t.tracerProvider)
		}

		return nil
	}

	// Create a tracer provider with no exporter
	res := createResource(t.serviceName, t.serviceVersion)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
	)

	t.sdkProvider = tp
	t.tracerProvider = tp
	t.tracer = tp.Tracer("rivaas.dev/tracing")

	if t.registerGlobal {
		t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "noop")
		otel.SetTracerProvider(tp)
	}

	return nil
}

// initStdoutProvider initializes the stdout trace exporter.
func (t *Tracer) initStdoutProvider() error {
	// If user provided a custom tracer provider, use it
	if t.customTracerProvider {
		t.emitDebug("Using custom user-provided tracer provider")
		if t.tracer == nil {
			t.tracer = t.tracerProvider.Tracer("rivaas.dev/tracing")
		}
		if t.registerGlobal {
			t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "stdout")
			otel.SetTracerProvider(t.tracerProvider)
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
	res := createResource(t.serviceName, t.serviceVersion)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	t.sdkProvider = tp
	t.tracerProvider = tp
	t.tracer = tp.Tracer("rivaas.dev/tracing")

	if t.registerGlobal {
		t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "stdout")
		otel.SetTracerProvider(tp)
	} else {
		t.emitDebug("Skipping global tracer provider registration", "provider", "stdout")
	}

	t.emitInfo("Tracing initialized", "provider", "stdout", "service", t.serviceName)

	return nil
}

// initOTLPProvider initializes the OTLP gRPC trace exporter.
// The context is used for connection establishment.
func (t *Tracer) initOTLPProvider(ctx context.Context) error {
	// If user provided a custom tracer provider, use it
	if t.customTracerProvider {
		t.emitDebug("Using custom user-provided tracer provider")
		if t.tracer == nil {
			t.tracer = t.tracerProvider.Tracer("rivaas.dev/tracing")
		}
		if t.registerGlobal {
			t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "otlp")
			otel.SetTracerProvider(t.tracerProvider)
		}

		return nil
	}

	// Build OTLP options
	opts := []otlptracegrpc.Option{}

	if t.otlpEndpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(t.otlpEndpoint))
	}

	if t.otlpInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	// Create OTLP exporter with the provided context
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	// Create resource with service information
	res := createResource(t.serviceName, t.serviceVersion)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	t.sdkProvider = tp
	t.tracerProvider = tp
	t.tracer = tp.Tracer("rivaas.dev/tracing")

	if t.registerGlobal {
		t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "otlp")
		otel.SetTracerProvider(tp)
	} else {
		t.emitDebug("Skipping global tracer provider registration", "provider", "otlp")
	}

	t.emitInfo("Tracing initialized", "provider", "otlp", "endpoint", t.otlpEndpoint, "service", t.serviceName)

	return nil
}

// initOTLPHTTPProvider initializes the OTLP HTTP trace exporter.
// The context is used for connection establishment.
func (t *Tracer) initOTLPHTTPProvider(ctx context.Context) error {
	// If user provided a custom tracer provider, use it
	if t.customTracerProvider {
		t.emitDebug("Using custom user-provided tracer provider")
		if t.tracer == nil {
			t.tracer = t.tracerProvider.Tracer("rivaas.dev/tracing")
		}
		if t.registerGlobal {
			t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "otlp-http")
			otel.SetTracerProvider(t.tracerProvider)
		}

		return nil
	}

	// Build OTLP HTTP options
	opts := []otlptracehttp.Option{}

	if t.otlpEndpoint != "" {
		// Parse endpoint to extract host:port and determine if HTTP or HTTPS
		endpoint := t.otlpEndpoint
		isHTTP := false

		// Remove protocol prefix if present
		if trimmed, ok := strings.CutPrefix(endpoint, "http://"); ok {
			endpoint = trimmed
			isHTTP = true
		} else if trimmedHTTPS, trimmedOk := strings.CutPrefix(endpoint, "https://"); trimmedOk {
			endpoint = trimmedHTTPS
		}

		// Remove trailing path if present
		if idx := strings.Index(endpoint, "/"); idx != -1 {
			endpoint = endpoint[:idx]
		}

		opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
		if isHTTP {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
	}

	// Create OTLP HTTP exporter with the provided context
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
	}

	// Create resource with service information
	res := createResource(t.serviceName, t.serviceVersion)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	t.sdkProvider = tp
	t.tracerProvider = tp
	t.tracer = tp.Tracer("rivaas.dev/tracing")

	if t.registerGlobal {
		t.emitDebug("Setting global OpenTelemetry tracer provider", "provider", "otlp-http")
		otel.SetTracerProvider(tp)
	} else {
		t.emitDebug("Skipping global tracer provider registration", "provider", "otlp-http")
	}

	t.emitInfo("Tracing initialized", "provider", "otlp-http", "endpoint", t.otlpEndpoint, "service", t.serviceName)

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
