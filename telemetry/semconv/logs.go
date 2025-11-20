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
