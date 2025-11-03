package compression

// WithLevel sets the gzip compression level.
// Valid values: 0 (no compression) to 9 (best compression).
// Default: gzip.DefaultCompression (-1, which is typically level 6)
//
// Example:
//
//	compression.New(compression.WithLevel(gzip.BestCompression))
func WithLevel(level int) Option {
	return func(cfg *config) {
		cfg.level = level
	}
}

// WithMinSize sets the minimum response size to compress (in bytes).
//
// DESIGN DECISION: This feature is intentionally NOT implemented.
//
// Why minSize is not implemented:
//   - Requires buffering entire response before compression decision
//   - Adds memory overhead (buffer per request)
//   - Adds latency (wait for full response before sending)
//   - Breaks streaming responses
//   - Modern networks handle small compressed payloads efficiently
//
// Alternative approaches (recommended):
//   - Use WithExcludePaths for small endpoints
//   - Use WithExcludeContentTypes for small data types
//   - Let CDN/reverse proxy handle compression (nginx, CloudFlare)
//   - Small responses compress quickly anyway (<100µs overhead)
//
// If you truly need minSize, implement at the reverse proxy level where
// buffering is already happening (nginx, CloudFlare, etc.)
//
// This option exists for API compatibility but is a no-op.
// Default: 1024 (1KB, not enforced)
//
// Example:
//
//	compression.New(compression.WithMinSize(2048)) // No effect (by design)
func WithMinSize(size int) Option {
	return func(cfg *config) {
		cfg.minSize = size
		// NOTE: Intentionally not implemented - see function documentation
	}
}

// WithExcludePaths sets paths that should not be compressed.
// Useful for endpoints that already serve compressed content or streaming responses.
//
// Example:
//
//	compression.New(compression.WithExcludePaths("/metrics", "/stream"))
func WithExcludePaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.excludePaths[path] = true
		}
	}
}

// WithExcludeExtensions sets file extensions that should not be compressed.
// Already compressed formats don't benefit from compression.
// Default: none (but should typically exclude .jpg, .png, .gif, .zip, etc.)
//
// Example:
//
//	compression.New(compression.WithExcludeExtensions(".jpg", ".png", ".gif", ".zip", ".gz"))
func WithExcludeExtensions(extensions ...string) Option {
	return func(cfg *config) {
		for _, ext := range extensions {
			cfg.excludeExtensions[ext] = true
		}
	}
}

// WithExcludeContentTypes sets content types that should not be compressed.
// Already compressed content types don't benefit from compression.
//
// Example:
//
//	compression.New(compression.WithExcludeContentTypes("image/jpeg", "image/png", "application/zip"))
func WithExcludeContentTypes(contentTypes ...string) Option {
	return func(cfg *config) {
		for _, ct := range contentTypes {
			cfg.excludeContentTypes[ct] = true
		}
	}
}
