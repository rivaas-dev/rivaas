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
	// These should be dependency-free checks that complete without external dependencies (e.g., can create goroutines).
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
// WithStandardEndpoints returns an error if any endpoint path already exists (collision detection).
//
// Example:
//
//	_ = a.WithStandardEndpoints(app.StandardEndpointsOpts{
//	    MountPrefix: "",
//	    Timeout:     800 * time.Millisecond,
//	    Liveness: map[string]app.CheckFunc{
//	        "process": func(ctx context.Context) error {
//	            // Dependency-free check: process is alive
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
			c.String(http.StatusOK, "ok")
			return
		}

		// Run liveness checks concurrently
		ctx := c.Request.Context()
		failures := runChecks(ctx, o.Liveness, timeout)

		if len(failures) > 0 {
			// 503 response - error formatting handled by app.Context.Error() if wrapped
			c.WriteErrorResponse(http.StatusServiceUnavailable, "Service Not Healthy: One or more liveness checks failed")
			return
		}

		c.String(http.StatusOK, "ok")
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
			// 503 response - error formatting handled by app.Context.Error() if wrapped
			c.WriteErrorResponse(http.StatusServiceUnavailable, "Service Not Ready: One or more dependencies failed readiness")
			return
		}

		c.NoContent()
	})

	return nil
}

// runChecks executes all checks concurrently with timeout and returns failures.
//
// runChecks runs checks concurrently, with each check launched in its own goroutine.
// This allows multiple checks to execute in parallel rather than sequentially.
//
// Timeout enforcement: Each check runs with an independent context.WithTimeout to
// prevent one slow dependency from blocking the entire health check. This ensures
// the probe responds within the configured timeout.
//
// runChecks returns a map of check name to error message for any failed checks.
func runChecks(ctx context.Context, checks map[string]CheckFunc, timeout time.Duration) map[string]string {
	type result struct {
		name string
		err  error
	}

	results := make(chan result, len(checks))

	for name, fn := range checks {
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
