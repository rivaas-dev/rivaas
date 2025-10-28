# Rivaas Router Examples

This directory contains practical examples demonstrating the features and capabilities of the Rivaas router.

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

### 3. [Middleware](./03-middleware/)

Common middleware patterns including auth, logging, recovery, and CORS.

```bash
cd 03-middleware && go run main.go
curl -H "Authorization: Bearer token123" http://localhost:8080/api/profile
```

**Learn:** Global middleware, group middleware, middleware chaining, authentication, CORS

---

### 4. [REST API](./04-rest-api/)

Complete CRUD REST API with in-memory storage.

```bash
cd 04-rest-api && go run main.go
curl http://localhost:8080/api/v1/users
curl -X POST http://localhost:8080/api/v1/users -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

**Learn:** Full CRUD operations, request/response handling, error handling, JSON parsing

---

### 5. [Advanced](./05-advanced/)

Advanced features: route constraints, helpers, static files, introspection.

```bash
cd 05-advanced && go run main.go
curl http://localhost:8080/users/123           # ✓ Valid (numeric)
curl http://localhost:8080/users/abc           # ✗ Invalid (not numeric)
```

**Learn:** Route validation, constraints (UUID, numeric, alpha), static files, cookie handling, introspection

---

### 6. [Advanced Routing](./06-advanced-routing/)

Advanced routing features: versioning and wildcards.

```bash
cd 06-advanced-routing && go run main.go
curl -H "API-Version: v1" http://localhost:8080/users
curl http://localhost:8080/files/static/image.jpg
```

**Learn:** API versioning (header/query), version-specific groups, wildcard routes, efficient routing

---

### 7. [Content Negotiation](./07-content-negotiation/)

HTTP content negotiation for flexible API responses.

```bash
cd 07-content-negotiation && go run main.go
curl -H "Accept: application/json" http://localhost:8080/api/user
curl -H "Accept-Language: fr" http://localhost:8080/api/greeting
```

**Learn:** Accept header parsing, format negotiation (JSON/XML/HTML), language/encoding/charset negotiation

---

### 8. [Request Binding](./08-binding/)

Automatic request data binding to structs.

```bash
cd 08-binding && go run main.go
curl -X POST http://localhost:8080/api/users -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","age":25}'
curl "http://localhost:8080/api/search?q=golang&page=2&tags=web&tags=api"
```

**Learn:** BindBody, BindQuery, BindParams, BindCookies, BindHeaders, type conversion, slices, pointers

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

1. Start with **01-hello-world** to understand the basics
2. Move to **02-routing** to learn about routes and parameters
3. Explore **03-middleware** for request processing
4. Build a **04-rest-api** to see everything together
5. Learn **05-advanced** features for constraints and helpers
6. Master **06-advanced-routing** for versioning and wildcards
7. Implement **07-content-negotiation** for flexible API responses
8. Use **08-binding** for automatic request data parsing

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

// Per-route
r.GET("/admin", handler).Use(adminMiddleware)
```

### Route Constraints

```go
r.GET("/users/:id", handler).WhereNumber("id")
r.GET("/entities/:uuid", handler).WhereUUID("uuid")
r.GET("/files/:name", handler).Where("name", `[a-zA-Z0-9._-]+`)
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

## 🧪 Testing Examples

Each example includes curl commands in its output. Generally:

```bash
# Run the example
go run main.go

# In another terminal, test it
curl http://localhost:8080/
```

## 📝 Building Your Own

1. **Copy a similar example** as a starting point
2. **Modify routes** to match your API design
3. **Add middleware** as needed for auth, logging, etc.
4. **Add validation** with route constraints
5. **Use versioning** for API evolution
6. **Implement wildcards** for flexible routing

## 🤝 Need Help?

- Check the [main README](../../README.md)
- Review similar examples
- Look at the router [documentation](../README.md)

## 📄 License

All examples are provided under the same license as the Rivaas project.
