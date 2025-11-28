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
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

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

// LimitError is returned when the custom metrics limit is reached.
type LimitError struct {
	MetricName string
	Limit      int
	Current    int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("metrics limit reached: cannot create '%s' (current: %d, limit: %d)",
		e.MetricName, e.Current, e.Limit)
}

// Unwrap returns nil as LimitError is a leaf error type.
// This allows errors.Is() and errors.As() to work correctly.
func (e *LimitError) Unwrap() error {
	return nil
}

// UpdateError is returned when atomic map update fails after max retries.
type UpdateError struct {
	Operation string
	Retries   int
}

func (e *UpdateError) Error() string {
	return fmt.Sprintf("failed to update metrics map after %d retries: %s", e.Retries, e.Operation)
}

// Unwrap returns nil as UpdateError is a leaf error type.
// This allows errors.Is() and errors.As() to work correctly.
func (e *UpdateError) Unwrap() error {
	return nil
}

const (
	// maxCASRetries is the maximum number of Compare-And-Swap retries before falling back to logging.
	maxCASRetries = 100
)

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

// requestMetrics holds metrics data for a single request.
type requestMetrics struct {
	startTime   time.Time
	requestSize int64
	attributes  []attribute.KeyValue
}

// StartRequest initializes metrics collection for a request.
// It creates a requestMetrics struct to track request timing and attributes.
// The router typically provides http.request.size as the first attribute.
func (c *Config) StartRequest(ctx context.Context, path string, isStatic bool, attributes ...attribute.KeyValue) interface{} {
	if !c.enabled {
		return nil
	}

	// Check if path should be excluded
	if c.ShouldExcludePath(path) {
		return nil
	}

	metrics := &requestMetrics{
		startTime: time.Now(),
	}

	// Build attributes with pre-computed service name/version and static route flag
	totalCap := 3 + len(attributes) // 3 base attrs + provided attrs
	metrics.attributes = make([]attribute.KeyValue, 3, totalCap)
	metrics.attributes[0] = c.serviceNameAttr
	metrics.attributes[1] = c.serviceVersionAttr
	if isStatic {
		metrics.attributes[2] = c.staticRouteAttr
	} else {
		metrics.attributes[2] = c.dynamicRouteAttr
	}
	metrics.attributes = append(metrics.attributes, attributes...)

	// Increment active requests atomically
	c.recordActiveRequestAtomically()
	c.activeRequests.Add(ctx, 1, metric.WithAttributes(metrics.attributes...))

	// Extract request size from first attribute if present.
	// The router typically provides http.request.size as the first attribute.
	if len(attributes) > 0 {
		if attributes[0].Key == "http.request.size" && attributes[0].Value.Type() == attribute.INT64 {
			size := attributes[0].Value.AsInt64()
			metrics.requestSize = size
			c.requestSize.Record(ctx, size, metric.WithAttributes(metrics.attributes...))
		}
	}

	return metrics
}

// FinishRequest completes metrics collection for a request.
func (c *Config) FinishRequest(ctx context.Context, metrics interface{}, statusCode int, responseSize int64, routePattern string) {
	requestMetrics, ok := metrics.(*requestMetrics)
	if !ok || requestMetrics == nil {
		return
	}

	// Calculate duration
	duration := time.Since(requestMetrics.startTime).Seconds()

	// Add status code and route pattern to attributes
	// Use routePattern (template) instead of raw path to prevent cardinality explosion
	finalAttributes := append(requestMetrics.attributes,
		attribute.Int("http.status_code", statusCode),
		attribute.String("http.status_class", getStatusClass(statusCode)),
		attribute.String("http.route", routePattern), // Add route template for cardinality control
	)

	// Record duration
	c.requestDuration.Record(ctx, duration, metric.WithAttributes(finalAttributes...))

	// Increment request count atomically
	c.recordRequestCountAtomically()
	c.requestCount.Add(ctx, 1, metric.WithAttributes(finalAttributes...))

	// Decrement active requests atomically
	c.recordActiveRequestCompleteAtomically()
	c.activeRequests.Add(ctx, -1, metric.WithAttributes(finalAttributes...))

	// Record error if status indicates error
	if statusCode >= 400 {
		c.recordErrorCountAtomically()
		c.errorCount.Add(ctx, 1, metric.WithAttributes(finalAttributes...))
	}

	// Record response size if available
	if responseSize > 0 {
		c.responseSize.Record(ctx, responseSize, metric.WithAttributes(finalAttributes...))
	}
}

