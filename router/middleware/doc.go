/*
Package middleware provides production-ready HTTP middlewares for rivaas/router.

This package contains shared types and constants. Each middleware is provided
in its own sub-package for better organization and cleaner imports.

# Available Middlewares

Each middleware is in its own package:

Security:
  - security: Sets security headers (HSTS, CSP, X-Frame-Options, etc.)
  - cors: Cross-Origin Resource Sharing configuration
  - basicauth: HTTP Basic Authentication

Observability:
  - accesslog: Structured HTTP access logging with sampling and filtering
  - requestid: Request ID generation and tracking for distributed tracing

Reliability:
  - recovery: Panic recovery with graceful error handling
  - timeout: Request timeout handling
  - ratelimit: Token bucket rate limiting per client
  - bodylimit: Request body size limiting to prevent DoS attacks

Performance:
  - compression: Gzip/Deflate response compression

# Usage Examples

Basic setup with common middlewares:

	import (
	    "rivaas.dev/logging"
	    "rivaas.dev/router"
	    "rivaas.dev/router/middleware/accesslog"
	    "rivaas.dev/router/middleware/requestid"
	    "rivaas.dev/router/middleware/recovery"
	    "rivaas.dev/router/middleware/security"
	)

	r := router.New(
	    logging.WithLogging(
	        logging.WithConsoleHandler(),
	        logging.WithDebugLevel(),
	    ),
	)
	r.Use(requestid.New())
	r.Use(accesslog.New())
	r.Use(recovery.New())
	r.Use(security.New())

Rate limiting with custom configuration:

	import "rivaas.dev/router/middleware/ratelimit"

	r.Use(ratelimit.New(
	    ratelimit.WithRequestsPerSecond(100),
	    ratelimit.WithBurst(20),
	))

Production setup:

	import (
	    "rivaas.dev/logging"
	    "rivaas.dev/router"
	    "rivaas.dev/router/middleware/accesslog"
	    "rivaas.dev/router/middleware/requestid"
	    "rivaas.dev/router/middleware/recovery"
	    "rivaas.dev/router/middleware/security"
	    "rivaas.dev/router/middleware/cors"
	    "rivaas.dev/router/middleware/ratelimit"
	    "rivaas.dev/router/middleware/compression"
	)

	r := router.New(
	    logging.WithLogging(
	        logging.WithJSONHandler(),
	        logging.WithLevel(logging.LevelInfo),
	    ),
	)

	// Observability
	r.Use(requestid.New())
	r.Use(accesslog.New(
	    accesslog.WithExcludePaths("/health"),
	))

	// Security
	r.Use(security.New())
	r.Use(cors.New(
	    cors.WithAllowedOrigins("https://example.com"),
	))

	// Reliability
	r.Use(recovery.New())
	r.Use(ratelimit.New(
	    ratelimit.WithRequestsPerSecond(1000),
	))

	// Performance
	r.Use(compression.New())

# Middleware Ordering

Recommended middleware order for optimal behavior:

 1. recovery          - Catch panics from all other middlewares
 2. requestid         - Generate ID early for logging
 3. accesslog         - Log all requests including failed ones
 4. security/cors     - Set security headers early
 5. ratelimit         - Reject excessive requests before processing
 6. timeout           - Set time limits for downstream processing
 7. compression       - Compress responses
 8. basicauth         - Authenticate after rate limiting
 9. Application logic - Your handlers

# Context Values

Some middlewares store values in the request context for use by other middlewares
or handlers. Use the provided getter functions to access these safely:

	import "rivaas.dev/router/middleware/requestid"
	import "rivaas.dev/router/middleware/basicauth"

	requestID := requestid.Get(c)
	username := basicauth.GetUsername(c)

Context keys are exported from this package as middleware.RequestIDKey and
middleware.AuthUsernameKey for advanced use cases.

# Thread Safety

All middlewares are safe for concurrent use. Rate limiting uses internal
synchronization to handle concurrent requests safely.
*/
package middleware
