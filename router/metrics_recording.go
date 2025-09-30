package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// requestMetrics holds metrics data for a single request.
type requestMetrics struct {
	startTime   time.Time
	requestSize int64
	attributes  []attribute.KeyValue
}

// startMetrics initializes metrics collection for a request.
func (r *Router) startMetrics(c *Context, path string, isStatic bool) *requestMetrics {
	if r.metrics == nil || !r.metrics.enabled {
		return nil
	}

	// Check if path should be excluded
	if r.metrics.excludePaths[path] {
		return nil
	}

	metrics := &requestMetrics{
		startTime: time.Now(),
	}

	// Calculate request size
	if c.Request.ContentLength > 0 {
		metrics.requestSize = c.Request.ContentLength
	}

	// Build base attributes
	metrics.attributes = []attribute.KeyValue{
		attribute.String("http.method", c.Request.Method),
		attribute.String("http.route", path),
		attribute.String("http.host", c.Request.Host),
		attribute.String("service.name", r.metrics.serviceName),
		attribute.String("service.version", r.metrics.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	}

	// Record parameters if enabled
	if r.metrics.recordParams && c.paramCount > 0 {
		for i := range c.paramCount {
			metrics.attributes = append(metrics.attributes, attribute.String(
				fmt.Sprintf("http.route.param.%s", c.paramKeys[i]),
				c.paramValues[i],
			))
		}
	}

	// Record specific headers if configured
	for _, header := range r.metrics.recordHeaders {
		if value := c.Request.Header.Get(header); value != "" {
			metrics.attributes = append(metrics.attributes, attribute.String(
				fmt.Sprintf("http.request.header.%s", strings.ToLower(header)),
				value,
			))
		}
	}

	// Increment active requests atomically
	r.metrics.recordActiveRequestAtomically()
	r.metrics.activeRequests.Add(context.Background(), 1, metric.WithAttributes(metrics.attributes...))

	// Record request size
	if metrics.requestSize > 0 {
		r.metrics.requestSize.Record(context.Background(), metrics.requestSize, metric.WithAttributes(metrics.attributes...))
	}

	return metrics
}

// finishMetrics completes metrics collection for a request.
func (r *Router) finishMetrics(c *Context, requestMetrics *requestMetrics) {
	if requestMetrics == nil {
		return
	}

	// Calculate duration
	duration := time.Since(requestMetrics.startTime).Seconds()

	// Capture response status if available
	statusCode := 200 // Default to 200 if not set
	if rw, ok := c.Response.(interface{ StatusCode() int }); ok {
		statusCode = rw.StatusCode()
	}

	// Add status code to attributes
	finalAttributes := append(requestMetrics.attributes,
		attribute.Int("http.status_code", statusCode),
		attribute.String("http.status_class", getStatusClass(statusCode)),
	)

	// Record duration
	r.metrics.requestDuration.Record(context.Background(), duration, metric.WithAttributes(finalAttributes...))

	// Increment request count atomically
	r.metrics.recordRequestCountAtomically()
	r.metrics.requestCount.Add(context.Background(), 1, metric.WithAttributes(finalAttributes...))

	// Decrement active requests atomically
	r.metrics.recordActiveRequestCompleteAtomically()
	r.metrics.activeRequests.Add(context.Background(), -1, metric.WithAttributes(finalAttributes...))

	// Record error if status indicates error
	if statusCode >= 400 {
		r.metrics.recordErrorCountAtomically()
		r.metrics.errorCount.Add(context.Background(), 1, metric.WithAttributes(finalAttributes...))
	}

	// Record response size if available
	if rw, ok := c.Response.(interface{ Size() int }); ok {
		responseSize := int64(rw.Size())
		if responseSize > 0 {
			r.metrics.responseSize.Record(context.Background(), responseSize, metric.WithAttributes(finalAttributes...))
		}
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
//
// Example:
//
//	c.RecordMetric("order_processing_duration_seconds", 2.5,
//		attribute.String("currency", "USD"),
//		attribute.String("payment_method", "card"),
//	)
func (c *Context) RecordMetric(name string, value float64, attributes ...attribute.KeyValue) {
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the histogram
	histogram, err := c.router.metrics.getOrCreateHistogram(name)
	if err != nil {
		// Log error but don't fail the request
		c.router.metrics.recordCustomMetricFailureAtomically()
		return
	}

	// Record the metric
	histogram.Record(context.Background(), value, metric.WithAttributes(attributes...))
}

// IncrementCounter increments a custom counter metric with the given name.
// This method is thread-safe and uses atomic operations for optimal performance.
//
// Example:
//
//	c.IncrementCounter("orders_total",
//		attribute.String("status", "success"),
//		attribute.String("type", "online"),
//	)
func (c *Context) IncrementCounter(name string, attributes ...attribute.KeyValue) {
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the counter
	counter, err := c.router.metrics.getOrCreateCounter(name)
	if err != nil {
		// Log error but don't fail the request
		c.router.metrics.recordCustomMetricFailureAtomically()
		return
	}

	// Increment the counter
	counter.Add(context.Background(), 1, metric.WithAttributes(attributes...))
}

// SetGauge sets a custom gauge metric with the given name and value.
// This method is thread-safe and uses atomic operations for optimal performance.
//
// Example:
//
//	c.SetGauge("active_connections", 42,
//		attribute.String("service", "api"),
//	)
func (c *Context) SetGauge(name string, value float64, attributes ...attribute.KeyValue) {
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the gauge
	gauge, err := c.router.metrics.getOrCreateGauge(name)
	if err != nil {
		// Log error but don't fail the request
		c.router.metrics.recordCustomMetricFailureAtomically()
		return
	}

	// Set the gauge value
	gauge.Record(context.Background(), value, metric.WithAttributes(attributes...))
}

// recordRouteRegistration records route registration metrics.
func (r *Router) recordRouteRegistration(method, path string) {
	if r.metrics == nil || !r.metrics.enabled {
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.String("service.name", r.metrics.serviceName),
		attribute.String("service.version", r.metrics.serviceVersion),
	}

	r.metrics.routeCount.Add(context.Background(), 1, metric.WithAttributes(attributes...))
}

// serveWithMetrics handles request serving with metrics collection.
func (r *Router) serveWithMetrics(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, path string, isStatic bool) {
	// Create context with router reference
	ctx := &Context{
		Request:    req,
		Response:   w,
		index:      -1,
		paramCount: 0,
		router:     r,
	}

	// Start metrics collection
	metrics := r.startMetrics(ctx, path, isStatic)

	// Execute handlers
	for _, handler := range handlers {
		handler(ctx)
	}

	// Finish metrics collection
	r.finishMetrics(ctx, metrics)
}

// serveDynamicWithMetrics handles dynamic route serving with metrics collection.
func (r *Router) serveDynamicWithMetrics(c *Context, handlers []HandlerFunc, path string) {
	// Start metrics collection
	metrics := r.startMetrics(c, path, false)

	// Execute handlers
	for _, handler := range handlers {
		handler(c)
	}

	// Finish metrics collection
	r.finishMetrics(c, metrics)
}

// serveWithTracingAndMetrics handles request serving with both tracing and metrics.
func (r *Router) serveWithTracingAndMetrics(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, path string, isStatic bool) {
	// Start tracing
	c := &Context{Request: req, Response: w, router: r}
	r.startTracing(c, path, isStatic)

	// Start metrics collection
	metrics := r.startMetrics(c, path, isStatic)

	// Execute handlers
	for _, handler := range handlers {
		handler(c)
	}

	// Finish metrics collection
	r.finishMetrics(c, metrics)

	// Finish tracing
	r.finishTracing(c)
}

// serveDynamicWithTracingAndMetrics handles dynamic route serving with both tracing and metrics.
func (r *Router) serveDynamicWithTracingAndMetrics(c *Context, handlers []HandlerFunc, path string) {
	// Start metrics collection
	metrics := r.startMetrics(c, path, false)

	// Execute handlers
	for _, handler := range handlers {
		handler(c)
	}

	// Finish metrics collection
	r.finishMetrics(c, metrics)
}

// Atomic operations for built-in metrics
func (m *MetricsConfig) recordRequestCountAtomically() {
	atomic.AddInt64(&m.atomicRequestCount, 1)
}

func (m *MetricsConfig) recordActiveRequestAtomically() {
	atomic.AddInt64(&m.atomicActiveRequests, 1)
}

func (m *MetricsConfig) recordActiveRequestCompleteAtomically() {
	atomic.AddInt64(&m.atomicActiveRequests, -1)
}

func (m *MetricsConfig) recordErrorCountAtomically() {
	atomic.AddInt64(&m.atomicErrorCount, 1)
}

func (m *MetricsConfig) recordCustomMetricFailureAtomically() {
	atomic.AddInt64(&m.atomicErrorCount, 1)
}

// Getters for atomic counters
func (m *MetricsConfig) getAtomicRequestCount() int64 {
	return atomic.LoadInt64(&m.atomicRequestCount)
}

func (m *MetricsConfig) getAtomicActiveRequests() int64 {
	return atomic.LoadInt64(&m.atomicActiveRequests)
}

func (m *MetricsConfig) getAtomicErrorCount() int64 {
	return atomic.LoadInt64(&m.atomicErrorCount)
}

func (m *MetricsConfig) getAtomicContextPoolHits() int64 {
	return atomic.LoadInt64(&m.atomicContextPoolHits)
}

func (m *MetricsConfig) getAtomicContextPoolMisses() int64 {
	return atomic.LoadInt64(&m.atomicContextPoolMisses)
}

// Atomic custom metrics operations
func (m *MetricsConfig) getAtomicCustomCounters() map[string]metric.Int64Counter {
	ptr := atomic.LoadPointer(&m.atomicCustomCounters)
	return *(*map[string]metric.Int64Counter)(ptr)
}

func (m *MetricsConfig) getAtomicCustomHistograms() map[string]metric.Float64Histogram {
	ptr := atomic.LoadPointer(&m.atomicCustomHistograms)
	return *(*map[string]metric.Float64Histogram)(ptr)
}

func (m *MetricsConfig) getAtomicCustomGauges() map[string]metric.Float64Gauge {
	ptr := atomic.LoadPointer(&m.atomicCustomGauges)
	return *(*map[string]metric.Float64Gauge)(ptr)
}

// updateAtomicCustomCounters atomically updates the custom counters map
func (m *MetricsConfig) updateAtomicCustomCounters(updater func(map[string]metric.Int64Counter) map[string]metric.Int64Counter) {
	for {
		currentPtr := atomic.LoadPointer(&m.atomicCustomCounters)
		current := *(*map[string]metric.Int64Counter)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&m.atomicCustomCounters, currentPtr, unsafe.Pointer(&newMap)) {
			return
		}
	}
}

// updateAtomicCustomHistograms atomically updates the custom histograms map
func (m *MetricsConfig) updateAtomicCustomHistograms(updater func(map[string]metric.Float64Histogram) map[string]metric.Float64Histogram) {
	for {
		currentPtr := atomic.LoadPointer(&m.atomicCustomHistograms)
		current := *(*map[string]metric.Float64Histogram)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&m.atomicCustomHistograms, currentPtr, unsafe.Pointer(&newMap)) {
			return
		}
	}
}

// updateAtomicCustomGauges atomically updates the custom gauges map
func (m *MetricsConfig) updateAtomicCustomGauges(updater func(map[string]metric.Float64Gauge) map[string]metric.Float64Gauge) {
	for {
		currentPtr := atomic.LoadPointer(&m.atomicCustomGauges)
		current := *(*map[string]metric.Float64Gauge)(currentPtr)
		newMap := updater(current)
		if atomic.CompareAndSwapPointer(&m.atomicCustomGauges, currentPtr, unsafe.Pointer(&newMap)) {
			return
		}
	}
}

// getOrCreateCounter gets or creates a custom counter metric
func (m *MetricsConfig) getOrCreateCounter(name string) (metric.Int64Counter, error) {
	// Check if counter already exists
	counters := m.getAtomicCustomCounters()
	if counter, exists := counters[name]; exists {
		return counter, nil
	}

	// Check total metrics count across all types
	histograms := m.getAtomicCustomHistograms()
	gauges := m.getAtomicCustomGauges()
	totalMetrics := len(counters) + len(histograms) + len(gauges)
	if totalMetrics >= m.maxCustomMetrics {
		return nil, fmt.Errorf("custom metrics limit reached (%d)", m.maxCustomMetrics)
	}

	// Create new counter
	counter, err := m.meter.Int64Counter(
		name,
		metric.WithDescription("Custom counter metric"),
	)
	if err != nil {
		return nil, err
	}

	// Atomically update the map
	m.updateAtomicCustomCounters(func(current map[string]metric.Int64Counter) map[string]metric.Int64Counter {
		// Double-check if another goroutine created it
		if _, exists := current[name]; exists {
			return current
		}
		// Create new map with the counter
		newMap := make(map[string]metric.Int64Counter, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = counter
		return newMap
	})

	return counter, nil
}

// getOrCreateHistogram gets or creates a custom histogram metric
func (m *MetricsConfig) getOrCreateHistogram(name string) (metric.Float64Histogram, error) {
	// Check if histogram already exists
	histograms := m.getAtomicCustomHistograms()
	if histogram, exists := histograms[name]; exists {
		return histogram, nil
	}

	// Check total metrics count across all types
	counters := m.getAtomicCustomCounters()
	gauges := m.getAtomicCustomGauges()
	totalMetrics := len(counters) + len(histograms) + len(gauges)
	if totalMetrics >= m.maxCustomMetrics {
		return nil, fmt.Errorf("custom metrics limit reached (%d)", m.maxCustomMetrics)
	}

	// Create new histogram
	histogram, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription("Custom histogram metric"),
	)
	if err != nil {
		return nil, err
	}

	// Atomically update the map
	m.updateAtomicCustomHistograms(func(current map[string]metric.Float64Histogram) map[string]metric.Float64Histogram {
		// Double-check if another goroutine created it
		if _, exists := current[name]; exists {
			return current
		}
		// Create new map with the histogram
		newMap := make(map[string]metric.Float64Histogram, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = histogram
		return newMap
	})

	return histogram, nil
}

// getOrCreateGauge gets or creates a custom gauge metric
func (m *MetricsConfig) getOrCreateGauge(name string) (metric.Float64Gauge, error) {
	// Check if gauge already exists
	gauges := m.getAtomicCustomGauges()
	if gauge, exists := gauges[name]; exists {
		return gauge, nil
	}

	// Check total metrics count across all types
	counters := m.getAtomicCustomCounters()
	histograms := m.getAtomicCustomHistograms()
	totalMetrics := len(counters) + len(histograms) + len(gauges)
	if totalMetrics >= m.maxCustomMetrics {
		return nil, fmt.Errorf("custom metrics limit reached (%d)", m.maxCustomMetrics)
	}

	// Create new gauge
	gauge, err := m.meter.Float64Gauge(
		name,
		metric.WithDescription("Custom gauge metric"),
	)
	if err != nil {
		return nil, err
	}

	// Atomically update the map
	m.updateAtomicCustomGauges(func(current map[string]metric.Float64Gauge) map[string]metric.Float64Gauge {
		// Double-check if another goroutine created it
		if _, exists := current[name]; exists {
			return current
		}
		// Create new map with the gauge
		newMap := make(map[string]metric.Float64Gauge, len(current)+1)
		for k, v := range current {
			newMap[k] = v
		}
		newMap[name] = gauge
		return newMap
	})

	return gauge, nil
}
