package middleware

// ContextKey is a type for context keys to avoid collisions with other packages.
// Using a custom type prevents conflicts with string-based context keys.
type ContextKey string

// Context keys used across middlewares.
// These are defined here to ensure uniqueness and prevent conflicts.
// Exported for use by middleware sub-packages.
const (
	// RequestIDKey is the context key for storing request ID.
	// Used by: RequestID middleware (sets it) and Logger middleware (reads it).
	RequestIDKey ContextKey = "middleware.request_id"

	// AuthUsernameKey is the context key for storing authenticated username.
	// Used by: BasicAuth middleware (sets it).
	AuthUsernameKey ContextKey = "middleware.auth_username"

	// OriginalMethodKey is the context key for storing the original HTTP method
	// before method override. Used by: MethodOverride middleware.
	OriginalMethodKey ContextKey = "middleware.original_method"
)
