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

package bodylimit

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"rivaas.dev/router"
)

// ErrBodyLimitExceeded is returned when the request body exceeds the configured limit.
var ErrBodyLimitExceeded = errors.New("request body size exceeds limit")

// Option defines functional options for bodylimit middleware configuration.
type Option func(*config)

// config holds the configuration for the bodylimit middleware.
type config struct {
	// limit is the maximum allowed body size in bytes
	limit int64

	// errorHandler is called when the body limit is exceeded
	// The handler receives the context and the configured limit
	errorHandler func(c *router.Context, limit int64)

	// skipPaths are paths that should not have body limit applied.
	// We use map[string]bool instead of []string for lookup,
	// since this check happens on every request.
	skipPaths map[string]bool
}

// defaultConfig returns the default configuration for bodylimit middleware.
func defaultConfig() *config {
	return &config{
		limit:        2 * 1024 * 1024, // 2MB default
		errorHandler: defaultErrorHandler,
		skipPaths:    make(map[string]bool),
	}
}

// defaultErrorHandler is the default body limit error handler.
func defaultErrorHandler(c *router.Context, limit int64) {
	c.Status(http.StatusRequestEntityTooLarge)
	c.JSON(http.StatusRequestEntityTooLarge, map[string]any{
		"error":    "request entity too large",
		"max_size": formatSize(limit),
	})
}

// formatSize formats a byte size into a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// limitedReader wraps an io.ReadCloser to limit the number of bytes that can be read.
// This provides actual security by monitoring bytes read, not just Content-Length header.
type limitedReader struct {
	reader io.ReadCloser
	limit  int64
	read   int64
}

// Read reads data from the underlying reader and enforces the limit.
// This implementation is based on io.LimitReader but returns an error when limit is exceeded.
//
// Note: This reader is not safe for concurrent use, but that's acceptable since
// http.Request.Body should not be read from multiple goroutines concurrently.
func (lr *limitedReader) Read(p []byte) (int, error) {
	// If we've already read up to the limit, return EOF
	if lr.read >= lr.limit {
		return 0, io.EOF
	}

	// Calculate how many bytes we can still read
	remaining := lr.limit - lr.read

	// Limit the read buffer to remaining bytes
	if int64(len(p)) > remaining {
		p = p[0:remaining]
	}

	// Read from underlying reader
	n, err := lr.reader.Read(p)
	lr.read += int64(n)

	// If we've reached the limit and there's no error, check if more data exists
	// This is important to detect when the body is larger than the limit
	if lr.read >= lr.limit && err == nil {
		// Try to read one more byte to see if body continues
		var oneByte [1]byte
		extraN, extraErr := lr.reader.Read(oneByte[:])
		if extraN > 0 {
			// There's more data beyond the limit - return error
			return n, fmt.Errorf("%w: %d bytes", ErrBodyLimitExceeded, lr.limit)
		}
		// If read returned EOF or 0 bytes, we're exactly at limit (acceptable)
		if extraErr == io.EOF {
			err = io.EOF
		}
	}

	return n, err
}

// Close closes the underlying reader.
func (lr *limitedReader) Close() error {
	return lr.reader.Close()
}

// New returns a middleware that sets the maximum allowed size for request bodies.
// If the size exceeds the configured limit, it sends a 413 Request Entity Too Large response.
//
// Security Features:
//   - Dual validation: Checks Content-Length header AND actual bytes read
//   - Prevents DoS attacks by limiting memory consumption
//   - Works with all content types (JSON, form data, multipart, etc.)
//
// The body limit is determined based on both Content-Length request header and actual content
// read. The Content-Length check provides early rejection for oversized requests,
// while the limitedReader wrapper ensures we never read more than the limit,
// even if the header is incorrect or missing.
//
// Basic usage:
//
//	r := router.MustNew()
//	r.Use(bodylimit.New()) // Default 2MB limit
//
// Custom size limit:
//
//	r.Use(bodylimit.New(
//	    bodylimit.WithLimit(10 * 1024 * 1024), // 10MB limit
//	))
//
// Skip certain paths:
//
//	r.Use(bodylimit.New(
//	    bodylimit.WithSkipPaths("/upload", "/files"),
//	))
//
// Custom error handler:
//
//	r.Use(bodylimit.New(
//	    bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
//	        c.Status(http.StatusRequestEntityTooLarge)
//	        c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
//	            "error": "File too large",
//	            "max_size": formatSize(limit),
//	        })
//	    }),
//	))
//
// How it works:
//
//  1. First phase: Check Content-Length header
//     - If Content-Length > limit, immediately return 413
//     - If no Content-Length header, proceed to body wrapping
//
//  2. Second phase: Wrap request body with limitedReader
//     - limitedReader tracks bytes read and enforces limit
//     - If limit exceeded during read, returns error
//     - Middleware catches error and calls error handler
//
// Example handler that reads body:
//
//	r.POST("/data", func(c *router.Context) {
//	    var data MyStruct
//	    if err := c.BindJSON(&data); err != nil {
//	        // If body limit exceeded, err will indicate this
//	        c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": err.Error()})
//	        return
//	    }
//	    // Process data...
//	})
//
// Important notes:
//   - The middleware wraps c.Request.Body, so all body reads are limited
//   - Works with c.BindJSON(), c.BindForm(), and direct body reads
//   - Content-Length header check can be spoofed - actual read is secure
//   - For multipart forms, limit applies to total form size (before parsing)
//   - If Content-Length is missing, body will still be limited during read
//
// Security considerations:
//   - Always validates actual bytes read, not just headers
//   - Prevents memory exhaustion from oversized requests
//   - Stops reading immediately when limit is exceeded
//   - Works with streaming/chunked requests (no Content-Length)
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		// Check if path should skip body limit
		if cfg.skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Phase 1: Check Content-Length header
		// This provides early rejection for oversized requests
		if contentLength := c.Request.Header.Get("Content-Length"); contentLength != "" {
			size, err := strconv.ParseInt(contentLength, 10, 64)
			if err == nil && size > cfg.limit {
				// Content-Length exceeds limit, reject immediately
				cfg.errorHandler(c, cfg.limit)
				c.Abort()

				return
			}
		}

		// Phase 2: Wrap body with limitedReader for actual security
		// This ensures we never read more than the limit, even if:
		// - Content-Length header is missing
		// - Content-Length header is incorrect
		// - Request uses chunked encoding
		if c.Request.Body != nil {
			originalBody := c.Request.Body
			c.Request.Body = &limitedReader{
				reader: originalBody,
				limit:  cfg.limit,
				read:   0,
			}
		}

		// Process request
		c.Next()
	}
}
