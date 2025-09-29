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
//	config, err := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithServiceVersion("v1.0.0"),
//	    tracing.WithProvider(tracing.StdoutProvider),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer config.Shutdown(context.Background())
//
// # Providers
//
// Three providers are supported:
//
//   - NoopProvider (default): No traces exported (safe default)
//   - StdoutProvider: Prints traces to stdout (for development/testing)
//   - OTLPProvider: Sends traces to OTLP collector (for production)
//
// # Custom Spans
//
// Create and manage spans using the provided methods:
//
//	ctx, span := config.StartSpan(ctx, "database-query")
//	defer config.FinishSpan(span, http.StatusOK)
//
//	config.SetSpanAttribute(span, "user.id", "123")
//	config.AddSpanEvent(span, "cache_hit",
//	    attribute.String("key", "user:123"),
//	)
//
// # Context Propagation
//
// Automatically propagate trace context across service boundaries:
//
//	ctx = config.ExtractTraceContext(ctx, req.Header)
//	config.InjectTraceContext(ctx, resp.Header)
//
// # Sampling
//
// Control which requests are traced using sampling:
//
//	config := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithSampleRate(0.1), // Sample 10% of requests
//	)
//
// # Thread Safety
//
// All methods are thread-safe. The Config struct is immutable after creation,
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
// # Production and Development Helpers
//
// Pre-configured setups for common scenarios:
//
//	// Production configuration: OTLP with conservative sampling
//	config, err := tracing.NewProduction("my-service", "v1.2.3")
//
//	// Development configuration: Stdout with full sampling
//	config, err := tracing.NewDevelopment("my-service", "dev")
//
// # Path Filtering
//
// Exclude specific paths from tracing:
//
//	config := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithExcludePaths("/health", "/metrics", "/ready"),
//	)
//
// # Custom Tracer Provider
//
// For advanced use cases, provide your own OpenTelemetry tracer provider:
//
//	config := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithTracerProvider(customProvider),
//	)
package tracing