// getStatusClass returns the HTTP status class (2xx, 3xx, 4xx, 5xx).
func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

// RecordMetric records a custom histogram metric with the given name and value.
// Returns early if the metric name is invalid or creation fails.
// Context cancellation is handled by the OpenTelemetry SDK internally.
//
// Example:
//
//	config.RecordMetric(ctx, "processing_duration", 1.5,
//	    attribute.String("operation", "create_user"))
func (c *Config) RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Get or create histogram (validation happens inside for new metrics only)
	histogram, err := c.getOrCreateHistogram(ctx, name)
	if err != nil {
		c.recordCustomMetricFailureAtomically()
		c.emitError("Failed to get or create histogram metric", "name", name, "error", err)
		return
	}

	// Record the metric (OTel SDK handles ctx.Done internally)
	histogram.Record(ctx, value, metric.WithAttributes(attributes...))
}

// IncrementCounter increments a custom counter metric with the given name.
// Returns early if the metric name is invalid or creation fails.
// Context cancellation is handled by the OpenTelemetry SDK internally.
//
// Example:
//
//	config.IncrementCounter(ctx, "requests_total",
//	    attribute.String("status", "success"))
func (c *Config) IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Get or create counter (validation happens inside for new metrics only)
	counter, err := c.getOrCreateCounter(ctx, name)
	if err != nil {
		c.recordCustomMetricFailureAtomically()
		c.emitError("Failed to get or create counter metric", "name", name, "error", err)
		return
	}

	// Increment the counter (OTel SDK handles ctx.Done internally)
	counter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// SetGauge sets a custom gauge metric with the given name and value.
// Thread-safe through atomic operations in metric creation and caching.
// Returns early if the metric name is invalid or creation fails.
// Context cancellation is handled by the OpenTelemetry SDK internally.
func (c *Config) SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Get or create gauge (validation happens inside for new metrics only)
	gauge, err := c.getOrCreateGauge(ctx, name)
	if err != nil {
		c.recordCustomMetricFailureAtomically()
		c.emitError("Failed to get or create gauge metric", "name", name, "error", err)
		return
	}

	// Set the gauge value (OTel SDK handles ctx.Done internally)
	gauge.Record(ctx, value, metric.WithAttributes(attributes...))
}

// RecordRouteRegistration records route registration metrics.
func (c *Config) RecordRouteRegistration(ctx context.Context, method, path string) {
	if !c.enabled {
		return
	}

	// Use pre-computed service attributes
	attributes := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		c.serviceNameAttr,
		c.serviceVersionAttr,
	}

	c.routeCount.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordContextPoolHit records a context pool hit (reused context).
func (c *Config) RecordContextPoolHit(ctx context.Context) {
	if !c.enabled {
		return
	}

	atomic.AddInt64(&c.atomicContextPoolHits, 1)
	c.contextPoolHits.Add(ctx, 1)
}

// RecordContextPoolMiss records a context pool miss.
func (c *Config) RecordContextPoolMiss(ctx context.Context) {
	if !c.enabled {
		return
	}

	atomic.AddInt64(&c.atomicContextPoolMisses, 1)
	c.contextPoolMisses.Add(ctx, 1)
}

