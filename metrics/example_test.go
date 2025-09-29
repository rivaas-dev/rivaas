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

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/metrics"
)

// ExampleNew demonstrates creating a new metrics configuration.
func ExampleNew() {
	config, err := metrics.New(
		metrics.WithServiceName("my-service"),
		metrics.WithServiceVersion("1.0.0"),
		metrics.WithProvider(metrics.PrometheusProvider),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer config.Shutdown(context.Background())

	fmt.Printf("Metrics enabled: %v\n", config.IsEnabled())
	// Output: Metrics enabled: true
}

// ExampleMustNew demonstrates creating metrics configuration that panics on error.
func ExampleMustNew() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.PrometheusProvider),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", config.ServiceName())
	// Output: Service: my-service
}

// ExampleRecordMetric demonstrates recording custom metrics.
func ExampleConfig_RecordMetric() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	config.RecordMetric(ctx, "processing_duration", 1.5,
		attribute.String("operation", "create_user"),
		attribute.String("status", "success"),
	)
}

// ExampleIncrementCounter demonstrates incrementing a counter.
func ExampleConfig_IncrementCounter() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	config.IncrementCounter(ctx, "requests_total",
		attribute.String("method", "GET"),
		attribute.String("status", "200"),
	)
}

// ExampleSetGauge demonstrates setting a gauge value.
func ExampleConfig_SetGauge() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	config.SetGauge(ctx, "active_connections", 42,
		attribute.String("server", "api-1"),
	)
}

// ExampleWithOTLPEndpoint demonstrates configuring OTLP exporter.
func ExampleWithOTLPEndpoint() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.OTLPProvider),
		metrics.WithOTLPEndpoint("localhost:4318"),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Provider: %s\n", config.GetProvider())
	// Output: Provider: otlp
}

// ExampleWithExcludePaths demonstrates excluding paths from metrics.
func ExampleWithExcludePaths() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.PrometheusProvider),
		metrics.WithExcludePaths("/health", "/metrics", "/ready"),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Metrics enabled: %v\n", config.IsEnabled())
	// Output: Metrics enabled: true
}

// ExampleWithHeaders demonstrates recording specific headers as attributes.
func ExampleWithHeaders() {
	config := metrics.MustNew(
		metrics.WithServiceName("my-service"),
		metrics.WithProvider(metrics.PrometheusProvider),
		metrics.WithHeaders("X-Request-ID", "X-User-ID"),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", config.ServiceName())
	// Output: Service: my-service
}
