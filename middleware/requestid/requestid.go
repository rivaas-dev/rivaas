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

package requestid

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"rivaas.dev/router"
)

type contextKey struct{}

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
		generator:     generateUUIDv7,
		allowClientID: true,
	}
}

// generateUUIDv7 generates a UUID v7 string for request IDs.
// UUID v7 is time-ordered and lexicographically sortable (RFC 9562).
func generateUUIDv7() string {
	return uuid.Must(uuid.NewV7()).String()
}

// ulidEntropy is a thread-safe entropy source for ULID generation.
// It provides monotonic ordering within the same millisecond.
var (
	ulidEntropy     = ulid.Monotonic(rand.Reader, 0)
	ulidEntropyLock sync.Mutex
)

// generateULID generates a ULID string for request IDs.
// ULID is time-ordered, lexicographically sortable, and has a compact 26-character representation.
func generateULID() string {
	ulidEntropyLock.Lock()
	defer ulidEntropyLock.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy).String()
}

// New returns a middleware that adds a unique request ID to each request.
// The request ID can be used for distributed tracing and log correlation.
//
// By default, UUID v7 is used for request ID generation. UUID v7 is time-ordered
// and lexicographically sortable (RFC 9562), making it ideal for debugging and
// log correlation.
//
// The middleware will:
// 1. Check if a request ID is already present in the configured header
// 2. Use the existing ID if allowed, or generate a new one
// 3. Set the request ID in the response header
//
// Basic usage (UUID v7 by default):
//
//	r := router.MustNew()
//	r.Use(requestid.New())
//
// Using ULID (shorter, 26 characters):
//
//	r.Use(requestid.New(requestid.WithULID()))
//
// Custom header name:
//
//	r.Use(requestid.New(
//	    requestid.WithHeader("X-Correlation-ID"),
//	))
//
// Custom generator:
//
//	r.Use(requestid.New(
//	    requestid.WithGenerator(func() string {
//	        return fmt.Sprintf("req-%d", time.Now().UnixNano())
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
		ctx := context.WithValue(c.Request.Context(), contextKey{}, requestID)
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
	if requestID, ok := c.Request.Context().Value(contextKey{}).(string); ok {
		return requestID
	}

	return ""
}