// RecordConstraintFailure records a route constraint validation failure.
func (c *Config) RecordConstraintFailure(ctx context.Context, constraint string, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Use pre-computed service attributes
	attrs := make([]attribute.KeyValue, 3, 3+len(attributes))
	attrs[0] = attribute.String("constraint.type", constraint)
	attrs[1] = c.serviceNameAttr
	attrs[2] = c.serviceVersionAttr
	attrs = append(attrs, attributes...)

	c.constraintFailures.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// initializeMetrics creates all the metric instruments.
func (c *Config) initializeMetrics() error {
	var err error

	// Request duration histogram
	c.requestDuration, err = c.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	// Request count counter
	c.requestCount, err = c.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request count counter: %w", err)
	}

	// Active requests gauge
	c.activeRequests, err = c.meter.Int64UpDownCounter(
		"http_requests_active",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create active requests gauge: %w", err)
	}

	// Request size histogram
	c.requestSize, err = c.meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request size histogram: %w", err)
	}

	// Response size histogram
	c.responseSize, err = c.meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("Size of HTTP response bodies in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create response size histogram: %w", err)
	}

	// Route count counter
	c.routeCount, err = c.meter.Int64Counter(
		"http_routes_total",
		metric.WithDescription("Total number of registered routes"),
	)
	if err != nil {
		return fmt.Errorf("failed to create route count counter: %w", err)
	}

	// Error count counter
	c.errorCount, err = c.meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP errors"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error count counter: %w", err)
	}

	// Constraint failures counter
	c.constraintFailures, err = c.meter.Int64Counter(
		"http_constraint_failures_total",
		metric.WithDescription("Total number of route constraint validation failures"),
	)
	if err != nil {
		return fmt.Errorf("failed to create constraint failures counter: %w", err)
	}

	// Context pool hits counter
	c.contextPoolHits, err = c.meter.Int64Counter(
		"router_context_pool_hits_total",
		metric.WithDescription("Total number of context pool hits"),
	)
	if err != nil {
		return fmt.Errorf("failed to create context pool hits counter: %w", err)
	}

	// Context pool misses counter
	c.contextPoolMisses, err = c.meter.Int64Counter(
		"router_context_pool_misses_total",
		metric.WithDescription("Total number of context pool misses"),
	)
	if err != nil {
		return fmt.Errorf("failed to create context pool misses counter: %w", err)
	}

	// Custom metric failures counter
	c.customMetricFailures, err = c.meter.Int64Counter(
		"router_custom_metric_failures_total",
		metric.WithDescription("Total number of custom metric creation failures"),
	)
	if err != nil {
		return fmt.Errorf("failed to create custom metric failures counter: %w", err)
	}

	// CAS retries counter for contention monitoring
	c.casRetriesCounter, err = c.meter.Int64Counter(
		"router_metrics_cas_retries_total",
		metric.WithDescription("Total number of Compare-And-Swap retry attempts when updating custom metrics maps"),
	)
	if err != nil {
		return fmt.Errorf("failed to create CAS retries counter: %w", err)
	}

	return nil
}

// Atomic operations for built-in metrics
func (c *Config) recordRequestCountAtomically() {
	atomic.AddInt64(&c.atomicRequestCount, 1)
}

func (c *Config) recordActiveRequestAtomically() {
	atomic.AddInt64(&c.atomicActiveRequests, 1)
}

func (c *Config) recordActiveRequestCompleteAtomically() {
	atomic.AddInt64(&c.atomicActiveRequests, -1)
}

func (c *Config) recordErrorCountAtomically() {
	atomic.AddInt64(&c.atomicErrorCount, 1)
}

func (c *Config) recordCustomMetricFailureAtomically() {
	atomic.AddInt64(&c.atomicCustomMetricFailures, 1)
}

func (c *Config) recordCASRetryAtomically() {
	atomic.AddInt64(&c.atomicCASRetries, 1)
}

// Getters for atomic counters used in tests
func (c *Config) getAtomicContextPoolHits() int64 {
	return atomic.LoadInt64(&c.atomicContextPoolHits)
}

func (c *Config) getAtomicContextPoolMisses() int64 {
	return atomic.LoadInt64(&c.atomicContextPoolMisses)
}

func (c *Config) getAtomicCustomMetricFailures() int64 {
	return atomic.LoadInt64(&c.atomicCustomMetricFailures)
}

func (c *Config) getAtomicCASRetries() int64 {
	return atomic.LoadInt64(&c.atomicCASRetries)
}

