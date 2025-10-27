package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// MetricsRecorder interface for integration with router.
// This is defined locally to avoid circular dependencies.
// It must match the MetricsRecorder interface in router/interfaces.go.
type MetricsRecorder interface {
	// RecordMetric records a custom histogram metric with the given name and value.
	RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// IncrementCounter increments a custom counter metric with the given name.
	IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue)

	// SetGauge sets a custom gauge metric with the given name and value.
	SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// RecordRouteRegistration records route registration metrics.
	RecordRouteRegistration(ctx context.Context, method, path string)

	// RecordContextPoolHit records a context pool hit (reused context).
	RecordContextPoolHit(ctx context.Context)

	// RecordContextPoolMiss records a context pool miss (new allocation).
	RecordContextPoolMiss(ctx context.Context)

	// RecordConstraintFailure records a route constraint validation failure.
	RecordConstraintFailure(ctx context.Context, constraint string, attributes ...attribute.KeyValue)

	// StartRequest initializes metrics collection for a request.
	// Returns a request metrics object (*requestMetrics) that should be passed to FinishRequest.
	// The return type is interface{} to avoid circular dependencies with the router package.
	// Returns nil if context is cancelled, path is excluded, or metrics are disabled.
	StartRequest(ctx context.Context, path string, isStatic bool, attributes ...attribute.KeyValue) interface{}

	// FinishRequest completes metrics collection for a request.
	// Takes the request metrics object (interface{}) returned by StartRequest.
	// If metrics is nil or invalid type, this is a no-op.
	FinishRequest(ctx context.Context, metrics interface{}, statusCode int, responseSize int64)

	// IsEnabled returns true if metrics are enabled.
	IsEnabled() bool
}

// WithMetrics creates a router option that enables metrics collection.
// This is the main entry point for integrating metrics with the router.
// Returns a function that can be used with router.New().
// Panics if metrics initialization fails. Use WithMetricsOrError for error handling.
func WithMetrics(opts ...Option) interface{} {
	return func(r interface{}) {
		// Create metrics configuration
		config := MustNew(opts...)

		// Try to set the metrics configuration on the router
		// This uses interface{} to avoid circular dependencies
		if setter, ok := r.(interface{ SetMetricsRecorder(MetricsRecorder) }); ok {
			setter.SetMetricsRecorder(config)
		}
	}
}

// WithMetricsFromConfig creates a router option from an existing metrics config.
func WithMetricsFromConfig(config *Config) interface{} {
	return func(r interface{}) {
		if setter, ok := r.(interface{ SetMetricsRecorder(MetricsRecorder) }); ok {
			setter.SetMetricsRecorder(config)
		}
	}
}

// Middleware creates a middleware function for manual integration.
// This is useful when you want to add metrics to an existing router
// without using the options pattern.
//
// Note: This middleware always marks routes as dynamic (isStatic=false) since it cannot
// distinguish between static and dynamic routes. For accurate static/dynamic route metrics,
// use the router's built-in metrics integration via SetMetricsRecorder.
func Middleware(config *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should be excluded
			if config.excludePaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Build attributes
			attributes := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.route", r.URL.Path),
				attribute.String("http.user_agent", r.UserAgent()),
			}

			// Record request size if available
			if r.ContentLength > 0 {
				attributes = append(attributes, attribute.Int64("http.request.size", r.ContentLength))
			}

			// Record specific headers if configured
			for _, header := range config.recordHeaders {
				if value := r.Header.Get(header); value != "" {
					attributes = append(attributes, attribute.String(
						fmt.Sprintf("http.request.header.%s", strings.ToLower(header)),
						value,
					))
				}
			}

			// Get context from request
			ctx := r.Context()

			// Start request metrics (mark as dynamic since we can't determine actual route type)
			requestMetrics := config.StartRequest(ctx, r.URL.Path, false, attributes...)

			// Wrap response writer to capture status code and size
			rw := &responseWriter{ResponseWriter: w}

			// Execute the next handler
			next.ServeHTTP(rw, r)

			// Finish metrics collection
			config.FinishRequest(ctx, requestMetrics, rw.StatusCode(), int64(rw.Size()))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	written    bool
}

// WriteHeader captures the status code and prevents duplicate calls.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size and marks as written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// StatusCode returns the HTTP status code.
func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

// Size returns the response size in bytes.
func (rw *responseWriter) Size() int {
	return rw.size
}

// Context integration helpers for router context
type ContextMetrics struct {
	config *Config
}

// NewContextMetrics creates a new context metrics helper.
func NewContextMetrics(config *Config) *ContextMetrics {
	return &ContextMetrics{config: config}
}

// RecordMetric records a custom histogram metric with the given name and value.
func (cm *ContextMetrics) RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	cm.config.RecordMetric(ctx, name, value, attributes...)
}

// IncrementCounter increments a custom counter metric with the given name.
func (cm *ContextMetrics) IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	cm.config.IncrementCounter(ctx, name, attributes...)
}

// SetGauge sets a custom gauge metric with the given name and value.
func (cm *ContextMetrics) SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	cm.config.SetGauge(ctx, name, value, attributes...)
}

// GetConfig returns the underlying metrics configuration.
func (cm *ContextMetrics) GetConfig() *Config {
	return cm.config
}

// NewStandalone creates a metrics configuration for standalone usage
// (not integrated with router). Panics if initialization fails.
// For error handling, use New() directly.
func NewStandalone(opts ...Option) *Config {
	return MustNew(opts...)
}
