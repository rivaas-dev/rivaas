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

package app

// Semantic conventions for observability (OpenTelemetry-compatible field names).
//
// These constants are used internally by the framework to build request-scoped
// loggers and structured observability data. Users don't need to use these
// directly - all context is added automatically to c.Logger().
//
// The field names follow OpenTelemetry semantic conventions where applicable,
// ensuring compatibility with standard observability tools and platforms.

// HTTP semantic conventions.
const (
	fieldHTTPMethod = "http.method"
	fieldHTTPRoute  = "http.route"
	fieldHTTPTarget = "http.target"
)

// Network semantic conventions.
const (
	fieldNetworkClientIP = "network.client.ip"
)

// Trace correlation fields.
const (
	fieldTraceID = "trace_id"
	fieldSpanID  = "span_id"
)

// Request identification.
const (
	fieldRequestID = "req.id"
)
