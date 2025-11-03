package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

// Option defines functional options for requestid middleware configuration.
type Option func(*config)

// config holds the configuration for the requestid middleware.
type config struct {
	// headerName is the name of the header to use for the request ID
	headerName string

	// generator is the function used to generate new request IDs
	generator func() string

	// allowClientID allows using request IDs provided by clients
	allowClientID bool
}

// defaultConfig returns the default configuration for requestid middleware.
func defaultConfig() *config {
	return &config{
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

// New returns a middleware that adds a unique request ID to each request.
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
//	r.Use(requestid.New())
//
// Custom header name:
//
//	r.Use(requestid.New(
//	    requestid.WithHeader("X-Correlation-ID"),
//	))
//
// With UUID generator:
//
//	import "github.com/google/uuid"
//
//	r.Use(requestid.New(
//	    requestid.WithGenerator(func() string {
//	        return uuid.New().String()
//	    }),
//	))
//
// Disable client IDs:
//
//	r.Use(requestid.New(
//	    requestid.WithAllowClientID(false),
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
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
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
		ctx := context.WithValue(c.Request.Context(), middleware.RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Continue processing
		c.Next()
	}
}

// Get retrieves the request ID from the context.
// Returns an empty string if no request ID has been set.
//
// Example:
//
//	func handler(c *router.Context) {
//	    requestID := requestid.Get(c)
//	    log.Printf("Processing request %s", requestID)
//	}
func Get(c *router.Context) string {
	if requestID, ok := c.Request.Context().Value(middleware.RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}
