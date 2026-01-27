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

// Package requestid provides middleware for generating and managing
// unique request IDs for distributed tracing and request correlation.
package requestid

// WithHeader sets the header name for the request ID.
// Default: "X-Request-ID"
//
// Example:
//
//	requestid.New(requestid.WithHeader("X-Trace-ID"))
func WithHeader(headerName string) Option {
	return func(cfg *config) {
		cfg.headerName = headerName
	}
}

// WithULID uses ULID for request ID generation instead of UUID v7.
// ULID provides time-ordered, lexicographically sortable identifiers
// with a compact 26-character representation.
//
// ULID format: 01ARZ3NDEKTSV4RRFFQ69G5FAV (26 characters)
// UUID v7 format: 018f3e9a-1b2c-7def-8000-abcdef123456 (36 characters)
//
// Use ULID when you need shorter IDs or case-insensitive identifiers.
//
// Example:
//
//	requestid.New(requestid.WithULID())
func WithULID() Option {
	return func(cfg *config) {
		cfg.generator = generateULID
	}
}

// WithGenerator sets a custom function to generate request IDs.
// The generator function should return a unique string for each call.
//
// By default, UUID v7 is used (time-ordered, RFC 9562 compliant).
// Use this option when you need a custom format.
//
// Example with custom format:
//
//	requestid.New(requestid.WithGenerator(func() string {
//	    return fmt.Sprintf("req-%d-%s", time.Now().Unix(), randomString(8))
//	}))
func WithGenerator(generator func() string) Option {
	return func(cfg *config) {
		cfg.generator = generator
	}
}

// WithAllowClientID controls whether to accept request IDs from clients.
// When true, if the client provides a request ID in the header, it will be used.
// When false, always generate a new request ID regardless of client input.
// Default: true
//
// Security note: Set to false if you need to ensure all request IDs are server-generated.
//
// Example:
//
//	requestid.New(requestid.WithAllowClientID(false))
func WithAllowClientID(allow bool) Option {
	return func(cfg *config) {
		cfg.allowClientID = allow
	}
}
