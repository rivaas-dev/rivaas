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
	config, err := tracing.New(
		tracing.WithServiceName("my-service"),
		tracing.WithServiceVersion("1.0.0"),
		tracing.WithProvider(tracing.StdoutProvider),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer config.Shutdown(context.Background())

	fmt.Printf("Tracing enabled: %v\n", config.IsEnabled())
	// Output: Tracing enabled: true
}

// ExampleMustNew demonstrates creating tracing configuration that panics on error.
func ExampleMustNew() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", config.ServiceName())
	// Output: Service: my-service
}

// ExampleStartSpan demonstrates creating and managing spans.
func ExampleConfig_StartSpan() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	ctx, span := config.StartSpan(ctx, "database-query")
	defer config.FinishSpan(span, http.StatusOK)

	config.SetSpanAttribute(span, "db.query", "SELECT * FROM users")
	config.SetSpanAttribute(span, "db.rows", 10)
}

// ExampleAddSpanEvent demonstrates adding events to spans.
func ExampleConfig_AddSpanEvent() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	ctx, span := config.StartSpan(ctx, "cache-operation")
	defer config.FinishSpan(span, http.StatusOK)

	config.AddSpanEvent(span, "cache_hit",
		attribute.String("key", "user:123"),
		attribute.Int("ttl", 3600),
	)
}

// ExampleExtractTraceContext demonstrates extracting trace context from headers.
func ExampleConfig_ExtractTraceContext() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	ctx = config.ExtractTraceContext(ctx, req.Header)
	ctx, span := config.StartSpan(ctx, "process-request")
	defer config.FinishSpan(span, http.StatusOK)
}

// ExampleWithSampleRate demonstrates configuring sampling rate.
func ExampleWithSampleRate() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
		tracing.WithSampleRate(0.1), // Sample 10% of requests
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", config.ServiceName())
	// Output: Service: my-service
}

// ExampleWithExcludePaths demonstrates excluding paths from tracing.
func ExampleWithExcludePaths() {
	config := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithProvider(tracing.StdoutProvider),
		tracing.WithExcludePaths("/health", "/metrics", "/ready"),
	)
	defer config.Shutdown(context.Background())

	fmt.Printf("Tracing enabled: %v\n", config.IsEnabled())
	// Output: Tracing enabled: true
}

// ExampleNewProduction demonstrates production configuration.
func ExampleNewProduction() {
	config, err := tracing.NewProduction("my-api", "v1.2.3")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s, Version: %s\n", config.ServiceName(), config.ServiceVersion())
	// Output: Service: my-api, Version: v1.2.3
}

// ExampleNewDevelopment demonstrates development configuration.
func ExampleNewDevelopment() {
	config, err := tracing.NewDevelopment("my-api", "dev")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer config.Shutdown(context.Background())

	fmt.Printf("Service: %s\n", config.ServiceName())
	// Output: Service: my-api
}
