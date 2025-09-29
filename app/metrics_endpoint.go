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
	"fmt"

	"rivaas.dev/router"
)

// MetricsEndpointOpts configures the metrics endpoint.
type MetricsEndpointOpts struct {
	// MountPath is the path where the metrics endpoint will be mounted.
	// Defaults to "/metrics" if not specified.
	MountPath string

	// Enabled determines if the metrics endpoint should be registered.
	// If false, no endpoint is registered regardless of other settings.
	Enabled bool
}

// WithMetricsEndpoint registers the metrics endpoint on the main application router.
//
// Design decision: WithMetricsEndpoint exists to support scenarios where metrics must be served
// on the same port as the application (e.g., Kubernetes environments with strict
// ingress rules, or when running behind a single load balancer).
//
// Default recommendation: Use the metrics package's auto-server feature (port :9090)
// to keep metrics separate from application traffic. This provides:
//   - Traffic isolation (metrics scraping doesn't compete with app requests)
//   - Independent rate limiting and access control
//   - Simpler firewall rules (metrics port can be internal-only)
//
// ⚠️ IMPORTANT: WithMetricsEndpoint conflicts with the metrics auto-server.
// You MUST disable auto-server with metrics.WithServerDisabled() to use this endpoint.
//
// The endpoint is only registered if:
//   - Enabled is true
//   - Metrics are configured on the app (via WithMetrics())
//   - Auto-server is disabled (via metrics.WithServerDisabled())
//
// WithMetricsEndpoint returns an error if:
//   - The path already exists (collision detection)
//   - Auto-server is still enabled (check via GetMetricsServerAddress())
//
// Example (with auto-server disabled):
//
//	_ = a.WithMetricsEndpoint(app.MetricsEndpointOpts{
//	    MountPath: "/metrics",
//	    Enabled:   a.HasMetrics(),
//	})
//
// Example (preferred approach - use auto-server instead):
//
//	// Don't call WithMetricsEndpoint at all
//	// Metrics will be available at http://localhost:9090/metrics
//	app.WithMetrics(
//	    metrics.WithPort(":9090"),
//	)
func (a *App) WithMetricsEndpoint(o MetricsEndpointOpts) error {
	if !o.Enabled {
		return nil // Silently skip if disabled
	}

	if a.metrics == nil {
		return nil // Silently skip if metrics not configured
	}

	// Check if auto-server is enabled - this creates a conflict
	if serverAddr := a.GetMetricsServerAddress(); serverAddr != "" {
		return fmt.Errorf(
			"cannot use WithMetricsEndpoint: metrics auto-server is enabled on %s. "+
				"Either disable auto-server with metrics.WithServerDisabled() or don't call WithMetricsEndpoint",
			serverAddr,
		)
	}

	path := o.MountPath
	if path == "" {
		path = "/metrics"
	}

	// Check for route collision
	if a.router.RouteExists("GET", path) {
		return fmt.Errorf("route already registered: GET %s", path)
	}

	// Get the metrics handler
	handler, err := a.GetMetricsHandler()
	if err != nil {
		return fmt.Errorf("failed to get metrics handler: %w", err)
	}

	// Register the endpoint
	a.Router().GET(path, func(c *router.Context) {
		handler.ServeHTTP(c.Response, c.Request)
	})

	return nil
}

// HasMetrics returns true if metrics are configured and enabled.
//
// Example:
//
//	if app.HasMetrics() {
//	    _ = app.WithMetricsEndpoint(app.MetricsEndpointOpts{
//	        MountPath: "/metrics",
//	        Enabled:   true,
//	    })
//	}
func (a *App) HasMetrics() bool {
	return a.metrics != nil
}
