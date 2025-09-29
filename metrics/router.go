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

package metrics

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
)

// NOTE: Router integration is now handled via app.ObservabilityRecorder.
// The old Recorder interface and WithMetrics functions have been removed.
//
// For standalone metrics (outside of the app/router framework), use:
//   - metrics.MustNew() to create a Config
//   - Config.StartRequest() and Config.FinishRequest() to track HTTP requests
//   - Config.RecordMetric(), Config.IncrementCounter(), etc. for custom metrics
//   - Middleware() for manual HTTP middleware integration
//
// For app-integrated metrics, use:
//   - app.WithMetrics() to enable metrics in your app
//   - The app package handles observability wiring automatically

// Middleware creates a middleware function for manual integration.
// This is useful when you want to add metrics to an existing router
// without using the options pattern.
//
// Note: This middleware always marks routes as dynamic (isStatic=false) since it cannot
// distinguish between static and dynamic routes. For accurate static/dynamic route metrics,
// use the router's built-in metrics integration via SetMetricsRecorder.
//
// Example:
//
//	config := metrics.MustNew(metrics.WithServiceName("my-service"))
//	mux := http.NewServeMux()
//	mux.Handle("/", metrics.Middleware(config)(myHandler))
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

			// Pre-allocate attributes slice with estimated capacity
			estimatedCap := 6 // base attributes
			if r.ContentLength > 0 {
				estimatedCap++
			}
			estimatedCap += len(config.recordHeaders)

			// Build base attributes
			attributes := make([]attribute.KeyValue, 6, estimatedCap)
			attributes[0] = attribute.String("http.method", r.Method)
			attributes[1] = attribute.String("http.url", r.URL.String())
			attributes[2] = attribute.String("http.scheme", r.URL.Scheme)
			attributes[3] = attribute.String("http.host", r.Host)
			attributes[4] = attribute.String("http.route", r.URL.Path)
			attributes[5] = attribute.String("http.user_agent", r.UserAgent())

			// Record request size if available
			if r.ContentLength > 0 {
				attributes = append(attributes, attribute.Int64("http.request.size", r.ContentLength))
			}

			// Record specific headers if configured
			for i, header := range config.recordHeaders {
				if value := r.Header.Get(header); value != "" {
					// Use pre-computed lowercase header name
					attrKey := "http.request.header." + config.recordHeadersLower[i]
					attributes = append(attributes, attribute.String(attrKey, value))
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
			// Use raw path as route pattern since middleware cannot determine actual route template
			config.FinishRequest(ctx, requestMetrics, rw.StatusCode(), int64(rw.Size()), r.URL.Path)
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

// ContextMetrics provides context integration helpers for router context.
type ContextMetrics struct {
	config *Config
}

// NewContextMetrics creates a new context metrics helper.
//
// Example:
//
//	config := metrics.MustNew()
//	cm := metrics.NewContextMetrics(config)
//	cm.IncrementCounter(ctx, "custom_events_total")
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
//
// Example:
//
//	config := metrics.NewStandalone(
//	    metrics.WithServiceName("standalone-service"),
//	    metrics.WithProvider(metrics.PrometheusProvider),
//	)
//	defer config.Shutdown(context.Background())
func NewStandalone(opts ...Option) *Config {
	return MustNew(opts...)
}
