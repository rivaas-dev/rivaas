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
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Option defines functional options for Tracer configuration.
// Options are applied during Tracer creation via New().
type Option func(*Tracer)

// WithTracerProvider allows you to provide a custom OpenTelemetry TracerProvider.
// When using this option, the package will NOT set the global otel.SetTracerProvider()
// by default. Use WithGlobalTracerProvider() if you want global registration.
//
// This is useful when:
//   - You want to manage the tracer provider lifecycle yourself
//   - You need multiple independent tracing configurations
//   - You want to avoid global state in your application
//
// Example:
//
//	tp := sdktrace.NewTracerProvider(...)
//	tracer, err := tracing.New(
//	    tracing.WithTracerProvider(tp),
//	    tracing.WithServiceName("my-service"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
//
// Note: When using WithTracerProvider, provider options (WithOTLP, WithStdout, etc.)
// are ignored since you're managing the provider yourself.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return func(t *Tracer) {
		t.tracerProvider = provider
		t.customTracerProvider = true
	}
}

// WithGlobalTracerProvider registers the tracer provider as the global
// OpenTelemetry tracer provider via otel.SetTracerProvider().
// By default, tracer providers are not registered globally to allow multiple
// tracing configurations to coexist in the same process.
//
// Example:
//
//	tracer := tracing.New(
//	    tracing.WithOTLP("localhost:4317"),
//	    tracing.WithGlobalTracerProvider(), // Register as global default
//	)
func WithGlobalTracerProvider() Option {
	return func(t *Tracer) {
		t.registerGlobal = true
	}
}

// WithServiceName sets the service name for tracing.
// This name appears in span attributes as 'service.name'.
//
// Example:
//
//	tracer := tracing.New(tracing.WithServiceName("my-api"))
func WithServiceName(name string) Option {
	return func(t *Tracer) {
		t.serviceName = name
	}
}

// WithServiceVersion sets the service version for tracing.
// This version appears in span attributes as 'service.version'.
//
// Example:
//
//	tracer := tracing.New(tracing.WithServiceVersion("v1.2.3"))
func WithServiceVersion(version string) Option {
	return func(t *Tracer) {
		t.serviceVersion = version
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0).
// Values outside this range will be clamped to valid bounds.
//
// A rate of 1.0 samples all requests, 0.5 samples 50%, and 0.0 samples none.
// Sampling decisions are made per-request based on the configured rate.
//
// Example:
//
//	tracer := tracing.New(tracing.WithSampleRate(0.1)) // Sample 10% of requests
func WithSampleRate(rate float64) Option {
	return func(t *Tracer) {
		if rate < 0.0 {
			rate = 0.0
		}
		if rate > 1.0 {
			rate = 1.0
		}
		t.sampleRate = rate
	}
}

// WithCustomTracer allows using a custom OpenTelemetry tracer.
// This is useful when you need specific tracer configuration or
// want to use a tracer from an existing OpenTelemetry setup.
//
// Example:
//
//	tp := trace.NewTracerProvider(...)
//	tracer := tp.Tracer("my-tracer")
//	t := tracing.New(tracing.WithCustomTracer(tracer))
func WithCustomTracer(tracer trace.Tracer) Option {
	return func(t *Tracer) {
		t.tracer = tracer
	}
}

// WithCustomPropagator allows using a custom OpenTelemetry propagator.
// This is useful for custom trace context propagation formats.
// By default, uses the global propagator from otel.GetTextMapPropagator().
//
// Example:
//
//	prop := propagation.TraceContext{}
//	tracer := tracing.New(tracing.WithCustomPropagator(prop))
func WithCustomPropagator(propagator propagation.TextMapPropagator) Option {
	return func(t *Tracer) {
		t.propagator = propagator
	}
}

// WithEventHandler sets a custom event handler for internal operational events.
// Use this for advanced use cases like sending errors to Sentry, custom alerting,
// or integrating with non-slog logging systems.
//
// Example:
//
//	tracing.New(tracing.WithEventHandler(func(e tracing.Event) {
//	    if e.Type == tracing.EventError {
//	        sentry.CaptureMessage(e.Message)
//	    }
//	    myLogger.Log(e.Type, e.Message, e.Args...)
//	}))
func WithEventHandler(handler EventHandler) Option {
	return func(t *Tracer) {
		t.eventHandler = handler
	}
}

// WithLogger sets the logger for internal operational events using the default event handler.
// This is a convenience wrapper around WithEventHandler that logs events to the provided slog.Logger.
//
// Example:
//
//	// Use stdlib slog
//	tracing.New(tracing.WithLogger(slog.Default()))
//
//	// Use custom slog logger
//	:= slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	tracing.New(tracing.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return WithEventHandler(DefaultEventHandler(logger))
}

