package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"rivaas.dev/router"
)

// CheckFunc defines a function that performs a health or readiness check.
// The function should return nil if the check passes, or an error if it fails.
// The context may be cancelled if the check takes too long.
type CheckFunc func(ctx context.Context) error

// StandardEndpointsOpts configures the standard health and readiness endpoints.
type StandardEndpointsOpts struct {
	// MountPrefix is the prefix under which to mount all endpoints.
	// Defaults to empty string (mounts at root).
	// Example: "/_system" mounts endpoints at "/_system/healthz" and "/_system/readyz"
	MountPrefix string

	// Liveness checks run to determine if the process is alive.
	// These should be fast, dependency-free checks (e.g., can create goroutines).
	// If any liveness check fails, /healthz returns 503.
	// If no liveness checks are provided, /healthz always returns 200.
	Liveness map[string]CheckFunc

	// Readiness checks run to determine if the service is ready to serve traffic.
	// These check external dependencies (database, cache, external APIs).
	// If any readiness check fails, /readyz returns 503 with problem details.
	// If no readiness checks are provided, /readyz always returns 204.
	Readiness map[string]CheckFunc

	// Timeout is the maximum time to wait for each check to complete.
	// Each check runs with this timeout applied via context.WithTimeout.
	// Defaults to 1 second if not specified.
	Timeout time.Duration
}

// WithStandardEndpoints registers standard health and readiness endpoints.
//
// Endpoints registered:
//   - GET /healthz (or /{MountPrefix}/healthz) - Liveness probe
//     Returns 200 "ok" if all liveness checks pass, 503 if any fail
//   - GET /readyz (or /{MountPrefix}/readyz) - Readiness probe
//     Returns 204 if all readiness checks pass, 503 with problem details if any fail
//
// Returns an error if any endpoint path already exists (collision detection).
//
// Example:
//
//	_ = a.WithStandardEndpoints(app.StandardEndpointsOpts{
//	    MountPrefix: "",
//	    Timeout:     800 * time.Millisecond,
//	    Liveness: map[string]app.CheckFunc{
//	        "process": func(ctx context.Context) error {
//	            // Quick check: process is alive
//	            return nil
//	        },
//	    },
//	    Readiness: map[string]app.CheckFunc{
//	        "database": func(ctx context.Context) error {
//	            return db.PingContext(ctx)
//	        },
//	        "cache": func(ctx context.Context) error {
//	            return redis.Ping(ctx).Err()
//	        },
//	    },
//	})
func (a *App) WithStandardEndpoints(o StandardEndpointsOpts) error {
	prefix := o.MountPrefix
	if prefix == "" {
		prefix = ""
	}

	// Check for route collisions
	healthzPath := prefix + "/healthz"
	readyzPath := prefix + "/readyz"

	if a.router.RouteExists("GET", healthzPath) {
		return fmt.Errorf("route already registered: GET %s", healthzPath)
	}
	if a.router.RouteExists("GET", readyzPath) {
		return fmt.Errorf("route already registered: GET %s", readyzPath)
	}

	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	// GET /healthz - Liveness probe (process health, no external deps)
	a.Router().GET(healthzPath, func(c *router.Context) {
		c.Header("Cache-Control", "no-store")

		// No liveness checks = always healthy (process is running)
		if len(o.Liveness) == 0 {
			_ = c.String(http.StatusOK, "ok")
			return
		}

		// Run liveness checks concurrently
		ctx := c.Request.Context()
		failures := runChecks(ctx, o.Liveness, timeout)

		if len(failures) > 0 {
			// 503 with problem details
			_ = c.Problem(
				http.StatusServiceUnavailable,
				c.ProblemType("not-healthy"),
				"Service Not Healthy",
				"One or more liveness checks failed.",
				map[string]any{"checks": failures},
			)
			return
		}

		_ = c.String(http.StatusOK, "ok")
	})

	// GET /readyz - Readiness probe (external deps: db, cache, otel)
	a.Router().GET(readyzPath, func(c *router.Context) {
		c.Header("Cache-Control", "no-store")

		// No readiness checks = always ready
		if len(o.Readiness) == 0 {
			c.NoContent()
			return
		}

		ctx := c.Request.Context()
		failures := runChecks(ctx, o.Readiness, timeout)

		if len(failures) > 0 {
			_ = c.Problem(
				http.StatusServiceUnavailable,
				c.ProblemType("not-ready"),
				"Service Not Ready",
				"One or more dependencies failed readiness.",
				map[string]any{"checks": failures},
			)
			return
		}

		c.NoContent()
	})

	return nil
}

// runChecks executes all checks concurrently with timeout and returns failures.
//
// Concurrency design: Launches one goroutine per check to minimize total latency.
// For N checks each taking T seconds, sequential execution takes N*T seconds,
// while concurrent execution takes max(T) seconds.
//
// Example: 5 checks (database, redis, etc.) each taking 200ms sequentially = 1000ms.
// Concurrent execution completes in ~200ms, 5x faster, improving startup/probe times.
//
// Timeout enforcement: Each check runs with an independent context.WithTimeout to
// prevent one slow dependency from blocking the entire health check. This ensures
// the probe responds within predictable time bounds.
//
// Returns a map of check name to error message for any failed checks.
func runChecks(ctx context.Context, checks map[string]CheckFunc, timeout time.Duration) map[string]string {
	type result struct {
		name string
		err  error
	}

	results := make(chan result, len(checks))

	for name, fn := range checks {
		name, fn := name, fn // Capture for goroutine
		go func() {
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			results <- result{name, fn(checkCtx)}
		}()
	}

	failures := make(map[string]string)
	for i := 0; i < len(checks); i++ {
		r := <-results
		if r.err != nil {
			failures[r.name] = r.err.Error()
		}
	}

	return failures
}
