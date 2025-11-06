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
// ⚠️ IMPORTANT: This function conflicts with the metrics package's auto-server feature.
// By default, metrics.New() automatically starts a separate HTTP server on port :9090.
// To use this function, you MUST disable the auto-server:
//
//	app.WithMetrics(
//	    metrics.WithServerDisabled(), // Required!
//	)
//
// The endpoint is only registered if:
//   - Enabled is true
//   - Metrics are configured on the app (via WithMetrics())
//   - Auto-server is disabled (via metrics.WithServerDisabled())
//
// Returns an error if:
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
func (a *App) HasMetrics() bool {
	return a.metrics != nil
}
