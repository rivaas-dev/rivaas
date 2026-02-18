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

package validation

import (
	"context"
	"errors"
	"maps"
	"reflect"

	"github.com/go-playground/validator/v10"
)

// MessageFunc generates a dynamic error message for parameterized validation tags.
// Use [WithMessageFunc] to configure messages for tags like "min", "max", "len", "oneof"
// that include parameters.
//
// The function receives the tag parameter (e.g., "3" for `min=3`) and the field's
// reflect.Kind to enable type-aware messages (e.g., "characters" for strings vs
// plain numbers for integers).
type MessageFunc func(param string, kind reflect.Kind) string

// Strategy defines the validation approach to use.
// Use [WithStrategy] to set a strategy, or leave as [StrategyAuto] for automatic selection.
type Strategy int

const (
	// StrategyAuto automatically selects the best strategy based on the type.
	// Priority: Interface methods > Struct tags > JSON Schema
	StrategyAuto Strategy = iota

	// StrategyTags uses struct tag validation (go-playground/validator).
	StrategyTags

	// StrategyJSONSchema uses JSON Schema validation.
	StrategyJSONSchema

	// StrategyInterface calls Validate() or ValidateContext() methods.
	StrategyInterface
)

// Redactor is a function that determines if a field should be redacted in error messages.
// It returns true if the field at the given path should have its value hidden.
// Use [WithRedactor] to configure a redactor for a [Validator].
//
// Example:
//
//	redactor := func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}
type Redactor func(path string) bool

// customTag holds a custom validation tag registration for use with [WithCustomTag].
type customTag struct {
	name string
	fn   validator.Func
}

// config holds internal validation configuration used by [Validator].
type config struct {
	strategy              Strategy
	runAll                bool
	requireAny            bool
	partial               bool
	maxErrors             int
	maxFields             int // Max fields to validate in partial mode (0 = default)
	maxCachedSchemas      int // Max schemas to cache (0 = default)
	disallowUnknownFields bool
	ctx                   context.Context // Optional context override
	presence              PresenceMap
	customSchema          string
	customSchemaID        string
	customValidator       func(any) error
	fieldNameMapper       func(string) string
	redactor              Redactor
	customTags            []customTag
	messages              map[string]string      // tag -> static message
	messageFuncs          map[string]MessageFunc // tag -> dynamic message function
}

// validate checks the configuration for errors.
func (c *config) validate() error {
	if c.maxErrors < 0 {
		return errors.New("maxErrors must be non-negative")
	}
	if c.maxFields < 0 {
		return errors.New("maxFields must be non-negative")
	}
	if c.maxCachedSchemas < 0 {
		return errors.New("maxCachedSchemas must be non-negative")
	}

	return nil
}

// clone creates a copy of the config for per-call option merging.
func (c *config) clone() *config {
	clone := *c
	// Deep copy slices
	if c.customTags != nil {
		clone.customTags = make([]customTag, 0, len(c.customTags))
		clone.customTags = append(clone.customTags, c.customTags...)
	}
	// Deep copy maps
	if c.messages != nil {
		clone.messages = make(map[string]string, len(c.messages))
		maps.Copy(clone.messages, c.messages)
	}
	if c.messageFuncs != nil {
		clone.messageFuncs = make(map[string]MessageFunc, len(c.messageFuncs))
		maps.Copy(clone.messageFuncs, c.messageFuncs)
	}

	return &clone
}

// Option is a functional option for configuring validation.
// Options can be passed to [New], [MustNew], [Validate], or [Validator.Validate].
type Option func(*config)

// WithStrategy sets the validation strategy.
//
// Example:
//
//	validator.Validate(ctx, &req, WithStrategy(StrategyTags))
func WithStrategy(strategy Strategy) Option {
	return func(c *config) {
		c.strategy = strategy
	}
}

