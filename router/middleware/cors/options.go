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

package cors

// WithAllowedOrigins sets the list of allowed origins.
// Use this for specific origins like ["https://example.com", "https://app.example.com"].
//
// Example:
//
//	cors.New(cors.WithAllowedOrigins("https://example.com"))
func WithAllowedOrigins(origins ...string) Option {
	return func(cfg *config) {
		cfg.allowedOrigins = origins
		cfg.allowAllOrigins = false
	}
}

// WithAllowAllOrigins allows all origins by setting Access-Control-Allow-Origin: *.
// WARNING: This is insecure and should only be used for public APIs.
//
// Example:
//
//	cors.New(cors.WithAllowAllOrigins(true))
func WithAllowAllOrigins(allow bool) Option {
	return func(cfg *config) {
		cfg.allowAllOrigins = allow
	}
}

// WithAllowedMethods sets the list of allowed HTTP methods.
// Default: ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]
//
// Example:
//
//	cors.New(cors.WithAllowedMethods("GET", "POST"))
func WithAllowedMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.allowedMethods = methods
	}
}

// WithAllowedHeaders sets the list of allowed request headers.
// Default: ["Origin", "Content-Type", "Accept", "Authorization"]
//
// Example:
//
//	cors.New(cors.WithAllowedHeaders("Content-Type", "X-Custom-Header"))
func WithAllowedHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.allowedHeaders = headers
	}
}

// WithExposedHeaders sets the list of headers exposed to the client.
// These headers can be accessed by the client-side JavaScript.
//
// Example:
//
//	cors.New(cors.WithExposedHeaders("X-Request-ID", "X-Rate-Limit"))
func WithExposedHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.exposedHeaders = headers
	}
}

// WithAllowCredentials enables credentials (cookies, authorization headers, TLS certificates).
// When true, Access-Control-Allow-Origin cannot be "*".
// Default: false
//
// Example:
//
//	cors.New(cors.WithAllowCredentials(true))
func WithAllowCredentials(allow bool) Option {
	return func(cfg *config) {
		cfg.allowCredentials = allow
	}
}

// WithMaxAge sets the max age for preflight cache in seconds.
// Default: 3600 (1 hour)
//
// Example:
//
//	cors.New(cors.WithMaxAge(7200)) // 2 hours
func WithMaxAge(seconds int) Option {
	return func(cfg *config) {
		cfg.maxAge = seconds
	}
}

// WithAllowOriginFunc sets a custom function to validate origins dynamically.
// This is useful for pattern matching or database lookups.
//
// Example:
//
//	cors.New(cors.WithAllowOriginFunc(func(origin string) bool {
//	    return strings.HasSuffix(origin, ".example.com")
//	}))
func WithAllowOriginFunc(fn func(origin string) bool) Option {
	return func(cfg *config) {
		cfg.allowOriginFunc = fn
	}
}
