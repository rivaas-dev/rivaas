// Package semconv provides semantic conventions for telemetry data.
// These constants ensure consistent field names across logs, metrics, and traces,
// following OpenTelemetry semantic conventions where applicable.
//
// Reference: https://opentelemetry.io/docs/specs/semconv/
package semconv

// Service metadata (set in base logger, not per-request)
const (
	ServiceName       = "service.name"
	ServiceVersion    = "service.version"
	ServiceNamespace  = "service.namespace"
	DeploymentEnviron = "deployment.environment"
)

// HTTP attributes (OpenTelemetry semantic conventions)
const (
	HTTPMethod     = "http.method"      // GET, POST, etc.
	HTTPRoute      = "http.route"       // Route template: /orders/:id
	HTTPTarget     = "http.target"      // Actual path: /orders/42
	HTTPStatusCode = "http.status_code" // 200, 404, etc.
	HTTPScheme     = "http.scheme"      // http, https
)

// Network attributes
const (
	NetworkPeerIP   = "network.peer.ip"   // Direct socket IP
	NetworkClientIP = "network.client.ip" // Real client IP (proxy-aware)
)

// Trace correlation (OpenTelemetry)
const (
	TraceID = "trace_id"
	SpanID  = "span_id"
)

// Request attributes
const (
	RequestID = "req.id" // X-Request-ID or similar
)
