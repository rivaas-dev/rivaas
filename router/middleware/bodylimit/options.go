package bodylimit

import "rivaas.dev/router"

// WithLimit sets the maximum allowed body size in bytes.
// Default: 2MB (2 * 1024 * 1024 bytes)
//
// Example:
//
//	// Limit to 1MB
//	bodylimit.New(bodylimit.WithLimit(1024 * 1024))
//
//	// Limit to 10MB
//	bodylimit.New(bodylimit.WithLimit(10 * 1024 * 1024))
func WithLimit(size int64) Option {
	return func(cfg *config) {
		if size <= 0 {
			panic("body limit must be positive")
		}
		cfg.limit = size
	}
}

// WithErrorHandler sets a custom handler for when body limit is exceeded.
// The handler receives both the context and the configured limit for flexibility.
// Default: Returns 413 Request Entity Too Large with JSON error
//
// Example:
//
//	bodylimit.New(
//	    bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
//	        c.String(413, fmt.Sprintf("Request body exceeds maximum allowed size of %d bytes", limit))
//	    }),
//	)
func WithErrorHandler(handler func(c *router.Context, limit int64)) Option {
	return func(cfg *config) {
		cfg.errorHandler = handler
	}
}

// WithSkipPaths sets paths that should not have body limit applied.
// Useful for endpoints that need to accept large uploads.
//
// Example:
//
//	bodylimit.New(
//	    bodylimit.WithSkipPaths("/upload", "/files"),
//	)
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}
