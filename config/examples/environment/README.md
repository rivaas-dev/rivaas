# Environment Variables Example

This example demonstrates how to use Config with environment variables for configuration.

## Features Demonstrated

- **Environment Variable Source**: Loading configuration from OS environment variables
- **Struct Binding**: Mapping environment variables to Go structs
- **Nested Configuration**: Handling nested configuration structures via environment variables
- **Direct Access**: Using dot notation to access configuration values
- **Type Conversion**: Automatic conversion of environment variable strings to Go types

## Configuration Structure

The example includes a simple web application configuration:

- **Server**: Host and port settings
- **Database**: Primary database connection details
- **Authentication**: JWT secret configuration
- **Features**: Debug mode flag

## Running the Example

### 1. Set Environment Variables

```bash
export WEBAPP_SERVER_HOST=localhost
export WEBAPP_SERVER_PORT=8080
export WEBAPP_DATABASE_PRIMARY_HOST=db.example.com
export WEBAPP_DATABASE_PRIMARY_PORT=5432
export WEBAPP_DATABASE_PRIMARY_DATABASE=myapp
export WEBAPP_AUTH_JWT_SECRET=your-secret-key
export WEBAPP_FEATURES_DEBUG_MODE=true
```

### 2. Run the Example

```bash
cd examples/environment
go run main.go
```

## Expected Output

```
=== Simple Configuration ===
Server: localhost:8080
Database: db.example.com:5432/myapp
Auth JWT Secret: your-secret-key
Debug Mode: true
============================

=== Direct Configuration Access ===
Server: localhost:8080
Database: db.example.com
Debug mode is enabled
```

## Environment Variable Naming Convention

Environment variables follow this naming convention:

- **Prefix**: `WEBAPP_` (configurable)
- **Hierarchy**: Underscores create nested levels
- **Conversion**: All keys are converted to lowercase

### Examples:

| Environment Variable | Configuration Path | Struct Field |
|---------------------|-------------------|--------------|
| `WEBAPP_SERVER_HOST` | `server.host` | `Server.Host` |
| `WEBAPP_DATABASE_PRIMARY_HOST` | `database.primary.host` | `Database.Primary.Host` |
| `WEBAPP_AUTH_JWT_SECRET` | `auth.jwt.secret` | `Auth.JWT.Secret` |

## Key Concepts

1. **Environment Variable Mapping**: Environment variables are mapped to configuration paths using underscores
2. **Type Safety**: String environment variables are automatically converted to appropriate Go types
3. **Nested Structures**: Deep nesting is supported through underscore-separated environment variable names
4. **Direct Access**: Use `cfg.GetString()`, `cfg.GetInt()`, etc. to access values directly

## Production Usage

This pattern is ideal for containerized applications and follows the [Twelve-Factor App methodology](https://12factor.net/config):

```bash
# Docker example
docker run -e WEBAPP_SERVER_HOST=0.0.0.0 \
           -e WEBAPP_SERVER_PORT=8080 \
           -e WEBAPP_DATABASE_PRIMARY_HOST=prod-db \
           -e WEBAPP_AUTH_JWT_SECRET=prod-secret \
           your-app
``` 