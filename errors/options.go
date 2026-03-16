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

package errors

import "fmt"

// Option configures a formatter. Options apply to an internal config;
// New/MustNew build a Formatter from the validated config.
type Option func(*config)

// formatterKind identifies which formatter implementation to build.
type formatterKind int

const (
	kindRFC9457 formatterKind = iota + 1
	kindJSONAPI
	kindSimple
)

// config holds formatter configuration. Options mutate config; New builds a Formatter from it.
type config struct {
	kind     formatterKind
	conflict bool // true if more than one formatter type option was applied

	// RFC9457-specific
	rfc9457BaseURL   string
	typeResolver     func(error) string
	statusResolver   func(error) int
	errorIDGenerator func() string
	disableErrorID   bool
}

// defaultConfig returns config with no formatter type set; New treats "unset" as RFC9457 with empty base URL.
func defaultConfig() *config {
	return &config{
		kind:           0, // unset: formatterFromConfig will use RFC9457
		rfc9457BaseURL: "",
	}
}

// validate returns an error if config is invalid (e.g. multiple formatter types specified).
func (c *config) validate() error {
	if c.conflict {
		return fmt.Errorf("errors: multiple formatter types specified (exactly one of WithRFC9457, WithJSONAPI, WithSimple required)")
	}
	return nil
}

// WithRFC9457 selects the RFC 9457 Problem Details formatter and sets the base URL
// for problem type URIs. Empty base URL is allowed (relative URIs).
//
// Example:
//
//	formatter := errors.MustNew(errors.WithRFC9457("https://api.example.com/problems"))
func WithRFC9457(baseURL string) Option {
	return func(c *config) {
		if c.kind != 0 && c.kind != kindRFC9457 {
			c.conflict = true
		}
		c.kind = kindRFC9457
		c.rfc9457BaseURL = baseURL
	}
}

// WithJSONAPI selects the JSON:API error formatter.
//
// Example:
//
//	formatter := errors.MustNew(errors.WithJSONAPI())
func WithJSONAPI() Option {
	return func(c *config) {
		if c.kind != 0 && c.kind != kindJSONAPI {
			c.conflict = true
		}
		c.kind = kindJSONAPI
	}
}

// WithSimple selects the Simple JSON error formatter.
//
// Example:
//
//	formatter := errors.MustNew(errors.WithSimple())
func WithSimple() Option {
	return func(c *config) {
		if c.kind != 0 && c.kind != kindSimple {
			c.conflict = true
		}
		c.kind = kindSimple
	}
}

// WithProblemTypeResolver sets the TypeResolver for the RFC9457 formatter.
// Only applies when using WithRFC9457. If nil, default mapping is used.
func WithProblemTypeResolver(fn func(error) string) Option {
	return func(c *config) {
		c.typeResolver = fn
	}
}

// WithProblemStatusResolver sets the StatusResolver for the RFC9457 formatter.
// Only applies when using WithRFC9457. If nil, default logic (ErrorType interface, then 500) is used.
func WithProblemStatusResolver(fn func(error) int) Option {
	return func(c *config) {
		c.statusResolver = fn
	}
}

// WithProblemErrorIDGenerator sets the ErrorIDGenerator for the RFC9457 formatter.
// Only applies when using WithRFC9457. If nil, default UUID-based generation is used.
func WithProblemErrorIDGenerator(fn func() string) Option {
	return func(c *config) {
		c.errorIDGenerator = fn
	}
}

// WithDisableProblemErrorID disables automatic error ID generation for the RFC9457 formatter.
// Only applies when using WithRFC9457.
func WithDisableProblemErrorID() Option {
	return func(c *config) {
		c.disableErrorID = true
	}
}

// WithStatusResolver sets the StatusResolver for formatters that support it (RFC9457, JSONAPI, Simple).
// If nil, default logic (ErrorType interface or 500) is used.
func WithStatusResolver(fn func(error) int) Option {
	return func(c *config) {
		c.statusResolver = fn
	}
}