// Atomic custom metrics operations
func (c *Config) getAtomicCustomCounters() map[string]metric.Int64Counter {
	ptr := atomic.LoadPointer(&c.atomicCustomCounters)
	return *(*map[string]metric.Int64Counter)(ptr)
}

func (c *Config) getAtomicCustomHistograms() map[string]metric.Float64Histogram {
	ptr := atomic.LoadPointer(&c.atomicCustomHistograms)
	return *(*map[string]metric.Float64Histogram)(ptr)
}

func (c *Config) getAtomicCustomGauges() map[string]metric.Float64Gauge {
	ptr := atomic.LoadPointer(&c.atomicCustomGauges)
	return *(*map[string]metric.Float64Gauge)(ptr)
}

// updateAtomicCustomCounters updates the custom counters map.
// The updater function receives the current map and returns a new map.
// If another goroutine modifies the map concurrently, the operation retries.
// Retry attempts are tracked via router_metrics_cas_retries_total for observability.
func (c *Config) updateAtomicCustomCounters(updater func(map[string]metric.Int64Counter) map[string]metric.Int64Counter) error {
	for attempt := 0; attempt < maxCASRetries; attempt++ {
		currentPtr := atomic.LoadPointer(&c.atomicCustomCounters)
		current := *(*map[string]metric.Int64Counter)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&c.atomicCustomCounters, currentPtr, unsafe.Pointer(&newMap)) {
			// Record total retries for this operation (observability)
			if attempt > 0 {
				c.recordCASRetryAtomically()
				c.casRetriesCounter.Add(context.Background(), int64(attempt))
			}
			return nil
		}

		// CAS failed, retry with backoff
		switch {
		case attempt < 3:
			runtime.Gosched()
		case attempt < 10:
			time.Sleep(time.Microsecond)
		default:
			backoff := time.Microsecond * time.Duration(1<<uint(attempt-10))
			if backoff > time.Millisecond {
				backoff = time.Millisecond
			}
			time.Sleep(backoff)
		}
	}
	c.emitWarning("Failed to update custom counters after max retries", "maxRetries", maxCASRetries)
	return &UpdateError{Operation: "updateCustomCounters", Retries: maxCASRetries}
}

// updateAtomicCustomHistograms updates the custom histograms map.
// See updateAtomicCustomCounters for implementation details.
func (c *Config) updateAtomicCustomHistograms(updater func(map[string]metric.Float64Histogram) map[string]metric.Float64Histogram) error {
	for attempt := 0; attempt < maxCASRetries; attempt++ {
		currentPtr := atomic.LoadPointer(&c.atomicCustomHistograms)
		current := *(*map[string]metric.Float64Histogram)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&c.atomicCustomHistograms, currentPtr, unsafe.Pointer(&newMap)) {
			// Record total retries for this operation (observability)
			if attempt > 0 {
				c.recordCASRetryAtomically()
				c.casRetriesCounter.Add(context.Background(), int64(attempt))
			}
			return nil
		}

		// CAS failed, retry with backoff
		switch {
		case attempt < 3:
			runtime.Gosched()
		case attempt < 10:
			time.Sleep(time.Microsecond)
		default:
			backoff := time.Microsecond * time.Duration(1<<uint(attempt-10))
			if backoff > time.Millisecond {
				backoff = time.Millisecond
			}
			time.Sleep(backoff)
		}
	}
	c.emitWarning("Failed to update custom histograms after max retries", "maxRetries", maxCASRetries)
	return &UpdateError{Operation: "updateCustomHistograms", Retries: maxCASRetries}
}