// WithRunAll runs all applicable validation strategies and aggregates errors.
// By default, validation stops at the first successful strategy.
//
// Example:
//
//	validator.Validate(ctx, &req, WithRunAll(true))
func WithRunAll(runAll bool) Option {
	return func(c *config) {
		c.runAll = runAll
	}
}

// WithRequireAny requires at least one validation strategy to pass when using WithRunAll.
// When enabled with WithRunAll(true), it causes validation to succeed if at least one strategy
// produces no errors, even if other strategies fail.
//
// It is useful when you have multiple validation strategies and want to accept
// the value if it passes any one of them (OR logic).
//
// Example:
//
//	// Validate with all strategies, succeed if any one passes
//	err := validator.Validate(ctx, &req,
//	    WithRunAll(true),
//	    WithRequireAny(true),
//	)
func WithRequireAny(require bool) Option {
	return func(c *config) {
		c.requireAny = require
	}
}

// WithPartial enables partial update validation mode (for PATCH requests).
// It validates only present fields and ignores "required" constraints
// for absent fields.
//
// Example:
//
//	validator.Validate(ctx, &req, WithPartial(true), WithPresence(presenceMap))
func WithPartial(partial bool) Option {
	return func(c *config) {
		c.partial = partial
	}
}

// WithMaxErrors limits the number of errors returned.
// Set to 0 for unlimited errors (default).
//
// Example:
//
//	validator.Validate(ctx, &req, WithMaxErrors(10))
func WithMaxErrors(maxErrors int) Option {
	return func(c *config) {
		c.maxErrors = maxErrors
	}
}

// WithDisallowUnknownFields rejects JSON with unknown fields (typo detection).
// When enabled, it causes BindJSONStrict to reject requests with fields not defined in the struct.
//
// Example:
//
//	BindAndValidateStrict(&req, WithDisallowUnknownFields(true))
func WithDisallowUnknownFields(disallow bool) Option {
	return func(c *config) {
		c.disallowUnknownFields = disallow
	}
}

// WithContext overrides the context used for validation.
// This is useful when you need a different context than the one passed to Validate().
//
// Note: In most cases, you should simply pass the context directly to Validate().
// This option exists for advanced use cases where context override is needed.
//
// Example:
//
//	validator.Validate(requestCtx, &req, WithContext(backgroundCtx))
func WithContext(ctx context.Context) Option {
	return func(c *config) {
		c.ctx = ctx
	}
}

// WithPresence sets the presence map for partial validation.
// The [PresenceMap] tracks which fields were provided in the request body.
// Use [ComputePresence] to create a PresenceMap from raw JSON.
//
// Example:
//
//	presence, _ := ComputePresence(rawJSON)
//	validator.Validate(ctx, &req, WithPresence(presence), WithPartial(true))
func WithPresence(presence PresenceMap) Option {
	return func(c *config) {
		c.presence = presence
	}
}

// WithCustomSchema sets a custom JSON Schema for validation.
// This overrides any schema provided by the [JSONSchemaProvider] interface.
//
// Example:
//
//	schema := `{"type": "object", "properties": {"email": {"type": "string", "format": "email"}}}`
//	validator.Validate(ctx, &req, WithCustomSchema("user-schema", schema))
func WithCustomSchema(id, schema string) Option {
	return func(c *config) {
		c.customSchemaID = id
		c.customSchema = schema
	}
}

// WithCustomValidator sets a custom validation function.
// It calls the function before any other validation strategies.
//
// Example:
//
//	validator.Validate(ctx, &req, WithCustomValidator(func(v any) error {
//	    req := v.(*UserRequest)
//	    if req.Age < 18 {
//	        return errors.New("must be 18 or older")
//	    }
//	    return nil
//	}))
func WithCustomValidator(fn func(any) error) Option {
	return func(c *config) {
		c.customValidator = fn
	}
}

