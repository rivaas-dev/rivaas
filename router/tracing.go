package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds OpenTelemetry tracing configuration.
type TracingConfig struct {
	enabled        bool
	serviceName    string
	serviceVersion string
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
	excludePaths   map[string]bool
	sampleRate     float64
	recordParams   bool
	recordHeaders  []string
}

// WithTracing enables OpenTelemetry tracing with default configuration.
func WithTracing() RouterOption {
	return func(r *Router) {
		r.tracing = &TracingConfig{
			enabled:        true,
			serviceName:    "rivaas-router",
			serviceVersion: "1.0.0",
			tracer:         otel.Tracer("github.com/rivaas-dev/rivaas/router"),
			propagator:     otel.GetTextMapPropagator(),
			excludePaths:   make(map[string]bool),
			sampleRate:     1.0,
			recordParams:   true,
		}
	}
}

// WithTracingServiceName sets the service name for tracing.
func WithTracingServiceName(name string) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.serviceName = name
		}
	}
}

// WithTracingServiceVersion sets the service version for tracing.
func WithTracingServiceVersion(version string) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.serviceVersion = version
		}
	}
}

// WithTracingSampleRate sets the sampling rate (0.0 to 1.0).
func WithTracingSampleRate(rate float64) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.sampleRate = rate
		}
	}
}

// WithTracingExcludePaths excludes specific paths from tracing.
func WithTracingExcludePaths(paths ...string) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			for _, path := range paths {
				r.tracing.excludePaths[path] = true
			}
		}
	}
}

// WithTracingHeaders records specific headers as span attributes.
func WithTracingHeaders(headers ...string) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.recordHeaders = headers
		}
	}
}

// WithTracingDisableParams disables recording URL parameters.
func WithTracingDisableParams() RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.recordParams = false
		}
	}
}

// WithCustomTracer allows using a custom OpenTelemetry tracer.
func WithCustomTracer(tracer trace.Tracer) RouterOption {
	return func(r *Router) {
		if r.tracing != nil {
			r.tracing.tracer = tracer
		}
	}
}

// TraceID returns the current trace ID from the active span.
// Returns an empty string if tracing is not active.
func (c *Context) TraceID() string {
	if c.span != nil {
		return c.span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the current span ID from the active span.
// Returns an empty string if tracing is not active.
func (c *Context) SpanID() string {
	if c.span != nil {
		return c.span.SpanContext().SpanID().String()
	}
	return ""
}

// SetSpanAttribute adds an attribute to the current span.
// This is a no-op if tracing is not active.
func (c *Context) SetSpanAttribute(key string, value interface{}) {
	if c.span != nil {
		c.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
	}
}

// AddSpanEvent adds an event to the current span with optional attributes.
// This is a no-op if tracing is not active.
func (c *Context) AddSpanEvent(name string, attrs ...attribute.KeyValue) {
	if c.span != nil {
		c.span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
// If tracing is not enabled, it returns the request context for proper cancellation support.
func (c *Context) TraceContext() context.Context {
	if c.traceCtx != nil {
		return c.traceCtx
	}
	// Use request context as parent for proper cancellation support
	if c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}

// startTracing initializes OpenTelemetry tracing for the request.
func (r *Router) startTracing(c *Context, path string, isStatic bool) {
	if r.tracing == nil || !r.tracing.enabled {
		return
	}

	// Extract trace context from headers
	ctx := r.tracing.propagator.Extract(context.Background(), propagation.HeaderCarrier(c.Request.Header))

	// Start span
	spanName := fmt.Sprintf("%s %s", c.Request.Method, path)
	ctx, span := r.tracing.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))

	c.traceCtx = ctx
	c.span = span

	// Set standard attributes
	span.SetAttributes(
		attribute.String("http.method", c.Request.Method),
		attribute.String("http.url", c.Request.URL.String()),
		attribute.String("http.scheme", c.Request.URL.Scheme),
		attribute.String("http.host", c.Request.Host),
		attribute.String("http.route", path),
		attribute.String("http.user_agent", c.Request.UserAgent()),
		attribute.String("service.name", r.tracing.serviceName),
		attribute.String("service.version", r.tracing.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	)

	// Record parameters if enabled
	if r.tracing.recordParams && c.paramCount > 0 {
		for i := 0; i < c.paramCount; i++ {
			span.SetAttributes(attribute.String(
				fmt.Sprintf("http.route.param.%s", c.paramKeys[i]),
				c.paramValues[i],
			))
		}
	}

	// Record specific headers if configured
	for _, header := range r.tracing.recordHeaders {
		if value := c.Request.Header.Get(header); value != "" {
			span.SetAttributes(attribute.String(
				fmt.Sprintf("http.request.header.%s", strings.ToLower(header)),
				value,
			))
		}
	}

	// Inject trace context into response headers
	r.tracing.propagator.Inject(ctx, propagation.HeaderCarrier(c.Response.Header()))
}

// finishTracing completes the OpenTelemetry span.
func (r *Router) finishTracing(c *Context) {
	if c.span == nil {
		return
	}

	// Capture response status if available
	if rw, ok := c.Response.(interface{ StatusCode() int }); ok {
		statusCode := rw.StatusCode()
		c.span.SetAttributes(attribute.Int("http.status_code", statusCode))

		if statusCode >= 400 {
			c.span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		} else {
			c.span.SetStatus(codes.Ok, "")
		}
	}

	c.span.End()
}

// serveWithTracing handles static routes with tracing.
func (r *Router) serveWithTracing(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, path string, isStatic bool) {
	ctx := &Context{
		Request:    req,
		Response:   w,
		index:      -1,
		paramCount: 0,
		router:     r,
	}

	r.startTracing(ctx, path, isStatic)
	defer r.finishTracing(ctx)

	for i := 0; i < len(handlers); i++ {
		handlers[i](ctx)
	}
}

// serveDynamicWithTracing handles dynamic routes with tracing.
func (r *Router) serveDynamicWithTracing(c *Context, handlers []HandlerFunc, path string) {
	r.startTracing(c, path, false)
	defer r.finishTracing(c)

	for i := 0; i < len(handlers); i++ {
		handlers[i](c)
	}
}
