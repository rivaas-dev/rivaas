package openapi

import "errors"

// Configuration Errors (returned by New)
var (
	// ErrTitleRequired indicates the API title was not provided.
	ErrTitleRequired = errors.New("openapi: title is required")

	// ErrVersionRequired indicates the API version was not provided.
	ErrVersionRequired = errors.New("openapi: version is required")

	// ErrLicenseMutuallyExclusive indicates both license identifier and URL were set.
	ErrLicenseMutuallyExclusive = errors.New("openapi: license identifier and url are mutually exclusive")

	// ErrServerVariablesNeedURL indicates server variables were set without a server URL.
	ErrServerVariablesNeedURL = errors.New("openapi: server variables require a server URL")

	// ErrInvalidVersion indicates an unsupported OpenAPI version was specified.
	ErrInvalidVersion = errors.New("openapi: invalid OpenAPI version")
)

// Generation Errors (returned by Generate)
var (
	// ErrDuplicateOperationID indicates two operations have the same ID.
	ErrDuplicateOperationID = errors.New("openapi: duplicate operation ID")

	// ErrNoOperations indicates Generate was called with no operations.
	ErrNoOperations = errors.New("openapi: at least one operation is required")
)

// Path Errors
var (
	// ErrPathEmpty indicates an empty path was provided.
	ErrPathEmpty = errors.New("openapi: path cannot be empty")

	// ErrPathNoLeadingSlash indicates the path doesn't start with '/'.
	ErrPathNoLeadingSlash = errors.New("openapi: path must start with '/'")

	// ErrPathDuplicateParameter indicates a path parameter appears twice.
	ErrPathDuplicateParameter = errors.New("openapi: duplicate path parameter")

	// ErrPathInvalidParameter indicates an invalid path parameter format.
	ErrPathInvalidParameter = errors.New("openapi: invalid path parameter format")
)

// Validation Errors (when WithValidation enabled)
var (
	// ErrSpecValidationFailed indicates the generated spec failed JSON Schema validation.
	ErrSpecValidationFailed = errors.New("openapi: generated spec failed JSON Schema validation")
)

// Strict Mode Errors (opt-in via WithStrictDownlevel)
var (
	// ErrStrictDownlevelViolation indicates 3.1 features were used with 3.0 target
	// when strict mode is enabled. This error is opt-in via WithStrictDownlevel(true).
	ErrStrictDownlevelViolation = errors.New("openapi: 3.1 features used with 3.0 target in strict mode")
)

// Extension Errors
var (
	// ErrInvalidExtensionKey indicates an extension key doesn't start with "x-".
	ErrInvalidExtensionKey = errors.New("openapi: extension key must start with 'x-'")

	// ErrReservedExtensionKey indicates an extension uses reserved prefix.
	ErrReservedExtensionKey = errors.New("openapi: extension key uses reserved prefix (x-oai- or x-oas-)")
)

// UI Configuration Errors
var (
	// ErrInvalidDocExpansion indicates an invalid docExpansion mode.
	ErrInvalidDocExpansion = errors.New("openapi: invalid docExpansion mode")

	// ErrInvalidDefaultModelRendering indicates an invalid defaultModelRendering mode.
	ErrInvalidDefaultModelRendering = errors.New("openapi: invalid defaultModelRendering mode")

	// ErrInvalidOperationsSorter indicates an invalid operationsSorter mode.
	ErrInvalidOperationsSorter = errors.New("openapi: invalid operationsSorter mode")

	// ErrInvalidTagsSorter indicates an invalid tagsSorter mode.
	ErrInvalidTagsSorter = errors.New("openapi: invalid tagsSorter mode")

	// ErrInvalidSyntaxTheme indicates an invalid syntax theme.
	ErrInvalidSyntaxTheme = errors.New("openapi: invalid syntax theme")
)
