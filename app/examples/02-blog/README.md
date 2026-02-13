# Blog API Example

A full-featured blog API demonstrating the capabilities of the Rivaas web framework, including configuration management, validation, OpenAPI documentation, observability, and comprehensive testing.

## Features

This example showcases:

- **Configuration Management** - Using `rivaas.dev/config` to load settings from YAML files and environment variables
- **Method-based Validation** - Proper validation using `IsValid()` methods instead of struct tags
- **OpenAPI Documentation** - Auto-generated Swagger UI at `/docs`
- **Observability** - Structured logging, Prometheus metrics, and OpenTelemetry tracing
- **Health Endpoints** - Liveness (`/livez`) and readiness (`/readyz`) checks
- **API Versioning** - Path-based versioning (`/v1/stats`, `/v1/popular`)
- **Integration Tests** - Using `app/testing.go` for comprehensive test coverage
- **Real-world Patterns** - Slug-based URLs, status transitions, nested resources, pagination

## Quick Start

### 1. Run the Application

```bash
cd app/examples/02-blog
go run main.go
```

The server starts on `http://localhost:8080`

### 2. Explore the API

- **OpenAPI Docs**: http://localhost:8080/docs
- **Health Check**: http://localhost:8080/livez
- **Metrics**: http://localhost:9090/metrics

### 3. Try Some Requests

```bash
# List all posts
curl http://localhost:8080/posts

# Get a specific post by slug
curl http://localhost:8080/posts/getting-started-with-go

# Create a new post
curl -X POST http://localhost:8080/posts \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My First Post",
    "slug": "my-first-post",
    "content": "# Hello World\n\nThis is my first blog post!",
    "authorId": 1,
    "status": "draft",
    "tags": ["go", "tutorial"]
  }'

# Publish a draft post
curl -X PATCH http://localhost:8080/posts/3/publish

# List comments on a post
curl http://localhost:8080/posts/getting-started-with-go/comments

# Add a comment
curl -X POST http://localhost:8080/posts/getting-started-with-go/comments \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Great post!",
    "authorName": "John Doe",
    "authorEmail": "john@example.com"
  }'

# Get blog statistics (versioned endpoint)
curl http://localhost:8080/v1/stats

# Get popular posts
curl http://localhost:8080/v1/popular?limit=5
```

## API Endpoints

### Posts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/posts` | List posts (with pagination and filters) |
| GET | `/posts/:slug` | Get post by slug |
| POST | `/posts` | Create new post |
| PUT | `/posts/:id` | Update post |
| PATCH | `/posts/:id/publish` | Publish a draft post |

### Authors

| Method | Path | Description |
|--------|------|-------------|
| GET | `/authors` | List all authors |
| GET | `/authors/:id` | Get author profile |
| GET | `/authors/:id/posts` | Get posts by author |

### Comments

| Method | Path | Description |
|--------|------|-------------|
| GET | `/posts/:slug/comments` | List comments on a post |
| POST | `/posts/:slug/comments` | Add comment to a post |

### Statistics (Versioned API)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/stats` | Blog statistics |
| GET | `/v1/popular` | Most viewed posts |

## Configuration

The application uses `rivaas.dev/config` to load settings from `config.yaml` and environment variables.

### config.yaml

```yaml
server:
  host: "localhost"
  port: 8080
  readTimeout: "15s"
  writeTimeout: "15s"
  shutdownTimeout: "30s"

blog:
  postsPerPage: 10
  maxTitleLength: 200
  maxContentLength: 50000
  allowedStatuses:
    - draft
    - published
    - archived
  enableComments: true
  requireModeration: false

observability:
  environment: "development"
  sampleRate: 1.0
  metricsPort: ":9090"
```

### Environment Variables

Override configuration using environment variables with the `BLOG_` prefix:

```bash
# Server configuration
export BLOG_SERVER_PORT=3000
export BLOG_SERVER_HOST=0.0.0.0

# Blog settings
export BLOG_BLOG_POSTSPERPAGE=20
export BLOG_BLOG_ENABLECOMMENTS=false

# Observability
export BLOG_OBSERVABILITY_ENVIRONMENT=production
export BLOG_OBSERVABILITY_SAMPLERATE=0.1

# You can also override the port directly
export PORT=3000
```

## Validation

This example demonstrates proper validation using method-based approaches instead of struct tag enums.

### PostStatus Validation

