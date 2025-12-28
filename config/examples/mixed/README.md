# Mixed YAML + Environment Variables Example

This example demonstrates how to use Config package with both YAML configuration files and environment variables, showing configuration precedence where environment variables override YAML defaults.

## Overview

This is a practical example of a common configuration pattern:
- **YAML file**: Provides default values for development
- **Environment variables**: Override defaults for different environments (staging, production)

## Features Demonstrated

- **Configuration Precedence**: Environment variables override YAML file values
- **Mixed Sources**: Loading from both YAML file and environment variables
- **Struct Binding**: Mapping configuration to Go structs
- **Direct Access**: Using dot notation to access configuration values

## Configuration Structure

The example includes a complete web application configuration with:

- **Server**: Host, port, timeouts, TLS settings
- **Database**: Primary and replica connections, connection pooling
- **Redis**: Connection settings and timeouts
- **Authentication**: JWT secrets and token duration
- **Logging**: Level, format, and output file
- **Monitoring**: Metrics and health check settings
- **Features**: Feature flags for rate limiting, caching, and debug mode

## Running the Example

### 1. YAML Configuration

The example includes a `config.yaml` file with development defaults:

```yaml
server:
  host: "localhost"
  port: 3000
  read:
    timeout: "30s"
  # ... more defaults
```

### 2. Environment Variables (Optional)

You can override YAML defaults with environment variables:

```bash
# Override server port
export WEBAPP_SERVER_PORT=8080

# Override database host
export WEBAPP_DATABASE_PRIMARY_HOST=production-db.example.com

# Override debug mode
export WEBAPP_FEATURES_DEBUG_MODE=false
```

### 3. Run the Example

```bash
cd examples/env_mixed
go run main.go
```

## Expected Output

### Without Environment Variables

```
=== Web Application Configuration (YAML + Environment Variables) ===
Server: localhost:3000
  Read Timeout: 30s
  Write Timeout: 30s
  TLS Enabled: false

Database Primary: localhost:5432/myapp_dev
Database Replica: localhost:5432/myapp_dev
Database Pool: MaxOpen=10, MaxIdle=5, MaxLifetime=5m0s

Redis: localhost:6379 (DB: 0)
Redis Timeout: 5s

Auth Token Duration: 24h0m0s
Logging Level: debug, Format: text

Monitoring Enabled: false

Features:
  Rate Limit: false
  Cache: false
  Debug Mode: true
=====================================
```

### With Environment Variables

```bash
export WEBAPP_SERVER_PORT=8080
export WEBAPP_DATABASE_PRIMARY_HOST=prod-db.example.com
export WEBAPP_FEATURES_DEBUG_MODE=false
go run main.go
```

Output shows environment variables overriding YAML defaults:

```
=== Web Application Configuration (YAML + Environment Variables) ===
Server: localhost:8080  # Port overridden by environment variable
  Read Timeout: 30s     # From YAML (not overridden)
  Write Timeout: 30s    # From YAML (not overridden)
  TLS Enabled: false    # From YAML (not overridden)

Database Primary: prod-db.example.com:5432/myapp_dev  # Host overridden
Database Replica: localhost:5432/myapp_dev            # From YAML
Database Pool: MaxOpen=10, MaxIdle=5, MaxLifetime=5m0s # From YAML

# ... rest of configuration

Features:
  Rate Limit: false
  Cache: false
  Debug Mode: false  # Overridden by environment variable
=====================================
```

## Configuration Precedence

The example demonstrates clear configuration precedence:

1. **YAML File (`config.yaml`)**: Provides default values
2. **Environment Variables (`WEBAPP_*`)**: Override YAML defaults

### Examples:

| YAML Value | Environment Variable | Final Value | Source |
|------------|---------------------|-------------|---------|
| `server.port: 3000` | `WEBAPP_SERVER_PORT=8080` | `8080` | Environment |
| `server.host: localhost` | (not set) | `localhost` | YAML |
| `features.debug_mode: true` | `WEBAPP_FEATURES_DEBUG_MODE=false` | `false` | Environment |

## Environment Variable Naming Convention

Environment variables follow the same naming convention as the original example:

- **Prefix**: `WEBAPP_` (configurable)
- **Hierarchy**: Underscores create nested levels
- **Conversion**: All keys are converted to lowercase

### Examples:

| Environment Variable | Configuration Path | Struct Field |
|---------------------|-------------------|--------------|
| `WEBAPP_SERVER_PORT` | `server.port` | `Server.Port` |
| `WEBAPP_DATABASE_PRIMARY_HOST` | `database.primary.host` | `Database.Primary.Host` |
| `WEBAPP_AUTH_JWT_SECRET` | `auth.jwt.secret` | `Auth.JWTSecret` |

## Production Usage

This pattern is ideal for production deployments:

1. **Development**: Use YAML defaults
2. **Staging**: Override with staging environment variables
3. **Production**: Override with production environment variables

### Example Production Setup

```bash
# Production environment variables
export WEBAPP_SERVER_HOST=0.0.0.0
export WEBAPP_SERVER_PORT=8080
export WEBAPP_DATABASE_PRIMARY_HOST=prod-db.example.com
export WEBAPP_DATABASE_PRIMARY_PASSWORD=prod-secret-password
export WEBAPP_AUTH_JWT_SECRET=super-secret-production-key
export WEBAPP_FEATURES_DEBUG_MODE=false
export WEBAPP_MONITORING_ENABLED=true
```

### Docker Example

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mixed_example examples/env_mixed/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mixed_example .
COPY --from=builder /app/examples/env_mixed/config.yaml .
CMD ["./mixed_example"]
```

```bash
docker build -t config-mixed-example .
docker run -e WEBAPP_SERVER_PORT=8080 -e WEBAPP_DATABASE_PRIMARY_HOST=prod-db config-mixed-example
```

## Benefits

1. **Development Friendly**: YAML provides readable defaults
2. **Environment Specific**: Environment variables for different deployments
3. **Secure**: Sensitive values (passwords, secrets) can be provided via environment variables
4. **Flexible**: Easy to override specific values without changing files
5. **Twelve-Factor Compliant**: Follows the [Twelve-Factor App methodology](https://12factor.net/config) 