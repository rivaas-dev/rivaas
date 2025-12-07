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

package tracing_test

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/tracing"
)

// ExampleNew demonstrates creating a new tracing configuration.
func ExampleNew() {
	tracer, err := tracing.New(
		tracing.WithServiceName("my-service"),
		tracing.WithServiceVersion("1.0.0"),
		tracing.WithStdout(),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer tracer.Shutdown(context.Background())

	fmt.Printf("Tracing enabled: %v\n", tracer.IsEnabled())
	// Output: Tracing enabled: true
}

// ExampleMustNew demonstrates creating tracing configuration that panics on error.
func ExampleMustNew() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", tracer.ServiceName())
	// Output: Service: my-service
}

// ExampleStartSpan demonstrates creating and managing spans.
func ExampleTracer_StartSpan() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "database-query")
	defer tracer.FinishSpan(span, http.StatusOK)

	tracer.SetSpanAttribute(span, "db.query", "SELECT * FROM users")
	tracer.SetSpanAttribute(span, "db.rows", 10)
	// Output:
}

// ExampleTracer_AddSpanEvent demonstrates adding events to spans.
func ExampleTracer_AddSpanEvent() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "cache-operation")
	defer tracer.FinishSpan(span, http.StatusOK)

	tracer.AddSpanEvent(span, "cache_hit",
		attribute.String("key", "user:123"),
		attribute.Int("ttl", 3600),
	)

	_ = ctx // use ctx
	// Output:
}

// ExampleExtractTraceContext demonstrates extracting trace context from headers.
func ExampleTracer_ExtractTraceContext() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	ctx := context.Background()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	ctx = tracer.ExtractTraceContext(ctx, req.Header)
	ctx, span := tracer.StartSpan(ctx, "process-request")
	defer tracer.FinishSpan(span, http.StatusOK)
	// Output:
}

// ExampleWithSampleRate demonstrates configuring sampling rate.
func ExampleWithSampleRate() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
		tracing.WithSampleRate(0.1), // Sample 10% of requests
	)
	defer tracer.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", tracer.ServiceName())
	// Output: Service: my-service
}

// ExampleMustMiddleware demonstrates using middleware with path exclusion.
func ExampleMustMiddleware() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	// Create handler with middleware options
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use MustMiddleware for convenience (panics on invalid options)
	handler := tracing.MustMiddleware(tracer,
		tracing.WithExcludePaths("/health", "/metrics", "/ready"),
		tracing.WithHeaders("X-Request-ID", "X-Correlation-ID"),
	)(mux)

	// Use handler...
	_ = handler
	// Output:
}

// ExampleMiddleware demonstrates using middleware with error handling.
func ExampleMiddleware() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer tracer.Shutdown(context.Background())

	// Create handler with middleware options
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use Middleware with error handling
	middleware, err := tracing.Middleware(tracer,
		tracing.WithExcludePaths("/health", "/metrics", "/ready"),
		tracing.WithHeaders("X-Request-ID", "X-Correlation-ID"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	handler := middleware(mux)
	_ = handler
	// Output:
}

// ExampleWithOTLP demonstrates configuring OTLP provider.
func ExampleWithOTLP() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithOTLP("localhost:4317", tracing.OTLPInsecure()),
	)
	defer tracer.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", tracer.ServiceName())
	// Output: Service: my-service
}

// ExampleWithOTLPHTTP demonstrates configuring OTLP HTTP provider.
func ExampleWithOTLPHTTP() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithOTLPHTTP("http://localhost:4318"),
	)
	defer tracer.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", tracer.ServiceName())
	// Output: Service: my-service
}
