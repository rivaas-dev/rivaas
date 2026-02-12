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
	"net/http"
)

// ObservabilityRecorder provides unified observability lifecycle hooks for HTTP requests.
// Implementations typically combine metrics collection, distributed tracing, and access logging.
//
// The Three Pillars of Observability:
//   - Metrics: Record quantitative measurements (request counts, durations, status codes)
//     → Implement in OnRequestEnd by extracting ResponseInfo and recording to metrics system
//   - Tracing: Track request flow and create distributed trace spans
//     → Implement in OnRequestStart (create span) and OnRequestEnd (finish span)
//   - Logging: Provide structured access logs
//     → Implement in OnRequestEnd for canonical access log lines
//
// These three pillars work together to provide complete visibility into system behavior.
// See observability_example_test.go for a reference implementation.
//
// Lifecycle:
//  1. Router calls OnRequestStart(ctx, req) → (enrichedCtx, state)
//     - Returns enriched context (e.g., with trace span propagation)
//     - Returns opaque state token (nil if request should be excluded)
//  2. Router ALWAYS calls req.WithContext(enrichedCtx)
//     - Context enrichment applies even to excluded requests
//     - Example: trace propagation for downstream calls works even if metrics are excluded
//  3. Router wraps ResponseWriter ONLY IF state != nil
//     - Excluded requests (state=nil) skip wrapping and OnRequestEnd
//  4. Handler executes
//  5. Router calls OnRequestEnd(ctx, state, writer, routePattern) ONLY IF state != nil
//     - Implementation extracts status/size from writer (via ResponseInfo interface)
//     - Records metrics, finishes traces, logs access entry
//
// Exclusion semantics:
//   - state=nil means: no wrapping, no OnRequestEnd, no access logs/metrics/traces
//   - state=nil does NOT affect: context enrichment
//   - Rationale: handlers on excluded paths can still make downstream calls
//     with proper trace propagation
//
// Thread safety: All methods must be safe for concurrent use.
type ObservabilityRecorder interface {
	// OnRequestStart is called before routing begins.
	// Returns an enriched context (e.g., with trace span) and an opaque state token.
	//
	// The enriched context is ALWAYS used for the request, regardless of state value.
	// This ensures context enrichment (trace propagation, etc.) works even for excluded paths.
	//
	// If the request should be excluded from observability (e.g., /health, /metrics),
	// return (enrichedCtx, nil). Router will skip WrapResponseWriter and OnRequestEnd,
	// but will still use the enriched context.
	//
	// The state token is passed to WrapResponseWriter and OnRequestEnd.
	// Router treats state as completely opaque.
	OnRequestStart(ctx context.Context, req *http.Request) (context.Context, any)

	// WrapResponseWriter wraps http.ResponseWriter to capture response metadata.
	// Returns the wrapped writer if observability is enabled (state != nil),
	// or the original writer if excluded (state == nil).
	//
	// The wrapped writer should implement ResponseInfo to expose status code and size.
	// If state is nil, this must return the original writer unchanged.
	WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter

	// OnRequestEnd is called after request handling completes.
	// Only called if state != nil (i.e., request is not excluded).
	//
	// The writer is the final ResponseWriter (potentially wrapped by WrapResponseWriter).
	// Implementation should type-assert writer to ResponseInfo to extract status/size.
	//
	// routePattern is the matched route pattern (e.g., "/users/{id}"), or a sentinel
	// value like "_not_found" or "_unmatched" if no route matched.
	// Implementations should use routePattern (not raw path) for metrics/traces
	// to prevent cardinality explosion.
	//
	// state is the opaque token returned by OnRequestStart.
	OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string)
}

// ResponseInfo is implemented by response writers that track response metadata.
// Implementations of ObservabilityRecorder should have their wrapped ResponseWriter
// implement this interface so OnRequestEnd can extract status and size.
type ResponseInfo interface {
	StatusCode() int
	Size() int64
}
