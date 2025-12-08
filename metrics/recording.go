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
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// metricNameRegex validates metric names according to OpenTelemetry conventions.
// Metric names must start with a letter and contain only alphanumeric characters, underscores, dots, and hyphens.
// Compiled in init() to catch any regex errors at package initialization time.
var metricNameRegex *regexp.Regexp

func init() {
	// Compile metric name validation regex
	// Using init() ensures we catch regex errors early and only compile once
	metricNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
}

const (
	// maxMetricNameLength is the maximum allowed length for metric names.
	maxMetricNameLength = 255
)

// Reserved metric name prefixes that should not be used for custom metrics.
// These prefixes are reserved by Prometheus, OpenTelemetry, or the metrics package itself.
var reservedPrefixes = []string{
	"__",      // Reserved by Prometheus for internal use
	"http_",   // Reserved by this package for HTTP metrics
	"router_", // Reserved by this package for router-specific metrics
}

// limitError is returned when the custom metrics limit is reached.
type limitError struct {
	metricName string
	limit      int
	current    int
}

func (e *limitError) Error() string {
	return fmt.Sprintf("metrics limit reached: cannot create '%s' (current: %d, limit: %d)",
		e.metricName, e.current, e.limit)
}

// validateMetricName validates that a metric name conforms to OpenTelemetry conventions.
// Returns an error if the name is invalid.
func validateMetricName(name string) error {
	if name == "" {
		return fmt.Errorf("metric name cannot be empty")
	}
	if len(name) > maxMetricNameLength {
		return fmt.Errorf("metric name too long: %d characters (max %d)", len(name), maxMetricNameLength)
	}
	if !metricNameRegex.MatchString(name) {
		return fmt.Errorf("invalid metric name '%s': must start with letter and contain only alphanumeric, underscore, dot, or hyphen", name)
	}

	// Check for reserved prefixes
	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return fmt.Errorf("metric name '%s' uses reserved prefix '%s': reserved prefixes are %v",
				name, prefix, reservedPrefixes)
		}
	}

	return nil
}

// RequestMetrics holds metrics data for a single request.
// This is an exported type for use by integrators (like app package).
type RequestMetrics struct {
	StartTime  time.Time
	Attributes []attribute.KeyValue
}

// Start initializes metrics collection for a request.
// This is the minimal API for app integration; it starts timing.
// Returns nil if metrics are disabled.
// Call [Recorder.Finish] when the request completes to record the metrics.
func (r *Recorder) Start(ctx context.Context) *RequestMetrics {
	if !r.enabled {
		return nil
	}

	m := &RequestMetrics{
		StartTime: time.Now(),
	}

	// Pre-allocate with service attributes
	m.Attributes = make([]attribute.KeyValue, 2, 8)
	m.Attributes[0] = r.serviceNameAttr
	m.Attributes[1] = r.serviceVersionAttr

	// Increment active requests
	r.activeRequests.Add(ctx, 1, metric.WithAttributes(m.Attributes...))

	return m
}

// Finish completes metrics collection for a request.
// This is the minimal API for app integration.
//
// Parameters:
//   - m: [RequestMetrics] returned from [Recorder.Start] (can be nil)
//   - statusCode: HTTP status code
//   - responseSize: Response body size in bytes
//   - route: Route pattern for cardinality control (e.g., "/users/{id}")
func (r *Recorder) Finish(ctx context.Context, m *RequestMetrics, statusCode int, responseSize int64, route string) {
	if m == nil {
		return
	}

	// Calculate duration
	duration := time.Since(m.StartTime).Seconds()

	// Add status code and route pattern to attributes
	finalAttributes := append(m.Attributes,
		attribute.Int("http.status_code", statusCode),
		attribute.String("http.status_class", statusClass(statusCode)),
		attribute.String("http.route", route),
	)

	// Record duration
	r.requestDuration.Record(ctx, duration, metric.WithAttributes(finalAttributes...))

	// Increment request count
	r.requestCount.Add(ctx, 1, metric.WithAttributes(finalAttributes...))

	// Decrement active requests
	r.activeRequests.Add(ctx, -1, metric.WithAttributes(finalAttributes...))

	// Record error if status indicates error
	if statusCode >= 400 {
		r.errorCount.Add(ctx, 1, metric.WithAttributes(finalAttributes...))
	}

	// Record response size if available
	if responseSize > 0 {
		r.responseSize.Record(ctx, responseSize, metric.WithAttributes(finalAttributes...))
	}
}

