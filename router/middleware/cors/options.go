package cors

// WithAllowedOrigins sets the list of allowed origins.
// Use this for specific origins like ["https://example.com", "https://app.example.com"].
//
// Example:
//
//	cors.New(cors.WithAllowedOrigins("https://example.com"))
func WithAllowedOrigins(origins ...string) Option {
	return func(cfg *config) {
		cfg.allowedOrigins = origins
		cfg.allowAllOrigins = false
	}
}

// WithAllowAllOrigins allows all origins by setting Access-Control-Allow-Origin: *.
// WARNING: This is insecure and should only be used for public APIs.
//
// Example:
//
//	cors.New(cors.WithAllowAllOrigins(true))
func WithAllowAllOrigins(allow bool) Option {
	return func(cfg *config) {
		cfg.allowAllOrigins = allow
	}
}

// WithAllowedMethods sets the list of allowed HTTP methods.
// Default: ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]
//
// Example:
//
//	cors.New(cors.WithAllowedMethods("GET", "POST"))
func WithAllowedMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.allowedMethods = methods
	}
}

// WithAllowedHeaders sets the list of allowed request headers.
// Default: ["Origin", "Content-Type", "Accept", "Authorization"]
//
// Example:
//
//	cors.New(cors.WithAllowedHeaders("Content-Type", "X-Custom-Header"))
func WithAllowedHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.allowedHeaders = headers
	}
}

// WithExposedHeaders sets the list of headers exposed to the client.
// These headers can be accessed by the client-side JavaScript.
//
// Example:
//
//	cors.New(cors.WithExposedHeaders("X-Request-ID", "X-Rate-Limit"))
func WithExposedHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.exposedHeaders = headers
	}
}

// WithAllowCredentials enables credentials (cookies, authorization headers, TLS certificates).
// When true, Access-Control-Allow-Origin cannot be "*".
// Default: false
//
// Example:
//
//	cors.New(cors.WithAllowCredentials(true))
func WithAllowCredentials(allow bool) Option {
	return func(cfg *config) {
		cfg.allowCredentials = allow
	}
}

// WithMaxAge sets the max age for preflight cache in seconds.
// Default: 3600 (1 hour)
//
// Example:
//
//	cors.New(cors.WithMaxAge(7200)) // 2 hours
func WithMaxAge(seconds int) Option {
	return func(cfg *config) {
		cfg.maxAge = seconds
	}
}

// WithAllowOriginFunc sets a custom function to validate origins dynamically.
// This is useful for pattern matching or database lookups.
//
// Example:
//
//	cors.New(cors.WithAllowOriginFunc(func(origin string) bool {
//	    return strings.HasSuffix(origin, ".example.com")
//	}))
func WithAllowOriginFunc(fn func(origin string) bool) Option {
	return func(cfg *config) {
		cfg.allowOriginFunc = fn
	}
}