// WithFieldNameMapper sets a function to transform field names in error messages.
// It is useful for localization or custom naming conventions.
//
// Example:
//
//	validator.Validate(ctx, &req, WithFieldNameMapper(func(name string) string {
//	    return strings.ReplaceAll(name, "_", " ")
//	}))
func WithFieldNameMapper(mapper func(string) string) Option {
	return func(c *config) {
		c.fieldNameMapper = mapper
	}
}

// WithRedactor sets a [Redactor] function to hide sensitive values in error messages.
// The redactor returns true if the field at the given path should be redacted.
//
// Example:
//
//	validator.Validate(ctx, &req, WithRedactor(func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}))
func WithRedactor(redactor Redactor) Option {
	return func(c *config) {
		c.redactor = redactor
	}
}

// WithMaxFields sets the maximum number of fields to validate in partial mode.
// It prevents pathological inputs with extremely large presence maps.
// Set to 0 to use the default (10000).
//
// Example:
//
//	validator.Validate(ctx, &req, WithMaxFields(5000), WithPartial(true))
func WithMaxFields(maxFields int) Option {
	return func(c *config) {
		c.maxFields = maxFields
	}
}

// WithMaxCachedSchemas sets the maximum number of JSON schemas to cache.
// Set to 0 to use the default (1024).
//
// Example:
//
//	validation.MustNew(validation.WithMaxCachedSchemas(2048))
func WithMaxCachedSchemas(maxCachedSchemas int) Option {
	return func(c *config) {
		c.maxCachedSchemas = maxCachedSchemas
	}
}

// WithCustomTag registers a custom validation tag for use in struct tags.
// Custom tags are registered when the [Validator] is created.
//
// Example:
//
//	validator := validation.MustNew(
//	    validation.WithCustomTag("phone", func(fl validator.FieldLevel) bool {
//	        return phoneRegex.MatchString(fl.Field().String())
//	    }),
//	)
//
//	type User struct {
//	    Phone string `validate:"phone"`
//	}
func WithCustomTag(name string, fn validator.Func) Option {
	return func(c *config) {
		c.customTags = append(c.customTags, customTag{name: name, fn: fn})
	}
}

// WithMessages sets static error messages for validation tags.
// Messages override the default English messages for specified tags.
// Unspecified tags continue to use defaults.
//
// Example:
//
//	validator := validation.MustNew(
//	    validation.WithMessages(map[string]string{
//	        "required": "cannot be empty",
//	        "email":    "invalid email format",
//	    }),
//	)
func WithMessages(messages map[string]string) Option {
	return func(c *config) {
		if c.messages == nil {
			c.messages = make(map[string]string)
		}
		maps.Copy(c.messages, messages)
	}
}

// WithMessageFunc sets a dynamic message generator for a parameterized tag.
// Use for tags like "min", "max", "len", "oneof" that include parameters.
//
// Example:
//
//	validator := validation.MustNew(
//	    validation.WithMessageFunc("min", func(param string, kind reflect.Kind) string {
//	        if kind == reflect.String {
//	            return fmt.Sprintf("too short (min %s chars)", param)
//	        }
//	        return fmt.Sprintf("too small (min %s)", param)
//	    }),
//	)
func WithMessageFunc(tag string, fn MessageFunc) Option {
	return func(c *config) {
		if c.messageFuncs == nil {
			c.messageFuncs = make(map[string]MessageFunc)
		}
		c.messageFuncs[tag] = fn
	}
}

// newConfig creates a new validation config with defaults.
func newConfig() *config {
	return &config{
		strategy:              StrategyAuto,
		runAll:                false,
		requireAny:            false,
		partial:               false,
		maxErrors:             0,
		maxFields:             0, // 0 means use default (10000)
		maxCachedSchemas:      0, // 0 means use default (1024)
		disallowUnknownFields: false,
	}
}

// applyOptions applies options to a config, returning a new config.
// This is used for per-call options that override the validator's base config.
func applyOptions(base *config, opts ...Option) *config {
	if len(opts) == 0 {
		return base
	}
	cfg := base.clone()
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}
