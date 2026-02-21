# Rivaas Router Examples

Pure router and middleware feature demonstrations.

**No domain logic here** - these examples focus solely on router and middleware capabilities.

For complete, production-ready applications with business logic, see `app/examples/`.

## 📚 Examples Overview

The examples are organized in a progressive learning path:

### 1. [Hello World](./01-hello-world/)

**Start here!** The simplest possible Rivaas application.

```bash
cd 01-hello-world && go run main.go
curl http://localhost:8080/
```

**Learn:** Basic router setup, simple JSON responses

---

### 2. [Routing](./02-routing/)

Routes, parameters, HTTP methods, and route groups.

```bash
cd 02-routing && go run main.go
curl http://localhost:8080/users/123
```

**Learn:** Path parameters, route groups, nested groups, HTTP methods (GET, POST, PUT, DELETE)

---

### 3. [Complete REST API](./03-complete-rest-api/)

Production-ready CRUD API with validation and error handling.

```bash
cd 03-complete-rest-api && go run main.go
curl http://localhost:8080/api/v1/users
curl -X POST http://localhost:8080/api/v1/users -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

**Learn:**

- Request binding (Bind, BindQuery, BindParams)
- Structured error responses with proper HTTP codes
- Input validation patterns
- Business logic separation
- Nested resources (users/:id/posts)

---

### 4. [Middleware Stack](./04-middleware-stack/)

Complete middleware guide for production.

```bash
cd 04-middleware-stack && go run main.go
curl http://localhost:8080/api/data
curl -H 'Authorization: Bearer token123' http://localhost:8080/protected/secret
```

**Learn:**

- Common middleware (auth, logging, recovery, CORS)
- Middleware patterns (global, group, per-route, conditional)
- Custom middleware creation with configuration
- Middleware ordering and composition
- Rate limiting, request tracking, validation

---

### 5. [Advanced Routing](./05-advanced-routing/)

Advanced features for mature APIs.

```bash
cd 05-advanced-routing && go run main.go
curl http://localhost:8080/users/123           # ✓ Valid (numeric)
curl http://localhost:8080/users/abc           # ✗ Invalid (not numeric)
curl -H "API-Version: v1" http://localhost:8080/users
curl http://localhost:8080/files/static/image.jpg
```

**Learn:**

- Route constraints (UUID, numeric, alpha, regex)
- Proper HTTP method semantics (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS)
- Wildcard routes for file serving
- Route introspection

---

### 6. [Content and Rendering](./06-content-and-rendering/)

Request handling and response rendering.

```bash
cd 06-content-and-rendering && go run main.go
curl -H "Accept: application/json" http://localhost:8080/api/user
curl http://localhost:8080/json/pure
curl http://localhost:8080/benchmark?format=pure
```

**Learn:**

- Content negotiation (Accept headers for JSON/XML/HTML)
- Rendering methods (JSON, PureJSON, YAML, etc.)
- Performance comparisons (PureJSON 35% faster, Data 98% faster)
- Context helpers (Query, Param, Header, Cookie, ClientIP)
- Streaming responses with DataFromReader
- JSONP for legacy support

---

### 7. [API Versioning](./07-versioning/)

Comprehensive API versioning strategies and migration patterns.

```bash
cd 07-versioning && go run main.go
curl -H 'API-Version: v2' http://localhost:8080/users
curl 'http://localhost:8080/users?version=v2'
curl http://localhost:8080/v2/users
```

**Learn:**

- All versioning methods (header, query, path, accept)
- Version-specific handlers and routes
- Migration patterns (v1 → v2 → v3)
- Deprecation warnings and sunset dates
- Version validation and observability
- Algorithm details
- Best practices for API evolution

**See Also:** [Versioning Guide](../../docs/VERSIONING.md) for comprehensive documentation

---

## 🚀 Quick Start

1. **Choose an example** based on what you want to learn
2. **Navigate to the directory:**

   ```bash
   cd router/examples/01-hello-world
   ```

3. **Run the example:**

   ```bash
   go run main.go
   ```

4. **Test with curl** (commands are shown in each example's output)

## 📖 Learning Path

### Progressive Learning

#### Core Routing (Start Here)

1. Start with **01-hello-world** to understand the basics
2. Move to **02-routing** to learn about routes and parameters

#### Practical Application

1. Build **03-complete-rest-api** for production-ready CRUD patterns
2. Study **04-middleware-stack** for complete middleware guide

#### Advanced Features

1. Master **05-advanced-routing** for constraints and HTTP methods
2. Explore **06-content-and-rendering** for flexible response handling
3. Study **07-versioning** for API evolution and migration patterns

## 🔧 Common Patterns

### Creating a Router

```go
r := router.New()
```

### With Options

```go
r := router.New(
    router.WithVersioning(
        router.WithHeaderVersioning("API-Version"),
        router.WithDefaultVersion("v1"),
    ),
)
```

### Adding Routes

```go
r.GET("/users/:id", handler)
r.POST("/users", handler)
r.PUT("/users/:id", handler)
r.DELETE("/users/:id", handler)
```

### Route Groups

```go
api := r.Group("/api/v1")
api.Use(authMiddleware)
api.GET("/users", listUsers)
```

### Middleware

```go
// Global
r.Use(logger, recovery)

