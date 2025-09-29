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

package semconv_test

import (
	"fmt"
	"log/slog"

	"rivaas.dev/telemetry/semconv"
)

// ExampleServiceName demonstrates how to set service metadata during logger initialization.
func ExampleServiceName() {
	// Service metadata is typically set once during logger initialization
	logger := slog.Default().With(
		semconv.ServiceName, "my-service",
		semconv.ServiceVersion, "1.0.0",
		semconv.ServiceNamespace, "api",
		semconv.DeploymentEnviron, "production",
	)

	logger.Info("service started")
	fmt.Println("Service metadata configured")
	// Output: Service metadata configured
}

// ExampleHTTPMethod demonstrates how to log HTTP request attributes.
func ExampleHTTPMethod() {
	logger := slog.Default()

	// Log HTTP request attributes
	logger.Info("request processed",
		semconv.HTTPMethod, "GET",
		semconv.HTTPStatusCode, 200,
		semconv.HTTPRoute, "/users/:id",
		semconv.HTTPTarget, "/users/123",
		semconv.HTTPScheme, "https",
	)

	fmt.Println("HTTP attributes logged")
	// Output: HTTP attributes logged
}

// ExampleNetworkPeerIP demonstrates how to log network attributes.
func ExampleNetworkPeerIP() {
	logger := slog.Default()

	// Log network attributes
	logger.Info("request received",
		semconv.NetworkPeerIP, "192.168.1.100",
		semconv.NetworkClientIP, "10.0.0.1",
	)

	fmt.Println("Network attributes logged")
	// Output: Network attributes logged
}

// ExampleTraceID demonstrates how to add trace correlation to logs.
func ExampleTraceID() {
	logger := slog.Default()

	// Add trace correlation (typically extracted from OpenTelemetry context)
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"

	logger = logger.With(
		semconv.TraceID, traceID,
		semconv.SpanID, spanID,
	)

	logger.Info("operation completed")
	fmt.Println("Trace correlation added")
	// Output: Trace correlation added
}

// ExampleRequestID demonstrates how to use request ID for request correlation.
func ExampleRequestID() {
	logger := slog.Default()

	// Add request ID (typically extracted from X-Request-ID header)
	requestID := "req-12345"

	logger = logger.With(
		semconv.RequestID, requestID,
	)

	logger.Info("request started")
	fmt.Println("Request ID added")
	// Output: Request ID added
}

// ExampleRequestID_complete demonstrates a complete example of logging a request with all attributes.
func ExampleRequestID_complete() {
	// Initialize logger with service metadata
	logger := slog.Default().With(
		semconv.ServiceName, "api-service",
		semconv.ServiceVersion, "2.1.0",
		semconv.DeploymentEnviron, "production",
	)

	// Simulate request context
	requestID := "req-abc123"
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"

	// Create request-scoped logger with correlation IDs
	requestLogger := logger.With(
		semconv.RequestID, requestID,
		semconv.TraceID, traceID,
		semconv.SpanID, spanID,
	)

	// Log request processing
	requestLogger.Info("request received",
		semconv.HTTPMethod, "POST",
		semconv.HTTPRoute, "/api/users",
		semconv.HTTPTarget, "/api/users",
		semconv.HTTPScheme, "https",
		semconv.NetworkClientIP, "192.168.1.50",
	)

	// Log response
	requestLogger.Info("request completed",
		semconv.HTTPStatusCode, 201,
	)

	fmt.Println("Complete request logged")
	// Output: Complete request logged
}

// ExampleTraceID_withContext demonstrates how to extract trace information from context.
func ExampleTraceID_withContext() {
	// This example shows the pattern for extracting trace information from OpenTelemetry context.
	// In a real application, you would use go.opentelemetry.io/otel/trace to extract span context.

	logger := slog.Default()

	// Simulate extracting trace context (in real code, use trace.SpanFromContext(ctx))
	// ctx := context.Background() // In real code, extract span from this context

	// Pattern for extracting trace information:
	// if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
	//     sc := span.SpanContext()
	//     logger = logger.With(
	//         semconv.TraceID, sc.TraceID().String(),
	//         semconv.SpanID, sc.SpanID().String(),
	//     )
	// }

	// For this example, we'll use mock values
	logger = logger.With(
		semconv.TraceID, "4bf92f3577b34da6a3ce929d0e0e4736",
		semconv.SpanID, "00f067aa0ba902b7",
	)

	logger.Info("operation completed")
	fmt.Println("Trace correlation from context")
	// Output: Trace correlation from context
}
