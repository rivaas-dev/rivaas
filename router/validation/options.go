package validation

import "context"

// Strategy defines the validation approach to use.
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

// Redactor function for sensitive fields.
// Returns true if the field at the given path should be redacted.
//
// Example:
//
//	redactor := func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}
type Redactor func(path string) bool

// config holds internal validation configuration.
type config struct {
	strategy              Strategy
	runAll                bool
	requireAny            bool
	partial               bool
	maxErrors             int
	maxFields             int // Max fields to validate in partial mode (0 = default)
	maxCachedSchemas      int // Max schemas to cache (0 = default)
	disallowUnknownFields bool
	ctx                   context.Context
	ctxExplicit           bool // Track if context was explicitly set via WithContext()
	presence              PresenceMap
	customSchema          string
	customSchemaID        string
	customValidator       func(any) error
	fieldNameMapper       func(string) string
	redactor              Redactor
}

// Option is a functional option for configuring validation.
type Option func(*config)

// WithStrategy sets the validation strategy.
//
// Example:
//
//	Validate(ctx, &req, WithStrategy(StrategyTags))
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
//	Validate(ctx, &req, WithRunAll(true))
func WithRunAll(runAll bool) Option {
	return func(c *config) {
		c.runAll = runAll
	}
}

// WithRequireAny requires at least one validation strategy to pass.
// This is useful when you want to ensure at least one validation method succeeds.
func WithRequireAny(require bool) Option {
	return func(c *config) {
		c.requireAny = require
	}
}

// WithPartial enables partial update validation mode (for PATCH requests).
// In this mode, only present fields are validated, and "required" constraints
// are ignored for absent fields.
//
// Example:
//
//	Validate(ctx, &req, WithPartial(true), WithPresence(presenceMap))
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
//	Validate(ctx, &req, WithMaxErrors(10))
func WithMaxErrors(maxErrors int) Option {
	return func(c *config) {
		c.maxErrors = maxErrors
	}
}

// WithDisallowUnknownFields rejects JSON with unknown fields (typo detection).
// When enabled, BindJSONStrict will reject requests with fields not defined in the struct.
//
// Example:
//
//	BindAndValidateStrict(&req, WithDisallowUnknownFields(true))
func WithDisallowUnknownFields(disallow bool) Option {
	return func(c *config) {
		c.disallowUnknownFields = disallow
	}
}

// WithContext sets the context for validation.
// Context is used for:
//   - Locale-aware validation messages
//   - Tenant-specific rules
//   - Time-bound schema fetching
//   - Passing raw JSON body for optimization
//
// Example:
//
//	Validate(ctx, &req, WithContext(ctx))
func WithContext(ctx context.Context) Option {
	return func(c *config) {
		c.ctx = ctx
		c.ctxExplicit = true
	}
}

// WithPresence sets the presence map for partial validation.
// The presence map tracks which fields were provided in the request body.
//
// Example:
//
//	presence := c.Presence()
//	Validate(ctx, &req, WithPresence(presence), WithPartial(true))
func WithPresence(presence PresenceMap) Option {
	return func(c *config) {
		c.presence = presence
	}
}

// WithCustomSchema sets a custom JSON Schema for validation.
//
// Example:
//
//	schema := `{"type": "object", "properties": {"email": {"type": "string", "format": "email"}}}`
//	Validate(ctx, &req, WithCustomSchema("user-schema", schema))
func WithCustomSchema(id, schema string) Option {
	return func(c *config) {
		c.customSchemaID = id
		c.customSchema = schema
	}
}

// WithCustomValidator sets a custom validation function.
// This function is called before any other validation strategies.
//
// Example:
//
//	Validate(ctx, &req, WithCustomValidator(func(v any) error {
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
// This is useful for localization or custom naming conventions.
//
// Example:
//
//	Validate(ctx, &req, WithFieldNameMapper(func(name string) string {
//	    return strings.ReplaceAll(name, "_", " ")
//	}))
func WithFieldNameMapper(mapper func(string) string) Option {
	return func(c *config) {
		c.fieldNameMapper = mapper
	}
}

// WithRedactor sets a function to redact sensitive values in error messages.
// Returns true if the field at the given path should be redacted.
//
// Example:
//
//	Validate(ctx, &req, WithRedactor(func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}))
func WithRedactor(redactor Redactor) Option {
	return func(c *config) {
		c.redactor = redactor
	}
}

// WithMaxFields sets the maximum number of fields to validate in partial mode.
// This prevents pathological inputs with extremely large presence maps.
// Set to 0 to use the default (10000).
//
// Example:
//
//	Validate(ctx, &req, WithMaxFields(5000), WithPartial(true))
func WithMaxFields(maxFields int) Option {
	return func(c *config) {
		c.maxFields = maxFields
	}
}

// WithMaxCachedSchemas sets the maximum number of JSON schemas to cache.
// This controls memory usage for schema caching.
// Set to 0 to use the default (1024).
//
// Example:
//
//	Validate(ctx, &req, WithMaxCachedSchemas(2048))
func WithMaxCachedSchemas(maxCachedSchemas int) Option {
	return func(c *config) {
		c.maxCachedSchemas = maxCachedSchemas
	}
}

// defaultConfig creates a new validation config with defaults and applies options.
func defaultConfig(opts ...Option) *config {
	cfg := &config{
		strategy:              StrategyAuto,
		runAll:                false,
		requireAny:            false,
		partial:               false,
		maxErrors:             0,
		maxFields:             0, // 0 means use default (10000)
		maxCachedSchemas:      0, // 0 means use default (1024)
		disallowUnknownFields: false,
		ctx:                   context.Background(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}
