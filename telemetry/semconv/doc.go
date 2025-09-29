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

// Package semconv provides semantic conventions for telemetry data.
//
// The semconv package defines constants for consistent field names across logs,
// metrics, and traces. These constants follow OpenTelemetry semantic conventions
// where applicable, ensuring interoperability with OpenTelemetry-compatible tools
// and observability platforms.
//
// # Key Features
//
//   - HTTP attributes: method, route, status code, scheme, target
//   - Network attributes: client IP, peer IP
//   - Service metadata: name, version, namespace, environment
//   - Trace correlation: trace ID, span ID
//   - Request identification: request ID
//
// # Usage
//
// Use these constants as keys when logging structured data:
//
//	package main
//
//	import (
//	    "log/slog"
//	    "rivaas.dev/telemetry/semconv"
//	)
//
//	func main() {
//	    logger := slog.Default()
//	    logger.Info("request processed",
//	        semconv.HTTPMethod, "GET",
//	        semconv.HTTPStatusCode, 200,
//	        semconv.HTTPRoute, "/users/:id",
//	        semconv.HTTPTarget, "/users/123",
//	    )
//	}
//
// Service metadata is typically set once during logger initialization:
//
//	package main
//
//	import (
//	    "log/slog"
//	    "rivaas.dev/telemetry/semconv"
//	)
//
//	func initLogger() *slog.Logger {
//	    return slog.Default().With(
//	        semconv.ServiceName, "my-service",
//	        semconv.ServiceVersion, "1.0.0",
//	        semconv.DeploymentEnviron, "production",
//	    )
//	}
//
// Trace correlation is automatically added when using OpenTelemetry tracing:
//
//	package main
//
//	import (
//	    "context"
//	    "log/slog"
//	    "rivaas.dev/telemetry/semconv"
//	    "go.opentelemetry.io/otel/trace"
//	)
//
//	func logWithTrace(ctx context.Context, logger *slog.Logger) {
//	    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
//	        sc := span.SpanContext()
//	        logger = logger.With(
//	            semconv.TraceID, sc.TraceID().String(),
//	            semconv.SpanID, sc.SpanID().String(),
//	        )
//	    }
//	    logger.Info("operation completed")
//	}
//
// # Reference
//
// OpenTelemetry Semantic Conventions: https://opentelemetry.io/docs/specs/semconv/
package semconv