// updateAtomicCustomGauges updates the custom gauges map.
// See updateAtomicCustomCounters for implementation details.
func (c *Config) updateAtomicCustomGauges(updater func(map[string]metric.Float64Gauge) map[string]metric.Float64Gauge) error {
	for attempt := 0; attempt < maxCASRetries; attempt++ {
		currentPtr := atomic.LoadPointer(&c.atomicCustomGauges)
		current := *(*map[string]metric.Float64Gauge)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&c.atomicCustomGauges, currentPtr, unsafe.Pointer(&newMap)) {
			// Record total retries for this operation (observability)
			if attempt > 0 {
				c.recordCASRetryAtomically()
				c.casRetriesCounter.Add(context.Background(), int64(attempt))
			}
			return nil
		}

		// CAS failed, retry with backoff
		switch {
		case attempt < 3:
			runtime.Gosched()
		case attempt < 10:
			time.Sleep(time.Microsecond)
		default:
			backoff := time.Microsecond * time.Duration(1<<uint(attempt-10))
			if backoff > time.Millisecond {
				backoff = time.Millisecond
			}
			time.Sleep(backoff)
		}
	}
	c.emitWarning("Failed to update custom gauges after max retries", "maxRetries", maxCASRetries)
	return &UpdateError{Operation: "updateCustomGauges", Retries: maxCASRetries}
}

