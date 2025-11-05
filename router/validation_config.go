package router

import "context"

// validationConfig holds validation configuration.
// This struct is private and should be constructed via ValidationOption functions.
type validationConfig struct {
	strategy              ValidationStrategy
	runAll                bool
	requireAny            bool
	partial               bool
	maxErrors             int
	maxFields             int // Max fields to validate in partial mode (0 = default)
	maxCachedSchemas      int // Max schemas to cache (0 = default)
	disallowUnknownFields bool
	ctx                   context.Context
	presence              PresenceMap
	customSchema          string
	customSchemaID        string
	customValidator       func(any) error
	fieldNameMapper       func(string) string
	redactor              Redactor
}

// ValidationOption is a functional option for configuring validation.
type ValidationOption func(*validationConfig)

// WithStrategy sets the validation strategy.
//
// Example:
//
//	Validate(&req, WithStrategy(ValidationTags))
func WithStrategy(strategy ValidationStrategy) ValidationOption {
	return func(c *validationConfig) {
		c.strategy = strategy
	}
}

// WithRunAll runs all applicable validation strategies and aggregates errors.
// By default, validation stops at the first successful strategy.
//
// Example:
//
//	Validate(&req, WithRunAll(true))
func WithRunAll(runAll bool) ValidationOption {
	return func(c *validationConfig) {
		c.runAll = runAll
	}
}

// WithRequireAny requires at least one validation strategy to pass.
// This is useful when you want to ensure at least one validation method succeeds.
func WithRequireAny(require bool) ValidationOption {
	return func(c *validationConfig) {
		c.requireAny = require
	}
}

// WithPartial enables partial update validation mode (for PATCH requests).
// In this mode, only present fields are validated, and "required" constraints
// are ignored for absent fields.
//
// Example:
//
//	Validate(&req, WithPartial(true), WithPresence(presenceMap))
func WithPartial(partial bool) ValidationOption {
	return func(c *validationConfig) {
		c.partial = partial
	}
}

// WithMaxErrors limits the number of errors returned.
// Set to 0 for unlimited errors (default).
//
// Example:
//
//	Validate(&req, WithMaxErrors(10))
func WithMaxErrors(max int) ValidationOption {
	return func(c *validationConfig) {
		c.maxErrors = max
	}
}

// WithDisallowUnknownFields rejects JSON with unknown fields (typo detection).
// When enabled, BindJSONStrict will reject requests with fields not defined in the struct.
//
// Example:
//
//	BindAndValidateStrict(&req, WithDisallowUnknownFields(true))
func WithDisallowUnknownFields(disallow bool) ValidationOption {
	return func(c *validationConfig) {
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
//	Validate(&req, WithContext(ctx))
func WithContext(ctx context.Context) ValidationOption {
	return func(c *validationConfig) {
		c.ctx = ctx
	}
}

// WithPresence sets the presence map for partial validation.
// The presence map tracks which fields were provided in the request body.
//
// Example:
//
//	presence := c.Presence()
//	Validate(&req, WithPresence(presence), WithPartial(true))
func WithPresence(presence PresenceMap) ValidationOption {
	return func(c *validationConfig) {
		c.presence = presence
	}
}

// WithCustomSchema sets a custom JSON Schema for validation.
//
// Example:
//
//	schema := `{"type": "object", "properties": {"email": {"type": "string", "format": "email"}}}`
//	Validate(&req, WithCustomSchema("user-schema", schema))
func WithCustomSchema(id, schema string) ValidationOption {
	return func(c *validationConfig) {
		c.customSchemaID = id
		c.customSchema = schema
	}
}

// WithCustomValidator sets a custom validation function.
// This function is called before any other validation strategies.
//
// Example:
//
//	Validate(&req, WithCustomValidator(func(v any) error {
//	    req := v.(*UserRequest)
//	    if req.Age < 18 {
//	        return errors.New("must be 18 or older")
//	    }
//	    return nil
//	}))
func WithCustomValidator(fn func(any) error) ValidationOption {
	return func(c *validationConfig) {
		c.customValidator = fn
	}
}

// WithFieldNameMapper sets a function to transform field names in error messages.
// This is useful for localization or custom naming conventions.
//
// Example:
//
//	Validate(&req, WithFieldNameMapper(func(name string) string {
//	    return strings.ReplaceAll(name, "_", " ")
//	}))
func WithFieldNameMapper(mapper func(string) string) ValidationOption {
	return func(c *validationConfig) {
		c.fieldNameMapper = mapper
	}
}

// WithRedactor sets a function to redact sensitive values in error messages.
// Returns true if the field at the given path should be redacted.
//
// Example:
//
//	Validate(&req, WithRedactor(func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}))
func WithRedactor(redactor Redactor) ValidationOption {
	return func(c *validationConfig) {
		c.redactor = redactor
	}
}

// WithMaxFields sets the maximum number of fields to validate in partial mode.
// This prevents pathological inputs with extremely large presence maps.
// Set to 0 to use the default (10000).
//
// Example:
//
//	Validate(&req, WithMaxFields(5000), WithPartial(true))
func WithMaxFields(max int) ValidationOption {
	return func(c *validationConfig) {
		c.maxFields = max
	}
}

// WithMaxCachedSchemas sets the maximum number of JSON schemas to cache.
// This controls memory usage for schema caching.
// Set to 0 to use the default (1024).
//
// Example:
//
//	Validate(&req, WithMaxCachedSchemas(2048))
func WithMaxCachedSchemas(max int) ValidationOption {
	return func(c *validationConfig) {
		c.maxCachedSchemas = max
	}
}

// newValidationConfig creates a new validation config with defaults and applies options.
func newValidationConfig(opts ...ValidationOption) *validationConfig {
	config := &validationConfig{
		strategy:              ValidationAuto,
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
		opt(config)
	}

	return config
}
