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

package methodoverride

// Option defines functional options for method override middleware configuration.
type Option func(*config)

// config holds the configuration for the method override middleware.
type config struct {
	header           string
	queryParam       string
	allow            []string
	onlyOn           []string
	respectBody      bool
	requireCSRFToken bool
}

// defaultConfig returns the default configuration for method override middleware.
func defaultConfig() *config {
	return &config{
		header:           "X-HTTP-Method-Override",
		queryParam:       "_method",
		allow:            []string{"PUT", "PATCH", "DELETE"},
		onlyOn:           []string{"POST"},
		respectBody:      false,
		requireCSRFToken: false,
	}
}

// WithHeader sets the header name for method override.
// Default: "X-HTTP-Method-Override"
//
// Example:
//
//	methodoverride.New(methodoverride.WithHeader("X-HTTP-Method"))
func WithHeader(header string) Option {
	return func(cfg *config) {
		cfg.header = header
	}
}

// WithQueryParam sets the query parameter name for method override.
// Default: "_method"
// Set to empty string to disable query parameter support.
//
// Example:
//
//	methodoverride.New(methodoverride.WithQueryParam("_method"))
func WithQueryParam(param string) Option {
	return func(cfg *config) {
		cfg.queryParam = param
	}
}

// WithAllow sets the allowlist of HTTP methods that can be overridden.
// Default: ["PUT", "PATCH", "DELETE"]
//
// Example:
//
//	methodoverride.New(methodoverride.WithAllow("PUT", "PATCH", "DELETE", "HEAD"))
func WithAllow(methods ...string) Option {
	return func(cfg *config) {
		cfg.allow = methods
	}
}

// WithOnlyOn sets which HTTP methods can trigger method override.
// Default: ["POST"]
// Only requests with these methods will be checked for override.
//
// Example:
//
//	methodoverride.New(methodoverride.WithOnlyOn("POST", "GET"))
func WithOnlyOn(methods ...string) Option {
	return func(cfg *config) {
		cfg.onlyOn = methods
	}
}

// WithRespectBody requires a request body for method overrides.
// When enabled, requests without a body will not be overridden.
// Default: false
//
// Example:
//
//	methodoverride.New(methodoverride.WithRespectBody(true))
func WithRespectBody(required bool) Option {
	return func(cfg *config) {
		cfg.respectBody = required
	}
}

// WithRequireCSRFToken requires CSRF token verification before allowing method override.
// When enabled, the middleware expects a CSRF verification middleware to run first
// and set c.Request.Context().Value(CSRFVerifiedKey) == true.
// Default: false
//
// SECURITY WARNING: This middleware should only be used when you control
// the client (e.g., HTML forms). Never enable for public APIs without
// RequireCSRFToken=true, as it can be exploited for CSRF attacks.
//
// Example:
//
//	r.Use(csrf.Verify()) // Sets CSRF verification flag
//	r.Use(methodoverride.New(methodoverride.WithRequireCSRFToken(true)))
func WithRequireCSRFToken(required bool) Option {
	return func(cfg *config) {
		cfg.requireCSRFToken = required
	}
}