```go
type PostStatus string

const (
    StatusDraft     PostStatus = "draft"
    StatusPublished PostStatus = "published"
    StatusArchived  PostStatus = "archived"
)

func (s PostStatus) IsValid() bool {
    return slices.Contains([]PostStatus{StatusDraft, StatusPublished, StatusArchived}, s)
}
```

### Request Validation

```go
func (r *CreatePostRequest) Validate() error {
    if !r.Status.IsValid() {
        return WrapError(ErrValidationFailed, "status must be one of: draft, published, archived")
    }
    // ... more validation
    return nil
}
```

## Testing

Run the integration tests using Go's testing framework:

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestCreatePost

# Run with race detector
go test -race

# View coverage
go test -cover
```

### Test Examples

The test suite demonstrates using `app/testing.go` for integration testing:

```go
func TestCreatePost(t *testing.T) {
    a := setupTestApp(t)
    
    body := handlers.CreatePostRequest{
        Title:    "My First Blog Post",
        Slug:     "my-first-blog-post",
        Content:  "# Hello World",
        AuthorID: 1,
        Status:   handlers.StatusDraft,
    }
    
    resp, err := a.TestJSON(http.MethodPost, "/posts", body)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()
    
    var result handlers.PostResponse
    app.ExpectJSON(t, resp, http.StatusCreated, &result)
    
    if result.Title != body.Title {
        t.Errorf("Expected title %q, got %q", body.Title, result.Title)
    }
}
```

## Project Structure

```
02-blog/
├── config.yaml              # Application configuration
├── main.go                  # Application entry point
├── main_test.go             # Integration tests
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies
├── README.md                # This file
└── handlers/
    ├── types.go             # Domain types and validation
    ├── errors.go            # Error handling
    ├── posts.go             # Post CRUD handlers
    ├── authors.go           # Author handlers
    ├── comments.go          # Comment handlers
    └── stats.go             # Statistics handlers
```

## Key Patterns

### 1. Configuration Loading

```go
var blogConfig BlogConfig
cfg := config.MustNew(
    config.WithFile("config.yaml"),
    config.WithEnv("BLOG_"),
    config.WithBinding(&blogConfig),
)
if err := cfg.Load(context.Background()); err != nil {
    log.Fatal(err)
}
```

### 2. Route Registration with OpenAPI

```go
a.POST("/posts", handlers.CreatePost,
    app.WithDoc(
        openapi.WithSummary("Create post"),
        openapi.WithDescription("Creates a new blog post"),
        openapi.WithRequest(handlers.CreatePostRequest{}),
        openapi.WithResponse(http.StatusCreated, handlers.PostResponse{}),
        openapi.WithTags("posts"),
    ),
)
```

### 3. Slug-based URLs with Regex Constraints

```go
a.GET("/posts/:slug", handlers.GetPostBySlug,
    app.WithDoc(/* ... */),
).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)
```

### 4. Status Transitions

```go
func PublishPost(c *app.Context) {
    post.Status = StatusPublished
    now := time.Now()
    post.PublishedAt = &now
    post.UpdatedAt = now
    // ...
}
```

### 5. API Versioning

```go
v1 := a.Version("v1")
v1.GET("/stats", handlers.GetBlogStats)
v1.GET("/popular", handlers.GetPopularPosts)
```

## Observability

### Logging

Structured logs are automatically generated for each request:

```json
{
  "level": "info",
  "msg": "request completed",
  "method": "GET",
  "path": "/posts/getting-started-with-go",
  "status": 200,
  "duration": "2.5ms",
  "trace_id": "abc123..."
}
```

### Metrics

Prometheus metrics are exposed at `:9090/metrics`:

- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration histogram
- `posts_created_total` - Custom counter for post creation
- `post_views_total` - Custom counter for post views

### Tracing

OpenTelemetry traces capture request flow with custom attributes:

```go
c.SetSpanAttribute("post.slug", slug)
c.AddSpanEvent("fetching_post_by_slug")
```

## Next Steps

- Add database persistence (PostgreSQL, MongoDB)
- Implement authentication and authorization
- Add rate limiting
- Implement full-text search
- Add caching layer (Redis)
- Deploy to production (Docker, Kubernetes)

## Learn More

- [Rivaas Documentation](../../README.md)
- [Configuration Package](../../../config/README.md)
- [Validation Package](../../../validation/README.md)
- [OpenAPI Package](../../../openapi/README.md)

## License

Apache License 2.0 - see [LICENSE](../../../LICENSE) for details.

