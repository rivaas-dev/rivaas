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

package router

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

// Observability is unified through the ObservabilityRecorder interface (see observability.go).
//
// For request-level observability, use ObservabilityRecorder which combines:
//   - Metrics collection
//   - Distributed tracing
//   - Access logging
//   - Request-scoped logger creation
//
// For handler-level custom metrics and tracing, use ContextMetricsRecorder and
// ContextTracingRecorder which remain available through router.Context.

// ContextMetricsRecorder interface for context-level metrics recording.
// This interface provides methods that can be called from router.Context
// to record custom metrics.
type ContextMetricsRecorder interface {
	// RecordMetric records a custom histogram metric with the given name and value.
	RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// IncrementCounter increments a custom counter metric with the given name.
	IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue)

	// SetGauge sets a custom gauge metric with the given name and value.
	SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)
}

// ContextTracingRecorder interface for context-level tracing recording.
// This interface provides methods that can be called from router.Context
// to interact with tracing.
type ContextTracingRecorder interface {
	// TraceID returns the current trace ID from the active span.
	// Returns an empty string if tracing is not active.
	TraceID() string

	// SpanID returns the current span ID from the active span.
	// Returns an empty string if tracing is not active.
	SpanID() string

	// SetSpanAttribute adds an attribute to the current span.
	// This is a no-op if tracing is not active.
	SetSpanAttribute(key string, value any)

	// AddSpanEvent adds an event to the current span with optional attributes.
	// This is a no-op if tracing is not active.
	AddSpanEvent(name string, attrs ...attribute.KeyValue)

	// TraceContext returns the OpenTelemetry trace context.
	// This can be used for manual span creation or context propagation.
	// If tracing is not enabled, it returns the request context for proper cancellation support.
	TraceContext() context.Context
}

// RequestMetrics interface for request-level metrics tracking.
// This interface abstracts the metrics data structure used during request processing.
// It is implemented by the metrics package and returned by StartRequest().
// The interface is intentionally minimal to allow for flexible implementations.
type RequestMetrics interface {
	// GetStartTime returns the request start time.
	GetStartTime() any

	// GetRequestSize returns the request size in bytes.
	GetRequestSize() int64

	// GetAttributes returns the request attributes.
	GetAttributes() []attribute.KeyValue
}
