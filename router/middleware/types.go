package middleware

// contextKey is a type for context keys to avoid collisions with other packages.
// Using a custom type prevents conflicts with string-based context keys.
type contextKey string

// Context keys used across middlewares.
// These are defined here to ensure uniqueness and prevent conflicts.
// Exported for use by middleware sub-packages.
const (
	// RequestIDKey is the context key for storing request ID.
	// Used by: RequestID middleware (sets it) and Logger middleware (reads it).
	RequestIDKey contextKey = "middleware.request_id"

	// AuthUsernameKey is the context key for storing authenticated username.
	// Used by: BasicAuth middleware (sets it).
	AuthUsernameKey contextKey = "middleware.auth_username"
)
