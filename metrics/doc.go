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

// Package metrics provides comprehensive OpenTelemetry-based metrics collection
// for Go applications. It supports multiple exporters (Prometheus, OTLP, stdout)
// and integrates seamlessly with the Rivaas router.
//
// # Basic Usage
//
//	config := metrics.MustNew(
//	    metrics.WithServiceName("my-service"),
//	    metrics.WithProvider(metrics.PrometheusProvider),
//	)
//	defer config.Shutdown(context.Background())
//
//	// Record custom metrics
//	ctx := context.Background()
//	config.IncrementCounter(ctx, "requests_total",
//	    attribute.String("method", "GET"),
//	    attribute.String("status", "200"),
//	)
//
// # Thread Safety
//
// All methods are safe for concurrent use. Custom metrics are limited
// (default 1000) to prevent unbounded metric creation.
//
// # Global State Warning
//
// This package sets the global OpenTelemetry meter provider via otel.SetMeterProvider().
// Only one metrics configuration should be active per process. Creating multiple
// configurations will cause them to overwrite each other's global meter provider.
//
// # Providers
//
// Three providers are supported:
//   - PrometheusProvider (default): Exposes metrics via HTTP endpoint
//   - OTLPProvider: Sends metrics to OTLP collector
//   - StdoutProvider: Prints metrics to stdout (for development/testing)
//
// # Custom Metrics
//
// Record custom metrics using the provided methods:
//
//	config.RecordMetric(ctx, "processing_duration", 1.5,
//	    attribute.String("operation", "create_user"))
//	config.IncrementCounter(ctx, "requests_total",
//	    attribute.String("status", "success"))
//	config.SetGauge(ctx, "active_connections", 42)
package metrics
