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

// Package metrics provides OpenTelemetry-based metrics collection for Go applications.
// It supports multiple exporters (Prometheus, OTLP, stdout) and integrates with the Rivaas router.
//
// # Basic Usage
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithServiceName("my-service"),
//	)
//
//	// Start metrics server (required for Prometheus, OTLP)
//	if err := recorder.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer recorder.Shutdown(context.Background())
//
//	// Record custom metrics
//	_ = recorder.IncrementCounter(ctx, "requests_total",
//	    attribute.String("method", "GET"),
//	    attribute.String("status", "200"),
//	)
//
// # Thread Safety
//
// All [Recorder] methods are safe for concurrent use. Custom metrics are limited
// (default 1000) to prevent unbounded metric creation.
//
// # Global State
//
// By default, this package does NOT set the global OpenTelemetry meter provider.
// Use [WithGlobalMeterProvider] if you want global registration.
// This allows multiple [Recorder] instances to coexist in the same process.
//
// # Providers
//
// Three providers are supported:
//   - [PrometheusProvider] (default): Exposes metrics via HTTP endpoint
//   - [OTLPProvider]: Sends metrics to OTLP collector
//   - [StdoutProvider]: Prints metrics to stdout (for development/testing)
//
// Provider initialization timing:
//   - Prometheus: Initialized in New(), server starts in Start()
//   - OTLP: Deferred to Start() for lifecycle context
//   - Stdout: Initialized in New(), works without Start()
//
// For OTLP provider, you must call Start(ctx) before recording metrics.
//
// # Custom Metrics
//
// Record custom metrics using the provided methods. All methods return errors
// for explicit error handling:
//
//	// Handle errors explicitly
//	if err := recorder.RecordHistogram(ctx, "processing_duration", 1.5,
//	    attribute.String("operation", "create_user")); err != nil {
//	    log.Printf("metrics error: %v", err)
//	}
//
//	// Or ignore errors with _ (fire-and-forget)
//	_ = recorder.IncrementCounter(ctx, "requests_total",
//	    attribute.String("status", "success"))
//	_ = recorder.SetGauge(ctx, "active_connections", 42)
//
// See [Recorder.RecordHistogram], [Recorder.IncrementCounter], and [Recorder.SetGauge]
// for custom metric recording.
//
// # Lifecycle Management
//
// For proper initialization and shutdown:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	recorder, _ := metrics.New(metrics.WithOTLP("http://localhost:4318"))
//	recorder.Start(ctx)  // Required for OTLP, starts server for Prometheus
//	defer recorder.Shutdown(context.Background())
//
// Use [Recorder.ForceFlush] to immediately export pending metrics before
// critical operations or deployments (useful for push-based providers).
//
// # Security
//
// Sensitive headers (Authorization, Cookie, X-API-Key, etc.) are automatically
// filtered out when using [WithHeaders] to prevent accidental credential exposure.
package metrics
