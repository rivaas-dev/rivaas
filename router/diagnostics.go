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

// DiagnosticEvent represents a router diagnostic or anomaly.
// These are informational events that may indicate configuration issues
// or security concerns.
//
// Diagnostic events are optional - the router functions correctly whether
// they are collected or not. They provide visibility into edge cases and
// potential issues for observability systems.
type DiagnosticEvent struct {
	Kind    DiagnosticKind
	Message string
	Fields  map[string]any // Structured context
}

// DiagnosticKind categorizes diagnostic events.
type DiagnosticKind string

const (
	// Security-related diagnostics
	DiagXFFSuspicious   DiagnosticKind = "xff_suspicious_chain"
	DiagHeaderInjection DiagnosticKind = "header_injection_blocked"
	DiagInvalidProto    DiagnosticKind = "invalid_x_forwarded_proto"

	// Configuration diagnostics
	DiagHighParamCount  DiagnosticKind = "route_param_count_high"
	DiagH2CEnabled      DiagnosticKind = "h2c_enabled"
	DiagRouteRegistered DiagnosticKind = "route_registered"
)

// DiagnosticHandler receives diagnostic events from the router.
// Implementations may log, emit metrics, trace events, or ignore them.
//
// This interface is optional - if not provided, diagnostics are silently dropped.
// The router's behavior is unchanged whether diagnostics are collected or not.
//
// Example with logging:
//
//	import "log/slog"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    slog.Warn(e.Message, "kind", e.Kind, "fields", e.Fields)
//	})
//	r := router.MustNew(router.WithDiagnostics(handler))
//
// Example with metrics:
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    metrics.Increment("router.diagnostics", "kind", string(e.Kind))
//	})
//
// Example with OpenTelemetry:
//
//	import "go.opentelemetry.io/otel/attribute"
//	import "go.opentelemetry.io/otel/trace"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    span := trace.SpanFromContext(ctx)
//	    if span.IsRecording() {
//	        attrs := []attribute.KeyValue{
//	            attribute.String("diagnostic.kind", string(e.Kind)),
//	        }
//	        for k, v := range e.Fields {
//	            attrs = append(attrs, attribute.String(k, fmt.Sprint(v)))
//	        }
//	        span.AddEvent(e.Message, trace.WithAttributes(attrs...))
//	    }
//	})
type DiagnosticHandler interface {
	OnDiagnostic(DiagnosticEvent)
}

// DiagnosticHandlerFunc is a function adapter for DiagnosticHandler.
type DiagnosticHandlerFunc func(DiagnosticEvent)

func (f DiagnosticHandlerFunc) OnDiagnostic(e DiagnosticEvent) {
	f(e)
}
