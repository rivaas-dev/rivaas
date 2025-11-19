// Package timeout provides middleware for setting request timeouts.
package timeout

import "rivaas.dev/router"

// WithHandler sets a custom handler for timeout errors.
// This handler is called when a request exceeds the timeout duration.
//
// Example:
//
//	timeout.New(30*time.Second,
//	    timeout.WithHandler(func(c *router.Context) {
//	        c.JSON(http.StatusRequestTimeout, map[string]string{
//	            "error": "Request took too long",
//	            "request_id": c.Response.Header().Get("X-Request-ID"),
//	        })
//	    }),
//	)
func WithHandler(handler func(c *router.Context)) Option {
	return func(cfg *config) {
		cfg.errorHandler = handler
	}
}

// WithSkipPaths sets paths that should not have timeout applied.
// Useful for long-running endpoints like streaming or webhooks.
//
// Example:
//
//	timeout.New(30*time.Second,
//	    timeout.WithSkipPaths("/stream", "/webhook"),
//	)
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}