// RecordRequestSize records the request body size.
// Call this after [Recorder.Start] if you have the request size available.
func (r *Recorder) RecordRequestSize(ctx context.Context, m *RequestMetrics, size int64) {
	if m == nil || size <= 0 {
		return
	}
	r.requestSize.Record(ctx, size, metric.WithAttributes(m.Attributes...))
}

// AddAttributes adds attributes to the request metrics.
// This should be called before [Recorder.Finish] to add custom attributes.
func (m *RequestMetrics) AddAttributes(attrs ...attribute.KeyValue) {
	if m == nil {
		return
	}
	m.Attributes = append(m.Attributes, attrs...)
}

// statusClass returns the HTTP status class (2xx, 3xx, 4xx, 5xx).
func statusClass(statusCode int) string {
	switch statusCode / 100 {
	case 2:
		return "2xx"
	case 3:
		return "3xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	default:
		return "unknown"
	}
}

// RecordHistogram records a custom histogram metric with the given name and value.
// Returns an error if the metric name is invalid or creation fails.
//
// Example:
//
//	err := recorder.RecordHistogram(ctx, "processing_duration", 1.5,
//	    attribute.String("operation", "create_user"))
//	if err != nil {
//	    // Handle error or ignore with: _ = recorder.RecordHistogram(...)
//	}
func (r *Recorder) RecordHistogram(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) error {
	if !r.enabled {
		return nil
	}

	histogram, err := r.getOrCreateHistogram(name)
	if err != nil {
		atomic.AddInt64(&r.atomicCustomMetricFailures, 1)
		r.customMetricFailures.Add(ctx, 1)

		return fmt.Errorf("record histogram %q: %w", name, err)
	}

	histogram.Record(ctx, value, metric.WithAttributes(attributes...))

	return nil
}

// IncrementCounter increments a custom counter metric by 1.
// Returns an error if the metric name is invalid or creation fails.
//
// Example:
//
//	err := recorder.IncrementCounter(ctx, "requests_total",
//	    attribute.String("status", "success"))
//	if err != nil {
//	    // Handle error or ignore with: _ = recorder.IncrementCounter(...)
//	}
func (r *Recorder) IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue) error {
	return r.AddCounter(ctx, name, 1, attributes...)
}

// AddCounter adds a value to a custom counter metric.
// Returns an error if the metric name is invalid or creation fails.
//
// Example:
//
//	err := recorder.AddCounter(ctx, "bytes_processed", 1024,
//	    attribute.String("type", "upload"))
func (r *Recorder) AddCounter(ctx context.Context, name string, value int64, attributes ...attribute.KeyValue) error {
	if !r.enabled {
		return nil
	}

	counter, err := r.getOrCreateCounter(name)
	if err != nil {
		atomic.AddInt64(&r.atomicCustomMetricFailures, 1)
		r.customMetricFailures.Add(ctx, 1)

		return fmt.Errorf("add counter %q: %w", name, err)
	}

	counter.Add(ctx, value, metric.WithAttributes(attributes...))

	return nil
}

// SetGauge sets a custom gauge metric with the given name and value.
// Returns an error if the metric name is invalid or creation fails.
//
// Example:
//
//	err := recorder.SetGauge(ctx, "active_connections", 42,
//	    attribute.String("server", "api-1"))
func (r *Recorder) SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) error {
	if !r.enabled {
		return nil
	}

	gauge, err := r.getOrCreateGauge(name)
	if err != nil {
		atomic.AddInt64(&r.atomicCustomMetricFailures, 1)
		r.customMetricFailures.Add(ctx, 1)

		return fmt.Errorf("set gauge %q: %w", name, err)
	}

	gauge.Record(ctx, value, metric.WithAttributes(attributes...))

	return nil
}

