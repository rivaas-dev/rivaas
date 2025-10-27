package metrics

import (
	"context"
	"fmt"
	"regexp"
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

// MetricsLimitError is returned when the custom metrics limit is reached.
type MetricsLimitError struct {
	MetricName string
	Limit      int
	Current    int64
}

func (e *MetricsLimitError) Error() string {
	return fmt.Sprintf("metrics limit reached: cannot create '%s' (current: %d, limit: %d)",
		e.MetricName, e.Current, e.Limit)
}

// MetricsUpdateError is returned when atomic map update fails after max retries.
type MetricsUpdateError struct {
	Operation string
	Retries   int
}

func (e *MetricsUpdateError) Error() string {
	return fmt.Sprintf("failed to update metrics map after %d retries: %s", e.Retries, e.Operation)
}

const (
	// maxCASRetries is the maximum number of Compare-And-Swap retries before falling back to logging.
	maxCASRetries = 100
	// casBackoffThreshold is the retry count after which we start backing off.
	casBackoffThreshold = 10
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
func (c *Config) StartRequest(ctx context.Context, path string, isStatic bool, attributes ...attribute.KeyValue) interface{} {
	if !c.enabled {
		return nil
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// Check if path should be excluded
	if c.excludePaths[path] {
		return nil
	}

	metrics := &requestMetrics{
		startTime: time.Now(),
	}

	// Build base attributes with pre-allocated capacity
	baseAttrs := []attribute.KeyValue{
		attribute.String("service.name", c.serviceName),
		attribute.String("service.version", c.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	}

	// Pre-allocate slice with exact capacity needed
	metrics.attributes = make([]attribute.KeyValue, len(baseAttrs), len(baseAttrs)+len(attributes))
	copy(metrics.attributes, baseAttrs)
	metrics.attributes = append(metrics.attributes, attributes...)

	// Increment active requests atomically
	c.recordActiveRequestAtomically()
	c.activeRequests.Add(ctx, 1, metric.WithAttributes(metrics.attributes...))

	// Record request size if available
	if len(attributes) > 0 {
		for _, attr := range attributes {
			if attr.Key == "http.request.size" {
				if attr.Value.Type() == attribute.INT64 {
					size := attr.Value.AsInt64()
					metrics.requestSize = size
					c.requestSize.Record(ctx, size, metric.WithAttributes(metrics.attributes...))
				}
				break
			}
		}
	}

	return metrics
}

// FinishRequest completes metrics collection for a request.
func (c *Config) FinishRequest(ctx context.Context, metrics interface{}, statusCode int, responseSize int64) {
	requestMetrics, ok := metrics.(*requestMetrics)
	if !ok || requestMetrics == nil {
		return
	}

	// Calculate duration
	duration := time.Since(requestMetrics.startTime).Seconds()

	// Add status code to attributes
	finalAttributes := append(requestMetrics.attributes,
		attribute.Int("http.status_code", statusCode),
		attribute.String("http.status_class", getStatusClass(statusCode)),
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
// This method is thread-safe and uses atomic operations for optimal performance.
// Returns early if the context is cancelled or if the metric name is invalid.
func (c *Config) RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Validate metric name
	if err := validateMetricName(name); err != nil {
		c.recordCustomMetricFailureAtomically()
		c.logError("Invalid metric name", "name", name, "error", err)
		return
	}

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Get or create the histogram
	histogram, err := c.getOrCreateHistogram(ctx, name)
	if err != nil {
		// Record failure and log error
		c.recordCustomMetricFailureAtomically()
		c.logError("Failed to create histogram metric", "name", name, "error", err)
		return
	}

	// Record the metric
	histogram.Record(ctx, value, metric.WithAttributes(attributes...))
}

// IncrementCounter increments a custom counter metric with the given name.
// This method is thread-safe and uses atomic operations for optimal performance.
// Returns early if the context is cancelled or if the metric name is invalid.
func (c *Config) IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Validate metric name
	if err := validateMetricName(name); err != nil {
		c.recordCustomMetricFailureAtomically()
		c.logError("Invalid metric name", "name", name, "error", err)
		return
	}

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Get or create the counter
	counter, err := c.getOrCreateCounter(ctx, name)
	if err != nil {
		// Record failure and log error
		c.recordCustomMetricFailureAtomically()
		c.logError("Failed to create counter metric", "name", name, "error", err)
		return
	}

	// Increment the counter
	counter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// SetGauge sets a custom gauge metric with the given name and value.
// This method is thread-safe and uses atomic operations for optimal performance.
// Returns early if the context is cancelled or if the metric name is invalid.
func (c *Config) SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue) {
	if !c.enabled {
		return
	}

	// Validate metric name
	if err := validateMetricName(name); err != nil {
		c.recordCustomMetricFailureAtomically()
		c.logError("Invalid metric name", "name", name, "error", err)
		return
	}

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Get or create the gauge
	gauge, err := c.getOrCreateGauge(ctx, name)
	if err != nil {
		// Record failure and log error
		c.recordCustomMetricFailureAtomically()
		c.logError("Failed to create gauge metric", "name", name, "error", err)
		return
	}

	// Set the gauge value
	gauge.Record(ctx, value, metric.WithAttributes(attributes...))
}

// RecordRouteRegistration records route registration metrics.
func (c *Config) RecordRouteRegistration(ctx context.Context, method, path string) {
	if !c.enabled {
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.String("service.name", c.serviceName),
		attribute.String("service.version", c.serviceVersion),
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

// RecordContextPoolMiss records a context pool miss (new allocation).
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

	attrs := []attribute.KeyValue{
		attribute.String("constraint.type", constraint),
		attribute.String("service.name", c.serviceName),
		attribute.String("service.version", c.serviceVersion),
	}
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
		metric.WithDescription("Total number of context pool misses (new allocations)"),
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

// updateAtomicCustomCounters atomically updates the custom counters map using Compare-And-Swap.
//
// This is a lock-free operation that uses Compare-And-Swap (CAS) to atomically update
// the map pointer. The updater function receives the current map and returns a new map.
// If another goroutine modifies the map between the load and CAS, the operation retries.
//
// Performance characteristics:
//   - Fast with low contention (single CAS operation)
//   - Under high contention, creates temporary map copies that are discarded (GC pressure)
//   - Reads are lock-free and very fast (just a pointer load and dereference)
//   - Limited retries prevent infinite loops under extreme contention
//   - CAS retry attempts are tracked via router_metrics_cas_retries_total for observability
//
// Monitoring: Watch router_metrics_cas_retries_total. Sustained rates >1000/sec may indicate
// high contention. In such cases, consider reducing metric cardinality or pre-creating metrics.
//
// TODO: For workloads with extreme contention, consider adding a WithMutexBasedMetrics() option
// that uses sync.RWMutex instead of this lock-free CAS approach. This would trade latency for
// predictable GC behavior in high-contention scenarios.
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
		// CAS failed, another goroutine modified the map, retry with backoff
		if attempt > casBackoffThreshold {
			time.Sleep(time.Microsecond * time.Duration(attempt-casBackoffThreshold))
		}
	}
	c.logWarn("Failed to update custom counters after max retries", "maxRetries", maxCASRetries)
	return &MetricsUpdateError{Operation: "updateCustomCounters", Retries: maxCASRetries}
}

// updateAtomicCustomHistograms atomically updates the custom histograms map using Compare-And-Swap.
// See updateAtomicCustomCounters for implementation details and performance characteristics.
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
		// CAS failed, another goroutine modified the map, retry with backoff
		if attempt > casBackoffThreshold {
			time.Sleep(time.Microsecond * time.Duration(attempt-casBackoffThreshold))
		}
	}
	c.logWarn("Failed to update custom histograms after max retries", "maxRetries", maxCASRetries)
	return &MetricsUpdateError{Operation: "updateCustomHistograms", Retries: maxCASRetries}
}

// updateAtomicCustomGauges atomically updates the custom gauges map using Compare-And-Swap.
// See updateAtomicCustomCounters for implementation details and performance characteristics.
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
		// CAS failed, another goroutine modified the map, retry with backoff
		if attempt > casBackoffThreshold {
			time.Sleep(time.Microsecond * time.Duration(attempt-casBackoffThreshold))
		}
	}
	c.logWarn("Failed to update custom gauges after max retries", "maxRetries", maxCASRetries)
	return &MetricsUpdateError{Operation: "updateCustomGauges", Retries: maxCASRetries}
}