// WithSpanStartHook sets a callback that is invoked when a request span is started.
// The hook receives the context, span, and HTTP request, allowing custom attribute
// injection, dynamic sampling decisions, or integration with APM tools.
//
// Example:
//
//	hook := func(ctx context.Context, span trace.Span, req *http.Request) {
//	    if tenantID := req.Header.Get("X-Tenant-ID"); tenantID != "" {
//	        span.SetAttributes(attribute.String("tenant.id", tenantID))
//	    }
//	}
//	tracer := tracing.New(tracing.WithSpanStartHook(hook))
func WithSpanStartHook(hook SpanStartHook) Option {
	return func(t *Tracer) {
		t.spanStartHook = hook
	}
}

// WithSpanFinishHook sets a callback that is invoked when a request span is finished.
// The hook receives the span and HTTP status code, allowing custom metrics recording,
// logging, or post-processing.
//
// Example:
//
//	hook := func(span trace.Span, statusCode int) {
//	    if statusCode >= 500 {
//	        metrics.IncrementServerErrors()
//	    }
//	}
//	tracer := tracing.New(tracing.WithSpanFinishHook(hook))
func WithSpanFinishHook(hook SpanFinishHook) Option {
	return func(t *Tracer) {
		t.spanFinishHook = hook
	}
}

// OTLPOption configures OTLP provider behavior.
type OTLPOption func(*otlpConfig)

type otlpConfig struct {
	insecure bool
}

// OTLPInsecure enables insecure gRPC for OTLP.
// Default is false (uses TLS). Set to true for local development.
func OTLPInsecure() OTLPOption {
	return func(c *otlpConfig) {
		c.insecure = true
	}
}

// WithOTLP configures OTLP gRPC provider with endpoint.
// Endpoint format: "host:port" (e.g., "localhost:4317")
//
// Only one provider can be configured. Configuring multiple providers
// (e.g., WithOTLP and WithStdout) will result in a validation error.
//
// Example:
//
//	// Simple:
//	tracer := tracing.MustNew(tracing.WithOTLP("localhost:4317"))
//
//	// With insecure option:
//	tracer := tracing.MustNew(tracing.WithOTLP("localhost:4317", tracing.OTLPInsecure()))
func WithOTLP(endpoint string, opts ...OTLPOption) Option {
	return func(t *Tracer) {
		if t.providerSet {
			t.validationErrors = append(t.validationErrors,
				fmt.Errorf("provider: multiple providers configured (already have %q, cannot add %q); only one provider allowed", t.provider, OTLPProvider))

			return
		}
		t.provider = OTLPProvider
		t.otlpEndpoint = endpoint
		t.providerSet = true
		cfg := &otlpConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		t.otlpInsecure = cfg.insecure
	}
}

// WithOTLPHTTP configures OTLP HTTP provider with endpoint.
// Endpoint format: "http://host:port" (e.g., "http://localhost:4318")
//
// Only one provider can be configured. Configuring multiple providers
// will result in a validation error.
//
// Example:
//
//	tracer := tracing.MustNew(tracing.WithOTLPHTTP("http://localhost:4318"))
func WithOTLPHTTP(endpoint string) Option {
	return func(t *Tracer) {
		if t.providerSet {
			t.validationErrors = append(t.validationErrors,
				fmt.Errorf("provider: multiple providers configured (already have %q, cannot add %q); only one provider allowed", t.provider, OTLPHTTPProvider))

			return
		}
		t.provider = OTLPHTTPProvider
		t.otlpEndpoint = endpoint
		t.providerSet = true
	}
}

// WithStdout configures stdout provider for development/debugging.
//
// Only one provider can be configured. Configuring multiple providers
// will result in a validation error.
//
// Example:
//
//	tracer := tracing.MustNew(tracing.WithStdout())
func WithStdout() Option {
	return func(t *Tracer) {
		if t.providerSet {
			t.validationErrors = append(t.validationErrors,
				fmt.Errorf("provider: multiple providers configured (already have %q, cannot add %q); only one provider allowed", t.provider, StdoutProvider))

			return
		}
		t.provider = StdoutProvider
		t.providerSet = true
	}
}

// WithNoop configures noop provider (default, no traces exported).
//
// Only one provider can be configured. Configuring multiple providers
// will result in a validation error.
//
// Example:
//
//	tracer := tracing.MustNew(tracing.WithNoop())
func WithNoop() Option {
	return func(t *Tracer) {
		if t.providerSet {
			t.validationErrors = append(t.validationErrors,
				fmt.Errorf("provider: multiple providers configured (already have %q, cannot add %q); only one provider allowed", t.provider, NoopProvider))

			return
		}
		t.provider = NoopProvider
		t.providerSet = true
	}
}

// SpanStartHook is called when a request span is started.
// It receives the context, span, and HTTP request.
// This can be used for custom attribute injection, dynamic sampling, or integration with APM tools.
type SpanStartHook func(ctx context.Context, span trace.Span, req *http.Request)

// SpanFinishHook is called when a request span is finished.
// It receives the span and the HTTP status code.
// This can be used for custom metrics, logging, or post-processing.
type SpanFinishHook func(span trace.Span, statusCode int)