// initializeMetrics creates all the metric instruments.
func (r *Recorder) initializeMetrics() error {
	var err error

	// Request duration histogram with configurable buckets
	r.requestDuration, err = r.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(r.durationBuckets...),
	)
	if err != nil {
		return fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	// Request count counter
	r.requestCount, err = r.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request count counter: %w", err)
	}

	// Active requests gauge
	r.activeRequests, err = r.meter.Int64UpDownCounter(
		"http_requests_active",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create active requests gauge: %w", err)
	}

	// Request size histogram with configurable buckets
	r.requestSize, err = r.meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(r.sizeBuckets...),
	)
	if err != nil {
		return fmt.Errorf("failed to create request size histogram: %w", err)
	}

	// Response size histogram with configurable buckets
	r.responseSize, err = r.meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("Size of HTTP response bodies in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(r.sizeBuckets...),
	)
	if err != nil {
		return fmt.Errorf("failed to create response size histogram: %w", err)
	}

	// Error count counter
	r.errorCount, err = r.meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP errors"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error count counter: %w", err)
	}

	// Custom metric failures counter
	r.customMetricFailures, err = r.meter.Int64Counter(
		"custom_metric_failures_total",
		metric.WithDescription("Total number of custom metric creation failures"),
	)
	if err != nil {
		return fmt.Errorf("failed to create custom metric failures counter: %w", err)
	}

	return nil
}

// getOrCreateCounter gets or creates a custom counter metric.
// This method is safe for concurrent use.
func (r *Recorder) getOrCreateCounter(name string) (metric.Int64Counter, error) {
	// Fast path: read lock
	r.customMu.RLock()
	if counter, exists := r.customCounters[name]; exists {
		r.customMu.RUnlock()
		return counter, nil
	}
	r.customMu.RUnlock()

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Slow path: write lock
	r.customMu.Lock()
	defer r.customMu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := r.customCounters[name]; exists {
		return counter, nil
	}

	// Check limit
	if r.customMetricCount >= r.maxCustomMetrics {
		return nil, &limitError{
			metricName: name,
			limit:      r.maxCustomMetrics,
			current:    r.customMetricCount,
		}
	}

	// Create the metric
	counter, err := r.meter.Int64Counter(
		name,
		metric.WithDescription("Custom counter metric"),
	)
	if err != nil {
		return nil, err
	}

	r.customCounters[name] = counter
	r.customMetricCount++

	return counter, nil
}

// getOrCreateHistogram gets or creates a custom histogram metric.
// This method is safe for concurrent use.
func (r *Recorder) getOrCreateHistogram(name string) (metric.Float64Histogram, error) {
	// Fast path: read lock
	r.customMu.RLock()
	if histogram, exists := r.customHistograms[name]; exists {
		r.customMu.RUnlock()
		return histogram, nil
	}
	r.customMu.RUnlock()

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Slow path: write lock
	r.customMu.Lock()
	defer r.customMu.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists := r.customHistograms[name]; exists {
		return histogram, nil
	}

	// Check limit
	if r.customMetricCount >= r.maxCustomMetrics {
		return nil, &limitError{
			metricName: name,
			limit:      r.maxCustomMetrics,
			current:    r.customMetricCount,
		}
	}

	// Create the metric
	histogram, err := r.meter.Float64Histogram(
		name,
		metric.WithDescription("Custom histogram metric"),
	)
	if err != nil {
		return nil, err
	}

	r.customHistograms[name] = histogram
	r.customMetricCount++

	return histogram, nil
}

// getOrCreateGauge gets or creates a custom gauge metric.
// This method is safe for concurrent use.
func (r *Recorder) getOrCreateGauge(name string) (metric.Float64Gauge, error) {
	// Fast path: read lock
	r.customMu.RLock()
	if gauge, exists := r.customGauges[name]; exists {
		r.customMu.RUnlock()
		return gauge, nil
	}
	r.customMu.RUnlock()

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Slow path: write lock
	r.customMu.Lock()
	defer r.customMu.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists := r.customGauges[name]; exists {
		return gauge, nil
	}

	// Check limit
	if r.customMetricCount >= r.maxCustomMetrics {
		return nil, &limitError{
			metricName: name,
			limit:      r.maxCustomMetrics,
			current:    r.customMetricCount,
		}
	}

	// Create the metric
	gauge, err := r.meter.Float64Gauge(
		name,
		metric.WithDescription("Custom gauge metric"),
	)
	if err != nil {
		return nil, err
	}

	r.customGauges[name] = gauge
	r.customMetricCount++

	return gauge, nil
}

// getAtomicCustomMetricFailures returns the atomic custom metric failures counter (for testing).
func (r *Recorder) getAtomicCustomMetricFailures() int64 {
	return atomic.LoadInt64(&r.atomicCustomMetricFailures)
}

// CustomMetricCount returns the number of custom metrics created (for testing/monitoring).
func (r *Recorder) CustomMetricCount() int {
	r.customMu.RLock()
	defer r.customMu.RUnlock()

	return r.customMetricCount
}
