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

package semconv

// Service metadata constants.
//
// These constants represent service-level attributes that are typically set
// once during logger initialization, not per-request. They identify the service
// instance and deployment context.
const (
	// ServiceName identifies the service that generated the telemetry data.
	// It should be set to a stable service name that identifies the logical service.
	ServiceName = "service.name"

	// ServiceVersion identifies the version of the service that generated the telemetry data.
	// It should be set to the version string of the service instance.
	ServiceVersion = "service.version"

	// ServiceNamespace identifies the namespace of the service that generated the telemetry data.
	// It represents the namespace or logical grouping of services.
	ServiceNamespace = "service.namespace"

	// DeploymentEnviron identifies the environment where the service is deployed.
	// Common values include "production", "staging", "development", "testing".
	DeploymentEnviron = "deployment.environment"
)

// HTTP attribute constants.
//
// These constants represent HTTP request and response attributes following
// OpenTelemetry semantic conventions. They are typically added to log entries
// for HTTP request handling.
const (
	// HTTPMethod stores the HTTP request method.
	// Common values include "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS".
	HTTPMethod = "http.method"

	// HTTPRoute stores the route pattern matched by the router.
	// It represents the route template, not the actual path (e.g., "/orders/:id").
	HTTPRoute = "http.route"

	// HTTPTarget stores the actual path requested.
	// It represents the full path from the request URL (e.g., "/orders/42").
	HTTPTarget = "http.target"

	// HTTPStatusCode stores the HTTP response status code.
	// It represents the numeric status code returned to the client (e.g., 200, 404, 500).
	HTTPStatusCode = "http.status_code"

	// HTTPScheme stores the URI scheme used for the request.
	// Common values include "http" and "https".
	HTTPScheme = "http.scheme"
)

// Network attribute constants.
//
// These constants represent network-level attributes for identifying the source
// of network connections. They help distinguish between direct peer connections
// and the actual client IP when proxies or load balancers are involved.
const (
	// NetworkPeerIP stores the IP address of the direct peer connection.
	// It represents the socket-level IP address of the immediate connection peer,
	// which may be a proxy or load balancer rather than the actual client.
	NetworkPeerIP = "network.peer.ip"

	// NetworkClientIP stores the real client IP address.
	// It represents the actual client IP address, accounting for proxies and
	// load balancers. This is typically extracted from headers like X-Forwarded-For
	// or X-Real-IP when trusted proxies are configured.
	NetworkClientIP = "network.client.ip"
)

// Trace correlation constants.
//
// These constants represent trace and span identifiers used for correlating
// logs with distributed traces. They follow OpenTelemetry conventions for
// trace correlation.
const (
	// TraceID stores the unique identifier for a distributed trace.
	// It identifies the entire trace across all services and spans.
	TraceID = "trace_id"

	// SpanID stores the unique identifier for a span within a trace.
	// It identifies a specific span within the distributed trace.
	SpanID = "span_id"
)

// Request attribute constants.
//
// These constants represent request-level attributes used for request identification
// and correlation across services.
const (
	// RequestID stores a unique identifier for the request.
	// It is typically extracted from the X-Request-ID header or generated
	// if not present. This identifier is used for correlating logs and traces
	// related to a single request across multiple services.
	RequestID = "req.id"
)