// getOrCreateCounter gets or creates a custom counter metric.
// This function uses atomic operations to ensure thread-safety without locks.
// Performance: Uses double-checked locking pattern to avoid unnecessary metric creation.
func (c *Config) getOrCreateCounter(ctx context.Context, name string) (metric.Int64Counter, error) {
	// First, check if counter already exists (fast path, no allocation)
	counters := c.getAtomicCustomCounters()
	if counter, exists := counters[name]; exists {
		return counter, nil
	}

	// Check if context is cancelled
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
			return nil, &MetricsLimitError{
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
// This function uses atomic operations to ensure thread-safety without locks.
// Performance: Uses double-checked locking pattern to avoid unnecessary metric creation.
func (c *Config) getOrCreateHistogram(ctx context.Context, name string) (metric.Float64Histogram, error) {
	// First, check if histogram already exists (fast path, no allocation)
	histograms := c.getAtomicCustomHistograms()
	if histogram, exists := histograms[name]; exists {
		return histogram, nil
	}

	// Check if context is cancelled
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
			return nil, &MetricsLimitError{
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
// This function uses atomic operations to ensure thread-safety without locks.
// Performance: Uses double-checked locking pattern to avoid unnecessary metric creation.
func (c *Config) getOrCreateGauge(ctx context.Context, name string) (metric.Float64Gauge, error) {
	// First, check if gauge already exists (fast path, no allocation)
	gauges := c.getAtomicCustomGauges()
	if gauge, exists := gauges[name]; exists {
		return gauge, nil
	}

	// Check if context is cancelled
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
			return nil, &MetricsLimitError{
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
