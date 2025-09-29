# Full-Featured Rivaas App Example

This example demonstrates a **production-ready** Rivaas application with complete observability, showcasing:

- ✅ **Multi-exporter tracing** (stdout for dev, OTLP for prod)
- ✅ **Metrics collection** (Prometheus)
- ✅ **Environment-based configuration** (dev/prod modes)
- ✅ **Full middleware stack** (logger, recovery, CORS, timeout, request ID)
- ✅ **Custom metrics and tracing** in handlers
- ✅ **Graceful shutdown** with proper cleanup
- ✅ **RESTful API patterns** with route groups

## Running in Different Modes

### Development Mode (Stdout Tracing)

Traces are printed to stdout in pretty JSON format:

```bash
cd app/examples/02-full-featured
go run main.go
```

**Default settings:**

- Environment: `development`
- Tracing: Stdout exporter (see traces in terminal)
- Metrics: Prometheus on `:9090`
- Sampling: 100% of requests
- Logger: Enabled

### Production Mode (OTLP Tracing)

Traces sent to Jaeger/Tempo via OTLP:

```bash
# First, start Jaeger:
docker run -d --name jaeger \
  -p 4317:4317 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest

# Then run the app in production mode:
ENVIRONMENT=production \
SERVICE_NAME=my-prod-api \
OTLP_ENDPOINT=localhost:4317 \
go run main.go
```

**Production settings:**

- Environment: `production`
- Tracing: OTLP exporter to Jaeger/Tempo
- Metrics: Prometheus on `:9090`
- Sampling: 10% of requests
- Logger: Disabled (use custom logging in prod)

### Testing Mode (No Tracing)

Disable tracing completely:

```bash
ENVIRONMENT=testing go run main.go
```

## Environment Variables

Configure the app via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ENVIRONMENT` | `development` | Environment mode: `development`, `production`, or `testing` |
| `SERVICE_NAME` | `full-featured-api` | Service name for observability |
| `SERVICE_VERSION` | `v1.0.0` | Service version |
| `PORT` | `:8080` | HTTP server port |
| `METRICS_PORT` | `:9090` | Prometheus metrics port |
| `OTLP_ENDPOINT` | `localhost:4317` | OTLP collector endpoint (production only) |

## API Endpoints

### Root & Health

```bash
# API info
curl http://localhost:8080/

# Health check (excluded from tracing)
curl http://localhost:8080/health
```

### User Operations

```bash
# Get user (with full tracing)
curl http://localhost:8080/users/123

# Response includes trace_id and span_id for distributed tracing
```

### Order Operations

```bash
# Create order (with custom metrics)
curl -X POST http://localhost:8080/orders

# Check metrics to see order processing duration
curl http://localhost:9090/metrics | grep order_processing
```

### API v1 Group

```bash
# Status endpoint
curl http://localhost:8080/api/v1/status

# Products list
curl http://localhost:8080/api/v1/products
```

### Error Handling

```bash
# Trigger error (see recovery middleware in action)
curl http://localhost:8080/error
```

## Observability Features

### Metrics (Prometheus)

View metrics at:

```
http://localhost:9090/metrics
```

**Built-in metrics:**

- `http_request_duration_seconds` - Request latency histogram
- `http_requests_total` - Total requests counter
- `http_active_requests` - Active requests gauge

**Custom metrics:**

- `user_lookups_total` - User lookup counter
- `orders_total` - Orders created counter
- `order_processing_duration_seconds` - Order processing histogram
- `errors_total` - Errors counter

### Tracing

**Development (Stdout):**
Traces appear in your terminal as pretty JSON:

```json
{
  "Name": "GET /users/:id",
  "SpanContext": {
    "TraceID": "8d5c9f9b8c5e9f8b7c6d5e4f3a2b1c0d",
    "SpanID": "1a2b3c4d5e6f7a8b"
  },
  "Attributes": [
    {"Key": "user.id", "Value": "123"},
    {"Key": "operation.type", "Value": "read"}
  ],
  "Events": [
    {"Name": "user_lookup_started"},
    {"Name": "user_found"}
  ]
}
```

**Production (OTLP):**
View traces in Jaeger UI:

```
http://localhost:16686
```

Select "full-featured-api" from the service dropdown to see all traces.

## What Makes This Example "Full-Featured"?

### 1. Environment-Aware Configuration

Automatically adapts based on `ENVIRONMENT`:

- **Development**: Stdout tracing, logger enabled, 100% sampling
- **Production**: OTLP tracing, logger disabled, 10% sampling
- **Testing**: Noop tracing, minimal overhead

### 2. Complete Observability Stack

- **Metrics**: Prometheus with custom metrics
- **Tracing**: OpenTelemetry with multiple exporters
- **Logging**: Environment-aware request logging
- **Request IDs**: For log correlation

### 3. Production-Ready Patterns

- Graceful shutdown with configurable timeout
- Proper error handling throughout
- Context cancellation support
- Resource cleanup on exit
- Configuration validation

### 4. Middleware Stack

- **Recovery**: Panic recovery with stack traces
- **Logger**: Request logging (dev only)
- **RequestID**: Unique request IDs
- **CORS**: Cross-origin support
- **Timeout**: Request timeout handling

### 5. RESTful API Design

- Route groups (`/api/v1`)
- RESTful verbs (GET, POST, etc.)
- Path parameters (`:id`)
- Proper HTTP status codes

## Switching Between Exporters

The example shows how to switch tracing exporters at runtime:

```go
// Development: See traces in terminal
ENVIRONMENT=development go run main.go

// Production: Send to Jaeger/Tempo
ENVIRONMENT=production OTLP_ENDPOINT=collector:4317 go run main.go

// Testing: No tracing overhead
ENVIRONMENT=testing go run main.go
```

This is exactly like switching metrics providers:

```go
metrics.WithProvider(metrics.PrometheusProvider)  // or OTLPProvider, StdoutProvider
tracing.WithExporter(tracing.StdoutExporter)      // or OTLPExporter, NoopExporter
```

## Testing the Full Stack

### 1. Start the app

```bash
go run main.go
```

### 2. Make requests

```bash
curl http://localhost:8080/
curl http://localhost:8080/users/123
curl -X POST http://localhost:8080/orders
```

### 3. Check metrics

```bash
curl http://localhost:9090/metrics
```

### 4. View traces

**Development mode:** Check your terminal output

**Production mode:** Open Jaeger UI at <http://localhost:16686>

## Production Deployment Checklist

When deploying to production:

- [ ] Set `ENVIRONMENT=production`
- [ ] Configure `OTLP_ENDPOINT` to your collector
- [ ] Use HTTPS for OTLP (set `otlpInsecure: false`)
- [ ] Set appropriate `SERVICE_NAME` and `SERVICE_VERSION`
- [ ] Configure sampling rate (default 10%)
- [ ] Set up log aggregation (disable stdout logger)
- [ ] Configure proper timeout values
- [ ] Set up alerting on metrics
- [ ] Test graceful shutdown

## Comparison with Basic Example

**Example 01 (Quick Start):**

- Minimal setup
- No observability
- No middleware
- Good for learning basics

**Example 02 (This Example):**

- Full observability stack
- Production-ready
- Complete middleware
- Environment-aware
- Real-world patterns

## Learn More

- [Rivaas App Documentation](../../README.md)
- [Metrics Package](../../../metrics/README.md)
- [Tracing Package](../../../tracing/README.md)
- [Tracing Exporters](../../../tracing/EXPORTERS.md)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
