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

import (
	"rivaas.dev/binding"
	"rivaas.dev/validation"
)

// bindConfig holds unified configuration for Bind operations.
// It combines binding and validation options into a single configuration.
type bindConfig struct {
	// Binding behavior
	strict         bool // Reject unknown fields
	skipValidation bool // Skip validation step

	// Validation behavior
	partial  bool                   // Partial validation (PATCH)
	presence validation.PresenceMap // Field presence tracking

	// Pass-through options for advanced use
	bindingOpts    []binding.Option
	validationOpts []validation.Option
}

// BindOption configures Bind behavior.
// BindOptions can be passed to [Context.Bind], [Context.MustBind], [Bind], and [MustBind].
type BindOption func(*bindConfig)

// WithStrict rejects unknown JSON fields during binding.
// Use this to catch typos and API drift early.
//
// Example:
//
//	req, err := app.Bind[CreateUserRequest](c, app.WithStrict())
//
// Equivalent to the old BindAndValidateStrict method.
func WithStrict() BindOption {
	return func(cfg *bindConfig) {
		cfg.strict = true
	}
}

// WithPartial enables partial validation for PATCH requests.
// Only fields present in the request body are validated.
// The "required" constraint is ignored for absent fields.
//
// Example:
//
//	req, err := app.Bind[UpdateUserRequest](c, app.WithPartial())
func WithPartial() BindOption {
	return func(cfg *bindConfig) {
		cfg.partial = true
	}
}

// WithoutValidation skips the validation step.
// Use when you only need binding, or will validate separately.
//
// Example:
//
//	req, err := app.Bind[Request](c, app.WithoutValidation())
//
// Equivalent to using [Context.BindOnly] or [BindOnly].
func WithoutValidation() BindOption {
	return func(cfg *bindConfig) {
		cfg.skipValidation = true
	}
}

// WithPresence explicitly sets the presence map.
// Usually auto-detected from JSON body; use this for custom scenarios.
//
// Example:
//
//	pm, _ := validation.ComputePresence(rawJSON)
//	req, err := app.Bind[Request](c, app.WithPresence(pm))
func WithPresence(pm validation.PresenceMap) BindOption {
	return func(cfg *bindConfig) {
		cfg.presence = pm
	}
}

// WithBindingOptions passes options directly to the binding package.
// Use for advanced binding configuration.
//
// Example:
//
//	req, err := app.Bind[Request](c,
//	    app.WithBindingOptions(
//	        binding.WithTimeLayouts("2006-01-02"),
//	        binding.WithMaxDepth(16),
//	    ),
//	)
func WithBindingOptions(opts ...binding.Option) BindOption {
	return func(cfg *bindConfig) {
		cfg.bindingOpts = append(cfg.bindingOpts, opts...)
	}
}

// WithValidationOptions passes options directly to the validation package.
// Use for advanced validation configuration.
//
// Example:
//
//	req, err := app.Bind[Request](c,
//	    app.WithValidationOptions(
//	        validation.WithStrategy(validation.StrategyTags),
//	        validation.WithMaxErrors(10),
//	    ),
//	)
func WithValidationOptions(opts ...validation.Option) BindOption {
	return func(cfg *bindConfig) {
		cfg.validationOpts = append(cfg.validationOpts, opts...)
	}
}

// applyBindOptions creates a bindConfig with default values and applies the given options.
func applyBindOptions(opts []BindOption) *bindConfig {
	cfg := &bindConfig{
		strict:         false,
		skipValidation: false,
		partial:        false,
		presence:       nil,
		bindingOpts:    nil,
		validationOpts: nil,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}
