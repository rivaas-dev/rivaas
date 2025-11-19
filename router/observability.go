package router

import (
	"context"
	"log/slog"
	"net/http"
)

// ObservabilityRecorder provides unified observability lifecycle hooks for HTTP requests.
// Implementations typically combine metrics collection, distributed tracing, and access logging.
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
//  4. Router matches route and calls BuildRequestLogger(ctx, req, routePattern)
//     - Called AFTER routing completes (so routePattern is known)
//     - Called EVEN FOR excluded requests (enables business logging)
//  5. Handler executes
//  6. Router calls OnRequestEnd(ctx, state, writer, routePattern) ONLY IF state != nil
//     - Implementation extracts status/size from writer (via ResponseInfo interface)
//     - Records metrics, finishes traces, logs access entry
//
// Exclusion semantics:
//   - state=nil means: no wrapping, no OnRequestEnd, no access logs/metrics/traces
//   - state=nil does NOT affect: context enrichment, BuildRequestLogger
//   - Rationale: handlers on excluded paths can still do business logging and
//     make downstream calls with proper trace propagation
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
	// but will still use the enriched context and call BuildRequestLogger.
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
	// routePattern is the matched route template (e.g., "/users/{id}"), or a sentinel
	// value like "_not_found" or "_unmatched" if no route matched.
	// Implementations should use routePattern (not raw path) for metrics/traces
	// to prevent cardinality explosion.
	//
	// state is the opaque token returned by OnRequestStart.
	OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string)

	// BuildRequestLogger creates a request-scoped logger enriched with HTTP metadata.
	// Called after routing completes (so routePattern is available).
	//
	// This is called EVEN FOR excluded paths (state=nil), allowing handlers to perform
	// business logging. Access logging is separate (happens in OnRequestEnd).
	//
	// Returns a non-nil logger. If logging is disabled, returns a no-op logger.
	// The returned logger is injected into Context for handlers to use.
	BuildRequestLogger(ctx context.Context, req *http.Request, routePattern string) *slog.Logger
}

// ResponseInfo is implemented by response writers that track response metadata.
// Implementations of ObservabilityRecorder should have their wrapped ResponseWriter
// implement this interface so OnRequestEnd can extract status and size.
type ResponseInfo interface {
	StatusCode() int
	Size() int64
}
