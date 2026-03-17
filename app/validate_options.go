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
	"fmt"

	"rivaas.dev/validation"
)

// validateConfig holds configuration for [Context.Validate] operations.
type validateConfig struct {
	partial        bool
	strict         bool
	validationOpts []validation.Option
}

// ValidateOption configures [Context.Validate] behavior.
// ValidateOptions can be passed to [Context.Validate].
type ValidateOption func(*validateConfig)

// WithValidatePartial enables partial validation for this validate call.
// Only fields present in the request (or in the presence map) are validated;
// "required" is ignored for absent fields. Use after [Context.BindOnly] for PATCH-style flows.
//
// Example:
//
//	if err := c.Validate(&req, app.WithValidatePartial()); err != nil {
//	    c.Fail(err)
//	    return
//	}
func WithValidatePartial() ValidateOption {
	return func(cfg *validateConfig) {
		cfg.partial = true
	}
}

// WithValidateStrict disallows unknown fields for this validate call.
// Use when validating JSON-backed structs and you want to reject unknown keys.
//
// Example:
//
//	if err := c.Validate(&req, app.WithValidateStrict()); err != nil {
//	    c.Fail(err)
//	    return
//	}
func WithValidateStrict() ValidateOption {
	return func(cfg *validateConfig) {
		cfg.strict = true
	}
}

// WithValidateOptions passes options directly to the validation package for this validate call.
// Use for advanced validation configuration when using [Context.Validate].
//
// Example:
//
//	if err := c.Validate(&req,
//	    app.WithValidatePartial(),
//	    app.WithValidateOptions(validation.WithMaxErrors(5)),
//	); err != nil {
//	    c.Fail(err)
//	    return
//	}
func WithValidateOptions(opts ...validation.Option) ValidateOption {
	return func(cfg *validateConfig) {
		cfg.validationOpts = append(cfg.validationOpts, opts...)
	}
}

// applyValidateOptions creates a validateConfig with default values and applies the given options.
// Returns an error if any option is nil.
func applyValidateOptions(opts []ValidateOption) (*validateConfig, error) {
	cfg := &validateConfig{
		partial:        false,
		strict:         false,
		validationOpts: nil,
	}
	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("app: validate option at index %d cannot be nil", i)
		}
		opt(cfg)
	}
	return cfg, nil
}