// getOrCreateCounter gets or creates a custom counter metric.
// Validation only happens for new metrics, not for cached metrics.
func (c *Config) getOrCreateCounter(ctx context.Context, name string) (metric.Int64Counter, error) {
	// First, check if counter already exists
	counters := c.getAtomicCustomCounters()
	if counter, exists := counters[name]; exists {
		return counter, nil
	}

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Check if context is cancelled before expensive operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try to atomically increment the count with CAS
	for {
		currentCount := atomic.LoadInt64(&c.atomicCustomMetricsCount)

		// Check limit
		if currentCount >= int64(c.maxCustomMetrics) {
			return nil, &LimitError{
				MetricName: name,
				Limit:      c.maxCustomMetrics,
				Current:    currentCount,
			}
		}

		// Try to reserve a slot
		if atomic.CompareAndSwapInt64(&c.atomicCustomMetricsCount, currentCount, currentCount+1) {
			// We successfully reserved a slot
			// Double-check if another goroutine just created this metric while we were waiting
			counters := c.getAtomicCustomCounters()
			if counter, exists := counters[name]; exists {
				// Another goroutine created it, release the slot we reserved
				atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
				return counter, nil
			}
			// Metric doesn't exist yet, proceed to create it
			break
		}
		// CAS failed, retry
	}

	// At this point, we have reserved a slot and verified the metric doesn't exist
	// Now create the metric
	counter, err := c.meter.Int64Counter(
		name,
		metric.WithDescription("Custom counter metric"),
	)
	if err != nil {
		// Failed to create metric, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// Atomically update the map
	var finalCounter metric.Int64Counter
	var shouldReleaseSlot bool

	if err := c.updateAtomicCustomCounters(func(current map[string]metric.Int64Counter) map[string]metric.Int64Counter {
		// Check again if another goroutine created it while we were creating ours
		if existingCounter, exists := current[name]; exists {
			finalCounter = existingCounter
			shouldReleaseSlot = true // We didn't use our slot
			return current           // Don't modify map
		}

		// Create new map with our counter
		newMap := make(map[string]metric.Int64Counter, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = counter
		finalCounter = counter
		return newMap
	}); err != nil {
		// Failed to update map after max retries, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// If another goroutine already created this metric, release our reserved slot
	if shouldReleaseSlot {
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
	}

	return finalCounter, nil
}

// getOrCreateHistogram gets or creates a custom histogram metric.
// Validation only happens for new metrics, not for cached metrics.
func (c *Config) getOrCreateHistogram(ctx context.Context, name string) (metric.Float64Histogram, error) {
	// First, check if histogram already exists
	histograms := c.getAtomicCustomHistograms()
	if histogram, exists := histograms[name]; exists {
		return histogram, nil
	}

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Check if context is cancelled before expensive operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try to atomically increment the count with CAS
	for {
		currentCount := atomic.LoadInt64(&c.atomicCustomMetricsCount)

		// Check limit
		if currentCount >= int64(c.maxCustomMetrics) {
			return nil, &LimitError{
				MetricName: name,
				Limit:      c.maxCustomMetrics,
				Current:    currentCount,
			}
		}

		// Try to reserve a slot
		if atomic.CompareAndSwapInt64(&c.atomicCustomMetricsCount, currentCount, currentCount+1) {
			// We successfully reserved a slot
			// Double-check if another goroutine just created this metric while we were waiting
			histograms := c.getAtomicCustomHistograms()
			if histogram, exists := histograms[name]; exists {
				// Another goroutine created it, release the slot we reserved
				atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
				return histogram, nil
			}
			// Metric doesn't exist yet, proceed to create it
			break
		}
		// CAS failed, retry
	}

	// At this point, we have reserved a slot and verified the metric doesn't exist
	// Now create the metric
	histogram, err := c.meter.Float64Histogram(
		name,
		metric.WithDescription("Custom histogram metric"),
	)
	if err != nil {
		// Failed to create metric, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// Atomically update the map
	var finalHistogram metric.Float64Histogram
	var shouldReleaseSlot bool

	if err := c.updateAtomicCustomHistograms(func(current map[string]metric.Float64Histogram) map[string]metric.Float64Histogram {
		// Check again if another goroutine created it while we were creating ours
		if existingHistogram, exists := current[name]; exists {
			finalHistogram = existingHistogram
			shouldReleaseSlot = true // We didn't use our slot
			return current           // Don't modify map
		}

		// Create new map with our histogram
		newMap := make(map[string]metric.Float64Histogram, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = histogram
		finalHistogram = histogram
		return newMap
	}); err != nil {
		// Failed to update map after max retries, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// If another goroutine already created this metric, release our reserved slot
	if shouldReleaseSlot {
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
	}

	return finalHistogram, nil
}

// getOrCreateGauge gets or creates a custom gauge metric.
// Validation only happens for new metrics, not for cached metrics.
func (c *Config) getOrCreateGauge(ctx context.Context, name string) (metric.Float64Gauge, error) {
	// First, check if gauge already exists
	gauges := c.getAtomicCustomGauges()
	if gauge, exists := gauges[name]; exists {
		return gauge, nil
	}

	// Validate metric name only when creating new metric
	if err := validateMetricName(name); err != nil {
		return nil, err
	}

	// Check if context is cancelled before expensive operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try to atomically increment the count with CAS
	for {
		currentCount := atomic.LoadInt64(&c.atomicCustomMetricsCount)

		// Check limit
		if currentCount >= int64(c.maxCustomMetrics) {
			return nil, &LimitError{
				MetricName: name,
				Limit:      c.maxCustomMetrics,
				Current:    currentCount,
			}
		}

		// Try to reserve a slot
		if atomic.CompareAndSwapInt64(&c.atomicCustomMetricsCount, currentCount, currentCount+1) {
			// We successfully reserved a slot
			// Double-check if another goroutine just created this metric while we were waiting
			gauges := c.getAtomicCustomGauges()
			if gauge, exists := gauges[name]; exists {
				// Another goroutine created it, release the slot we reserved
				atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
				return gauge, nil
			}
			// Metric doesn't exist yet, proceed to create it
			break
		}
		// CAS failed, retry
	}

	// At this point, we have reserved a slot and verified the metric doesn't exist
	// Now create the metric
	gauge, err := c.meter.Float64Gauge(
		name,
		metric.WithDescription("Custom gauge metric"),
	)
	if err != nil {
		// Failed to create metric, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// Atomically update the map
	var finalGauge metric.Float64Gauge
	var shouldReleaseSlot bool

	if err := c.updateAtomicCustomGauges(func(current map[string]metric.Float64Gauge) map[string]metric.Float64Gauge {
		// Check again if another goroutine created it while we were creating ours
		if existingGauge, exists := current[name]; exists {
			finalGauge = existingGauge
			shouldReleaseSlot = true // We didn't use our slot
			return current           // Don't modify map
		}

		// Create new map with our gauge
		newMap := make(map[string]metric.Float64Gauge, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = gauge
		finalGauge = gauge
		return newMap
	}); err != nil {
		// Failed to update map after max retries, release the slot
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
		return nil, err
	}

	// If another goroutine already created this metric, release our reserved slot
	if shouldReleaseSlot {
		atomic.AddInt64(&c.atomicCustomMetricsCount, -1)
	}

	return finalGauge, nil
}
