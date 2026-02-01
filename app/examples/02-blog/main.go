// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main demonstrates a full-featured blog API using the Rivaas framework.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"example.com/blog/handlers"

	"rivaas.dev/app"
	"rivaas.dev/config"
	"rivaas.dev/logging"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/requestid"
	"rivaas.dev/router/middleware/timeout"
	"rivaas.dev/router/version"
)

// BlogConfig holds the complete blog application configuration
type BlogConfig struct {
	Environment   string                  `config:"environment"`
	Server        ServerConfig            `config:"server"`
	Blog          BlogSettings            `config:"blog"`
	Observability app.ObservabilityConfig `config:"observability"`
	Auth          AuthConfig              `config:"auth"`
}

type ServerConfig struct {
	Host            string        `config:"host"`
	Port            int           `config:"port"`
	ReadTimeout     time.Duration `config:"readTimeout"`
	WriteTimeout    time.Duration `config:"writeTimeout"`
	ShutdownTimeout time.Duration `config:"shutdownTimeout"`
}

type BlogSettings struct {
	PostsPerPage      int      `config:"postsPerPage"`
	MaxTitleLength    int      `config:"maxTitleLength"`
	MaxContentLength  int      `config:"maxContentLength"`
	AllowedStatuses   []string `config:"allowedStatuses"`
	EnableComments    bool     `config:"enableComments"`
	RequireModeration bool     `config:"requireModeration"`
}

type AuthConfig struct {
	JWTSecret     string        `config:"jwtSecret"`
	TokenDuration time.Duration `config:"tokenDuration"`
}

// Validate implements config struct validation
func (c *BlogConfig) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if c.Blog.PostsPerPage < 1 || c.Blog.PostsPerPage > 100 {
		return errors.New("blog.postsPerPage must be between 1 and 100")
	}
	if len(c.Blog.AllowedStatuses) == 0 {
		return errors.New("blog.allowedStatuses must not be empty")
	}
	// Validate allowed statuses contain only valid values
	validStatuses := []string{"draft", "published", "archived"}
	for _, s := range c.Blog.AllowedStatuses {
		if !slices.Contains(validStatuses, s) {
			return errors.New("blog.allowedStatuses contains invalid status: " + s)
		}
	}
	return nil
}

