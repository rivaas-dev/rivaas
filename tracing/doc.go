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

// Package tracing provides comprehensive OpenTelemetry-based distributed tracing
// for Go applications. It supports multiple exporters (Stdout, OTLP, Noop)
// and integrates seamlessly with HTTP frameworks.
//
// # Basic Usage
//
//	import (
//	    "context"
//	    "log"
//	    "rivaas.dev/tracing"
//	)
//
//	tracer, err := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithServiceVersion("v1.0.0"),
//	    tracing.WithStdout(),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tracer.Shutdown(context.Background())
//
// # Providers
//
// Three providers are supported with convenient options:
//
//   - WithNoop() (default): No traces exported (safe default)
//   - WithStdout(): Prints traces to stdout (for development/testing)
//   - WithOTLP(endpoint): Sends traces to OTLP collector via gRPC (for production)
//   - WithOTLPHTTP(endpoint): Sends traces to OTLP collector via HTTP
//
// # OTLP and Start
//
// With OTLP providers, the tracer is not fully operational until Start(ctx) is called.
// You must call Start(ctx) before exporting traces; otherwise no traces will be sent
// and no error is returned at New.
//
// # HTTP Middleware
//
// Use the middleware for automatic request tracing:
//
//	tracer := tracing.MustNew(
//	    tracing.WithServiceName("my-api"),
//	    tracing.WithOTLP("localhost:4317"),
//	)
//	defer tracer.Shutdown(context.Background())
//
//	// Use Middleware for HTTP handlers (returns error; use MustMiddleware to panic)
//	handler, err := tracing.Middleware(tracer,
//	    tracing.WithExcludePaths("/health", "/metrics"),
//	    tracing.WithHeaders("X-Request-ID"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	http.ListenAndServe(":8080", handler(mux))
//
// # Custom Spans
//
// Create and manage spans using the provided methods:
//
//	ctx, span := tracer.StartSpan(ctx, "database-query")
//	defer tracer.FinishSpan(span, http.StatusOK)
//
//	tracer.SetSpanAttribute(span, "user.id", "123")
//	tracer.AddSpanEvent(span, "cache_hit",
//	    attribute.String("key", "user:123"),
//	)
//
// When you only have context (e.g. from middleware), use SetSpanAttributeFromContext
// and AddSpanEventFromContext; when you have the *Tracer and span, use
// tracer.SetSpanAttribute and tracer.AddSpanEvent. The two styles are equivalent;
// the context helpers are for use when the tracer is not in scope.
//
// # App integration
//
// When using the app package, enable tracing with
// app.WithObservability(app.WithTracing(...)). Request spans are created with
// the same behavior as the standalone middleware (W3C propagation, sampling,
// standard attributes). From handlers, use c.StartSpan and c.FinishSpan for
// child spans; use c.Tracer() only for advanced use (e.g. passing the tracer
// to another library).
//
// # Context Propagation
//
// Automatically propagate trace context across service boundaries:
//
//	ctx = tracer.ExtractTraceContext(ctx, req.Header)
//	tracer.InjectTraceContext(ctx, resp.Header)
//
// # Sampling
//
// Control which requests are traced using sampling:
//
//	tracer := tracing.MustNew(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithSampleRate(0.1), // Sample 10% of requests
//	)
//
// # Thread Safety
//
// All methods are thread-safe. The Tracer struct is immutable after creation,
// with read-only maps and slices ensuring safe concurrent access without locks.
// Span operations use OpenTelemetry's thread-safe primitives.
//
// # Global State
//
// By default, this package does NOT set the global OpenTelemetry tracer provider.
// Use WithGlobalTracerProvider() option if you want to register the tracer provider
// as the global default via otel.SetTracerProvider().
//
// This allows multiple tracing configurations to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global tracer provider.
//
// # Path Filtering (Middleware Option)
//
// Exclude specific paths from tracing via middleware options:
//
//	handler, err := tracing.Middleware(tracer,
//	    tracing.WithExcludePaths("/health", "/metrics", "/ready"),
//	    tracing.WithExcludePrefixes("/debug/"),
//	    tracing.WithExcludePatterns("^/internal/.*"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// ... use handler(mux)
//
// # Custom Tracer Provider
//
// For advanced use cases, provide your own OpenTelemetry tracer provider:
//
//	tracer := tracing.MustNew(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithTracerProvider(customProvider),
//	)
package tracing