// Group-specific
api.Use(authMiddleware)

// Per-route (pass middleware as arguments)
r.GET("/admin", adminMiddleware, handler)
```

### Route Constraints

Typed constraints with OpenAPI semantics:

```go
r.GET("/users/:id", handler).WhereInt("id")           // OpenAPI: type integer
r.GET("/entities/:uuid", handler).WhereUUID("uuid")   // OpenAPI: format uuid
r.GET("/files/:name", handler).WhereRegex("name", `[a-zA-Z0-9._-]+`)
```

### API Versioning

```go
r := router.New(
    router.WithVersioning(
        router.WithHeaderVersioning("API-Version"),
        router.WithQueryVersioning("version"),
        router.WithDefaultVersion("v1"),
    ),
)

// Version-specific routes
v1 := r.Version("v1")
v1.GET("/users", getUsersV1)

v2 := r.Version("v2")
v2.GET("/users", getUsersV2)
```

### Wildcard Routes

```go
// Wildcard routes
r.GET("/files/*", fileHandler)
r.GET("/static/*", staticHandler)

// Access in handler
func fileHandler(c *router.Context) {
    filepath := c.Param("filepath")
    // Handle file request
}
```

### Request Binding

```go
// Bind request body
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

var req CreateUserRequest
if err := c.Bind(&req); err != nil {
    c.JSON(http.StatusBadRequest, errorResponse)
    return
}

// Bind query parameters
type ListParams struct {
    Page     int `query:"page"`
    PageSize int `query:"page_size"`
}

var params ListParams
c.BindQuery(&params)

// Bind path parameters
type PathParams struct {
    ID int `params:"id"`
}

var params PathParams
c.BindParams(&params)
```

### Structured Error Responses

```go
type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
    Path    string `json:"path,omitempty"`
}

c.JSON(http.StatusNotFound, APIError{
    Code:    "USER_NOT_FOUND",
    Message: "User not found",
    Path:    c.Request.URL.Path,
})
```

## 🧪 Testing Examples

Each example includes curl commands in its output. Generally:

```bash
# Run the example
go run main.go

# In another terminal, test it
curl http://localhost:8080/
```

## 🎯 What's Here vs. What's in `app/`

### Use `router/examples` to learn

- ✅ How routing works
- ✅ How to use middleware
- ✅ Router features and APIs
- ✅ Request/response handling
- ✅ Pure router capabilities

### Use `app/examples` for

- ❌ Complete applications
- ❌ Database integration
- ❌ Authentication patterns
- ❌ Business logic
- ❌ Production deployments

## 📝 Building Your Own

1. **Copy a similar example** as a starting point
2. **Modify routes** to match your API design
3. **Add middleware** as needed for auth, logging, etc.
4. **Add validation** with route constraints
5. **Use versioning** for API evolution
6. **Implement wildcards** for flexible routing
7. **Handle errors** with structured responses
8. **Build custom middleware** for your needs

## 🤝 Need Help?

- Check the [main README](../../README.md)
- Review similar examples
- Look at the router [documentation](../README.md)

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.

```text
Copyright 2025 The Rivaas Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
