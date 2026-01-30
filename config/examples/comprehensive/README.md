# Comprehensive Example

This example demonstrates advanced Config package features including mixed configuration sources, validation, and complex nested structures.

## Features Demonstrated

- **Mixed Configuration Sources**: YAML file for defaults + environment variables for overrides
- **Configuration Precedence**: Environment variables override YAML file values
- **Environment Variable Mapping**: How environment variables map to nested struct fields
- **Struct Validation**: Implementing the `Validator` interface to ensure required fields
- **Complex Nested Structures**: Handling deeply nested configuration
- **Type Safety**: Using different data types (string, int, bool, time.Duration)
- **Direct Configuration Access**: Using dot notation to access configuration values
- **Comprehensive Testing**: Multiple test files demonstrating different scenarios

## Configuration Structure

The example includes configuration for a complete web application:

- **Server**: Host, port, timeouts, TLS settings
- **Database**: Primary and replica connections, connection pooling
- **Redis**: Connection settings and timeouts
- **Authentication**: JWT secrets and token duration
- **Logging**: Level, format, and output file
- **Monitoring**: Metrics and health check settings
- **Features**: Feature flags for rate limiting, caching, and debug mode

## Running the Example

### 1. Configuration Files

The example includes a `config.yaml` file with default values:

```yaml
server:
  host: "localhost"
  port: 3000
  # ... more defaults
```

### 2. Set Environment Variables (Optional)

```bash
# Server Configuration
export WEBAPP_SERVER_HOST=0.0.0.0
export WEBAPP_SERVER_PORT=8080
export WEBAPP_SERVER_READ_TIMEOUT=30s
export WEBAPP_SERVER_WRITE_TIMEOUT=30s
export WEBAPP_SERVER_TLS_ENABLED=false

# Database Configuration
export WEBAPP_DATABASE_PRIMARY_HOST=localhost
export WEBAPP_DATABASE_PRIMARY_PORT=5432
export WEBAPP_DATABASE_PRIMARY_DATABASE=myapp
export WEBAPP_DATABASE_PRIMARY_USERNAME=postgres
export WEBAPP_DATABASE_PRIMARY_PASSWORD=secret123
export WEBAPP_DATABASE_PRIMARY_SSL_MODE=disable

# ... more environment variables
```

### 3. Run the Example

```bash
cd examples/comprehensive
go run main.go
```

### 4. Run Tests

```bash
cd examples/comprehensive
go test -v
```

## Expected Output

The program will output the complete configuration that was loaded from both YAML and environment variables:

```
=== Web Application Configuration (YAML + Environment Variables) ===
Server: 0.0.0.0:8080
  Read Timeout: 30s
  Write Timeout: 30s
  TLS Enabled: false

Database Primary: localhost:5432/myapp
Database Replica: replica.example.com:5432/myapp
Database Pool: MaxOpen=25, MaxIdle=5, MaxLifetime=5m0s

Redis: localhost:6379 (DB: 0)
Redis Timeout: 5s

Auth Token Duration: 24h0m0s
Logging Level: info, Format: json
Logging Output: /var/log/myapp.log

Monitoring Enabled: true
Metrics Port: 9090
Health Path: /health

Features:
  Rate Limit: true
  Cache: true
  Debug Mode: false
=====================================
```

## Configuration Precedence

The example demonstrates configuration precedence where environment variables override YAML file values:

1. **YAML File (`config.yaml`)**: Provides default values
2. **Environment Variables (`WEBAPP_*`)**: Override YAML defaults

### Examples:

| YAML Value | Environment Variable | Final Value | Source |
|------------|---------------------|-------------|---------|
| `server.port: 3000` | `WEBAPP_SERVER_PORT=8080` | `8080` | Environment |
| `server.host: localhost` | (not set) | `localhost` | YAML |
| `features.debug_mode: true` | `WEBAPP_FEATURES_DEBUG_MODE=false` | `false` | Environment |

## Test Files

The example includes comprehensive tests:

- **`main_test.go`**: Tests environment variable loading and struct binding
- **`mixed_test.go`**: Tests mixed YAML + environment variable configuration
- **`debug_test.go`**: Debug tests for troubleshooting environment variable loading

## Production Usage

For production deployments, you might want to:

1. **Use the mixed approach (recommended)**:
   ```go
   cfg, err := config.New(
       config.WithFile("config.yaml"),     // Defaults
       config.WithEnv("WEBAPP_"),                    // Overrides
       config.WithBinding(&config),
   )
   ```

2. **Environment-specific YAML files**:
   ```go
   cfg, err := config.New(
       config.WithFile("config.production.yaml"),
       config.WithEnv("WEBAPP_"),
       config.WithBinding(&config),
   )
   ```

3. **Add more validation rules**:
   ```go
   func (c *WebAppConfig) Validate() error {
       // Add more validation logic
       if c.Server.TLS.Enabled && c.Server.TLS.CertFile == "" {
           return errors.New("TLS certificate file is required when TLS is enabled")
       }
       return nil
   }
   ```

4. **Use secrets management** for sensitive values like database passwords and JWT secrets

## Docker Example

You can also run this example in Docker:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o comprehensive examples/comprehensive/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/comprehensive .
CMD ["./comprehensive"]
```

```bash
docker build -t config-comprehensive .
docker run -e WEBAPP_SERVER_HOST=0.0.0.0 -e WEBAPP_SERVER_PORT=8080 config-comprehensive
``` 