func main() {
	// Create context that listens for interrupt signal
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// Create a logger early for initialization-phase logging
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
	)

	// Load configuration
	var blogConfig BlogConfig
	cfg := config.MustNew(
		config.WithFile("config.yaml"),
		config.WithEnv("BLOG_"),
		config.WithBinding(&blogConfig),
	)

	if err := cfg.Load(ctx); err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create Rivaas app
	a, err := app.New(
		app.WithServiceName("blog-api"),
		app.WithServiceVersion("v1.0.0"),
		app.WithEnvironment(blogConfig.Environment),
		// Configure router with path-based versioning
		app.WithRouter(
			router.WithVersioning(
				version.WithPathDetection("/v{version}/"),
				version.WithDefault("v1"),
				version.WithValidVersions("v1"),
			),
		),
		// Configure observability from config (tracing, metrics, logging)
		app.WithObservabilityFromConfig(blogConfig.Observability),
		// Health endpoints
		app.WithHealthEndpoints(
			app.WithHealthTimeout(800*time.Millisecond),
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				return nil
			}),
		),
		// Server config
		app.WithHost(blogConfig.Server.Host),
		app.WithPort(blogConfig.Server.Port),
		app.WithServer(
			app.WithReadTimeout(blogConfig.Server.ReadTimeout),
			app.WithWriteTimeout(blogConfig.Server.WriteTimeout),
			app.WithShutdownTimeout(blogConfig.Server.ShutdownTimeout),
		),
		// OpenAPI documentation
		app.WithOpenAPI(
			openapi.WithTitle("blog-api", "v1.0.0"),
			openapi.WithInfoDescription("A full-featured blog API demonstrating Rivaas framework capabilities"),
			openapi.WithServer("http://localhost:8080", "Local development"),
			openapi.WithSwaggerUI(
				"/docs",
				openapi.WithUIExpansion(openapi.DocExpansionList),
				openapi.WithUITryItOut(true),
				openapi.WithUIRequestSnippets(true, openapi.SnippetCurlBash, openapi.SnippetCurlPowerShell),
			),
		),
	)
	if err != nil {
		logger.Error("failed to create app", "error", err)
		os.Exit(1)
	}

	// Global middleware
	a.Router().Use(requestid.New())
	a.Router().Use(cors.New(cors.WithAllowAllOrigins(true)))
	a.Router().Use(timeout.New(timeout.WithDuration(30 * time.Second)))

	// Root endpoint
	a.GET("/", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]any{
			"message":     "Blog API",
			"service":     "blog-api",
			"version":     "v1.0.0",
			"environment": blogConfig.Environment,
			"docs":        "/docs",
			"traceId":     c.TraceID(),
			"spanId":      c.SpanID(),
			"requestId":   c.Response.Header().Get("X-Request-ID"),
		}); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// Post endpoints
	a.GET("/posts", handlers.ListPosts,
		app.WithDoc(
			openapi.WithSummary("List posts"),
			openapi.WithDescription("Retrieves a paginated list of blog posts with optional filtering"),
			openapi.WithRequest(handlers.ListPostsParams{}),
			openapi.WithResponse(http.StatusOK, map[string]any{
				"posts":   []handlers.PostResponse{},
				"total":   0,
				"page":    1,
				"perPage": 10,
			}),
			openapi.WithTags("posts"),
		),
	)

	a.GET("/posts/:slug", handlers.GetPostBySlug,
		app.WithDoc(
			openapi.WithSummary("Get post by slug"),
			openapi.WithDescription("Retrieves a single blog post by its URL slug"),
			openapi.WithResponse(http.StatusOK, handlers.PostResponse{}),
			openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
			openapi.WithTags("posts"),
		),
	).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)

	a.POST("/posts", handlers.CreatePost,
		app.WithDoc(
			openapi.WithSummary("Create post"),
			openapi.WithDescription("Creates a new blog post"),
			openapi.WithRequest(handlers.CreatePostRequest{}),
			openapi.WithResponse(http.StatusCreated, handlers.PostResponse{}),
			openapi.WithResponse(http.StatusBadRequest, handlers.APIError{}),
			openapi.WithTags("posts"),
		),
	)

	a.PUT("/posts/:id", handlers.UpdatePost,
		app.WithDoc(
			openapi.WithSummary("Update post"),
			openapi.WithDescription("Updates an existing blog post"),
			openapi.WithRequest(handlers.UpdatePostRequest{}),
			openapi.WithResponse(http.StatusOK, handlers.PostResponse{}),
			openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
			openapi.WithTags("posts"),
		),
	).WhereInt("id")

	a.PATCH("/posts/:id/publish", handlers.PublishPost,
		app.WithDoc(
			openapi.WithSummary("Publish post"),
			openapi.WithDescription("Publishes a draft post"),
			openapi.WithResponse(http.StatusOK, handlers.PostResponse{}),
			openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
			openapi.WithResponse(http.StatusBadRequest, handlers.APIError{}),
			openapi.WithTags("posts"),
		),
	).WhereInt("id")

	// Author endpoints
	a.GET("/authors", handlers.ListAuthors,
		app.WithDoc(
			openapi.WithSummary("List authors"),
			openapi.WithDescription("Retrieves a list of all blog authors"),
			openapi.WithResponse(http.StatusOK, map[string]any{
				"authors": []handlers.Author{},
				"total":   0,
			}),
			openapi.WithTags("authors"),
		),
	)

	a.GET("/authors/:id", handlers.GetAuthor,
		app.WithDoc(
			openapi.WithSummary("Get author"),
			openapi.WithDescription("Retrieves an author profile by ID"),
			openapi.WithResponse(http.StatusOK, handlers.Author{}),
			openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
			openapi.WithTags("authors"),
		),
	).WhereInt("id")

	a.GET("/authors/:id/posts", handlers.GetAuthorPosts,
		app.WithDoc(
			openapi.WithSummary("Get author posts"),
			openapi.WithDescription("Retrieves all posts by a specific author"),
			openapi.WithResponse(http.StatusOK, map[string]any{
				"authorId": 1,
				"posts":    []handlers.PostResponse{},
				"total":    0,
			}),
			openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
			openapi.WithTags("authors", "posts"),
		),
	).WhereInt("id")

	// Comment endpoints
	if blogConfig.Blog.EnableComments {
		a.GET("/posts/:slug/comments", handlers.ListComments,
			app.WithDoc(
				openapi.WithSummary("List comments"),
				openapi.WithDescription("Retrieves all comments for a blog post"),
				openapi.WithResponse(http.StatusOK, map[string]any{
					"postId":   1,
					"comments": []handlers.CommentResponse{},
					"total":    0,
				}),
				openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
				openapi.WithTags("comments"),
			),
		).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)

		a.POST("/posts/:slug/comments", handlers.CreateComment,
			app.WithDoc(
				openapi.WithSummary("Create comment"),
				openapi.WithDescription("Adds a new comment to a blog post"),
				openapi.WithRequest(handlers.CreateCommentRequest{}),
				openapi.WithResponse(http.StatusCreated, handlers.CommentResponse{}),
				openapi.WithResponse(http.StatusNotFound, handlers.APIError{}),
				openapi.WithResponse(http.StatusBadRequest, handlers.APIError{}),
				openapi.WithTags("comments"),
			),
		).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)
	}

	// Versioned API (v1)
	v1 := a.Version("v1")

	v1.GET("/stats", handlers.GetBlogStats,
		app.WithDoc(
			openapi.WithSummary("Blog statistics"),
			openapi.WithDescription("Retrieves overall blog statistics"),
			openapi.WithResponse(http.StatusOK, handlers.BlogStatsResponse{}),
			openapi.WithTags("stats"),
		),
	)

	v1.GET("/popular", handlers.GetPopularPosts,
		app.WithDoc(
			openapi.WithSummary("Popular posts"),
			openapi.WithDescription("Retrieves the most viewed blog posts"),
			openapi.WithResponse(http.StatusOK, map[string]any{
				"posts": []handlers.PopularPostResponse{},
				"total": 0,
				"limit": 10,
			}),
			openapi.WithTags("stats"),
		),
	)

	if err := a.Start(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
