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

package metrics_test

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/metrics"
)

// ExampleNew demonstrates creating a new metrics Recorder.
func ExampleNew() {
	recorder, err := metrics.New(
		metrics.WithPrometheus(":9090", "/metrics"),
		metrics.WithServiceName("my-service"),
		metrics.WithServiceVersion("1.0.0"),
		metrics.WithServerDisabled(),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer recorder.Shutdown(context.Background())

	fmt.Printf("Metrics enabled: %v\n", recorder.IsEnabled())
	// Output: Metrics enabled: true
}

// ExampleMustNew demonstrates creating Recorder that panics on error.
func ExampleMustNew() {
	recorder := metrics.MustNew(
		metrics.WithPrometheus(":9090", "/metrics"),
		metrics.WithServiceName("my-service"),
		metrics.WithServerDisabled(),
	)
	defer recorder.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", recorder.ServiceName())
	// Output: Service: my-service
}

// ExampleRecorder_RecordHistogram demonstrates recording custom histogram metrics.
func ExampleRecorder_RecordHistogram() {
	recorder := metrics.MustNew(
		metrics.WithStdout(),
		metrics.WithServiceName("my-service"),
	)
	defer recorder.Shutdown(context.Background())

	ctx := context.Background()

	// Record histogram with error handling
	if err := recorder.RecordHistogram(ctx, "processing_duration", 1.5,
		attribute.String("operation", "create_user"),
		attribute.String("status", "success"),
	); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Or fire-and-forget (ignore errors)
	_ = recorder.RecordHistogram(ctx, "processing_duration", 2.3)
}

//nolint:testableexamples // Output is non-deterministic (contains timestamps)

// ExampleRecorder_IncrementCounter demonstrates incrementing a counter.
func ExampleRecorder_IncrementCounter() {
	recorder := metrics.MustNew(
		metrics.WithStdout(),
		metrics.WithServiceName("my-service"),
	)
	defer recorder.Shutdown(context.Background())

	ctx := context.Background()

	// Increment counter with error handling
	if err := recorder.IncrementCounter(ctx, "requests_total",
		attribute.String("method", "GET"),
		attribute.String("status", "200"),
	); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Or fire-and-forget (ignore errors)
	_ = recorder.IncrementCounter(ctx, "events_total")
}

//nolint:testableexamples // Output is non-deterministic (contains timestamps)

// ExampleRecorder_SetGauge demonstrates setting a gauge value.
func ExampleRecorder_SetGauge() {
	recorder := metrics.MustNew(
		metrics.WithStdout(),
		metrics.WithServiceName("my-service"),
	)
	defer recorder.Shutdown(context.Background())

	ctx := context.Background()

	// Set gauge with error handling
	if err := recorder.SetGauge(ctx, "active_connections", 42,
		attribute.String("server", "api-1"),
	); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Or fire-and-forget (ignore errors)
	_ = recorder.SetGauge(ctx, "cache_size", 1024)
}

//nolint:testableexamples // Output is non-deterministic (contains timestamps)

// ExampleWithOTLP demonstrates configuring OTLP exporter.
func ExampleWithOTLP() {
	recorder := metrics.MustNew(
		metrics.WithOTLP("http://localhost:4318"),
		metrics.WithServiceName("my-service"),
	)
	defer recorder.Shutdown(context.Background())

	fmt.Printf("Provider: %s\n", recorder.Provider())
	// Output: Provider: otlp
}

// ExampleMiddleware_WithExcludePaths demonstrates excluding paths from metrics via middleware.
func ExampleMiddleware_withExcludePaths() {
	recorder := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithPrometheus(":9090", "/metrics"),
		metrics.WithServerDisabled(),
	)
	defer recorder.Shutdown(context.Background())

	// Path exclusion is now configured on the middleware
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Apply middleware with path exclusions
	_ = metrics.Middleware(recorder,
		metrics.WithExcludePaths("/health", "/metrics", "/ready"),
	)(mux)

	fmt.Printf("Metrics enabled: %v\n", recorder.IsEnabled())
	// Output: Metrics enabled: true
}

// ExampleMiddleware_WithHeaders demonstrates recording specific headers as attributes.
// Note: Sensitive headers like Authorization and Cookie are automatically filtered.
func ExampleMiddleware_withHeaders() {
	recorder := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithPrometheus(":9090", "/metrics"),
		metrics.WithServerDisabled(),
	)
	defer recorder.Shutdown(context.Background())

	// Header recording is now configured on the middleware
	mux := http.NewServeMux()
	_ = metrics.Middleware(recorder,
		metrics.WithHeaders("X-Request-ID", "X-User-ID"),
	)(mux)

	fmt.Printf("Service: %s\n", recorder.ServiceName())
	// Output: Service: my-service
}
