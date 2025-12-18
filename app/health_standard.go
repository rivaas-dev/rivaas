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
	"rivaas.dev/router/route"
)

// registerHealthEndpoints registers health endpoints based on the provided settings.
// This is called internally by app.New() when health endpoints are configured.
func (a *App) registerHealthEndpoints(s *healthSettings) error {
	// Build full paths
	healthzPath := s.prefix + s.healthzPath
	readyzPath := s.prefix + s.readyzPath

	// Check for route collisions
	if a.router.RouteExists("GET", healthzPath) {
		return fmt.Errorf("route already registered: GET %s", healthzPath)
	}
	if a.router.RouteExists("GET", readyzPath) {
		return fmt.Errorf("route already registered: GET %s", readyzPath)
	}

	timeout := s.timeout
	if timeout <= 0 {
		timeout = time.Second
	}

	// GET /healthz - Liveness probe (process health, no external deps)
	a.Router().GET(healthzPath, func(c *router.Context) {
		c.Header("Cache-Control", "no-store")

		// No liveness checks = always healthy (process is running)
		if len(s.liveness) == 0 {
			if err := c.String(http.StatusOK, "ok"); err != nil {
				c.Logger().Error("failed to write healthz response", "err", err)
			}

			return
		}

		// Run liveness checks concurrently
		ctx := c.Request.Context()
		failures := runChecks(ctx, s.liveness, timeout)

		if len(failures) > 0 {
			// 503 response - error formatting handled by app.Context.Error() if wrapped
			c.WriteErrorResponse(http.StatusServiceUnavailable, "Service Not Healthy: One or more liveness checks failed")
			return
		}

		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write healthz response", "err", err)
		}
	})

	// Update route info to show builtin handler name
	a.router.UpdateRouteInfo("GET", healthzPath, "", func(info *route.Info) {
		info.HandlerName = "[builtin] health"
	})

	// GET /readyz - Readiness probe (external deps: db, cache, otel)
	a.Router().GET(readyzPath, func(c *router.Context) {
		c.Header("Cache-Control", "no-store")

		// No readiness checks = always ready
		if len(s.readiness) == 0 {
			c.NoContent()
			return
		}

		ctx := c.Request.Context()
		failures := runChecks(ctx, s.readiness, timeout)

		if len(failures) > 0 {
			// 503 response - error formatting handled by app.Context.Error() if wrapped
			c.WriteErrorResponse(http.StatusServiceUnavailable, "Service Not Ready: One or more dependencies failed readiness")
			return
		}

		c.NoContent()
	})

	// Update route info to show builtin handler name
	a.router.UpdateRouteInfo("GET", readyzPath, "", func(info *route.Info) {
		info.HandlerName = "[builtin] readiness"
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
	for range len(checks) {
		r := <-results
		if r.err != nil {
			failures[r.name] = r.err.Error()
		}
	}

	return failures
}
