package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"rivaas.dev/router"
)

// requestIDKey is the context key for storing request ID.
const requestIDKey contextKey = "request.id"

// RequestIDOption defines functional options for RequestID middleware configuration.
type RequestIDOption func(*requestIDConfig)

// requestIDConfig holds the configuration for the RequestID middleware.
type requestIDConfig struct {
	// headerName is the name of the header to use for the request ID
	headerName string

	// generator is the function used to generate new request IDs
	generator func() string

	// allowClientID allows using request IDs provided by clients
	allowClientID bool
}

// defaultRequestIDConfig returns the default configuration for RequestID middleware.
func defaultRequestIDConfig() *requestIDConfig {
	return &requestIDConfig{
		headerName:    "X-Request-ID",
		generator:     generateRandomID,
		allowClientID: true,
	}
}

// generateRandomID generates a random hex string for request IDs.
func generateRandomID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", mustGetTimestamp())
	}
	return hex.EncodeToString(bytes)
}

// mustGetTimestamp returns current timestamp in nanoseconds.
// This is a fallback for ID generation if crypto/rand fails.
func mustGetTimestamp() int64 {
	return time.Now().UnixNano()
}

// WithRequestIDHeader sets the header name for the request ID.
// Default: "X-Request-ID"
//
// Example:
//
//	middleware.RequestID(middleware.WithRequestIDHeader("X-Trace-ID"))
func WithRequestIDHeader(headerName string) RequestIDOption {
	return func(cfg *requestIDConfig) {
		cfg.headerName = headerName
	}
}

// WithRequestIDGenerator sets a custom function to generate request IDs.
// The generator function should return a unique string for each call.
//
// Example with UUID:
//
//	import "github.com/google/uuid"
//
//	middleware.RequestID(middleware.WithRequestIDGenerator(func() string {
//	    return uuid.New().String()
//	}))
//
// Example with custom format:
//
//	middleware.RequestID(middleware.WithRequestIDGenerator(func() string {
//	    return fmt.Sprintf("req-%d-%s", time.Now().Unix(), randomString(8))
//	}))
func WithRequestIDGenerator(generator func() string) RequestIDOption {
	return func(cfg *requestIDConfig) {
		cfg.generator = generator
	}
}

// WithAllowClientID controls whether to accept request IDs from clients.
// When true, if the client provides a request ID in the header, it will be used.
// When false, always generate a new request ID regardless of client input.
// Default: true
//
// Security note: Set to false if you need to ensure all request IDs are server-generated.
//
// Example:
//
//	middleware.RequestID(middleware.WithAllowClientID(false))
func WithAllowClientID(allow bool) RequestIDOption {
	return func(cfg *requestIDConfig) {
		cfg.allowClientID = allow
	}
}

// RequestID returns a middleware that adds a unique request ID to each request.
// The request ID can be used for distributed tracing and log correlation.
//
// The middleware will:
// 1. Check if a request ID is already present in the configured header
// 2. Use the existing ID if allowed, or generate a new one
// 3. Set the request ID in the response header
//
// Basic usage:
//
//	r := router.New()
//	r.Use(middleware.RequestID())
//
// Custom header name:
//
//	r.Use(middleware.RequestID(
//	    middleware.WithRequestIDHeader("X-Correlation-ID"),
//	))
//
// With UUID generator:
//
//	import "github.com/google/uuid"
//
//	r.Use(middleware.RequestID(
//	    middleware.WithRequestIDGenerator(func() string {
//	        return uuid.New().String()
//	    }),
//	))
//
// Disable client IDs:
//
//	r.Use(middleware.RequestID(
//	    middleware.WithAllowClientID(false),
//	))
//
// Accessing the request ID in handlers:
//
//	r.GET("/users/:id", func(c *router.Context) {
//	    requestID := c.Response.Header().Get("X-Request-ID")
//	    // Use requestID for logging, tracing, etc.
//	})
//
// Performance: This middleware has minimal overhead (~100ns per request).
// ID generation adds ~500ns when generating new IDs.
func RequestID(opts ...RequestIDOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultRequestIDConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		var requestID string

		// Check for existing request ID if allowed
		if cfg.allowClientID {
			requestID = c.Request.Header.Get(cfg.headerName)
		}

		// Generate new ID if none exists or client IDs are disabled
		if requestID == "" {
			requestID = cfg.generator()
		}

		// Set request ID in response header
		c.Response.Header().Set(cfg.headerName, requestID)

		// Store request ID in context for use by other middleware (e.g., logger)
		ctx := context.WithValue(c.Request.Context(), requestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Continue processing
		c.Next()
	}
}

// GetRequestID retrieves the request ID from the context.
// Returns an empty string if no request ID has been set.
//
// Example:
//
//	func handler(c *router.Context) {
//	    requestID := middleware.GetRequestID(c)
//	    log.Printf("Processing request %s", requestID)
//	}
func GetRequestID(c *router.Context) string {
	if requestID, ok := c.Request.Context().Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}
