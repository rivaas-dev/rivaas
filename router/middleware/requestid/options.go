// Package requestid provides middleware for generating and managing
// unique request IDs for distributed tracing and request correlation.
package requestid

// WithHeader sets the header name for the request ID.
// Default: "X-Request-ID"
//
// Example:
//
//	requestid.New(requestid.WithHeader("X-Trace-ID"))
func WithHeader(headerName string) Option {
	return func(cfg *config) {
		cfg.headerName = headerName
	}
}

// WithGenerator sets a custom function to generate request IDs.
// The generator function should return a unique string for each call.
//
// Example with UUID:
//
//	import "github.com/google/uuid"
//
//	requestid.New(requestid.WithGenerator(func() string {
//	    return uuid.New().String()
//	}))
//
// Example with custom format:
//
//	requestid.New(requestid.WithGenerator(func() string {
//	    return fmt.Sprintf("req-%d-%s", time.Now().Unix(), randomString(8))
//	}))
func WithGenerator(generator func() string) Option {
	return func(cfg *config) {
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
//	requestid.New(requestid.WithAllowClientID(false))
func WithAllowClientID(allow bool) Option {
	return func(cfg *config) {
		cfg.allowClientID = allow
	}
}
