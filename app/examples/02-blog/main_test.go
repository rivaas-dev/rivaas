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

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/blog/handlers"

	"rivaas.dev/app"
	"rivaas.dev/config"
	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
	"rivaas.dev/middleware/cors"
	"rivaas.dev/middleware/requestid"
	"rivaas.dev/middleware/timeout"
	"rivaas.dev/router/version"
	"rivaas.dev/tracing"
)

func setupTestApp(t *testing.T) *app.App {
	t.Helper()

	// Load test configuration
	var blogConfig BlogConfig
	cfg := config.MustNew(
		config.WithFile("config.yaml"),
		config.WithBinding(&blogConfig),
	)

	if err := cfg.Load(context.Background()); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create test app
	a, err := app.New(
		app.WithServiceName("blog-api-test"),
		app.WithServiceVersion("v1.0.0"),
		app.WithEnvironment("development"),
		app.WithRouter(
			router.WithVersioning(
				version.WithPathDetection("/v{version}/"),
				version.WithDefault("v1"),
				version.WithValidVersions("v1"),
			),
		),
		app.WithObservability(
			app.WithLogging(logging.WithConsoleHandler()),
			app.WithMetrics(metrics.WithPrometheus(":9091", "/metrics")),
			app.WithTracing(tracing.WithStdout(), tracing.WithSampleRate(1.0)),
			app.WithExcludePaths("/livez", "/readyz", "/metrics"),
		),
		app.WithHealthEndpoints(
			app.WithHealthTimeout(800*time.Millisecond),
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				return nil
			}),
		),
		app.WithOpenAPI(
			openapi.WithTitle("blog-api-test", "v1.0.0"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Register middleware
	a.Router().Use(requestid.New())
	a.Router().Use(cors.New(cors.WithAllowAllOrigins(true)))
	a.Router().Use(timeout.New(timeout.WithDuration(30 * time.Second)))

	// Register routes (same as main.go)
	registerRoutes(a, &blogConfig)
	a.Router().Warmup()

	return a
}

func registerRoutes(a *app.App, blogConfig *BlogConfig) {
	// Root endpoint
	a.GET("/", func(c *app.Context) {
		_ = c.JSON(http.StatusOK, map[string]any{"message": "Blog API"})
	})

	// Post endpoints
	a.GET("/posts", handlers.ListPosts)
	a.GET("/posts/:slug", handlers.GetPostBySlug).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)
	a.POST("/posts", handlers.CreatePost)
	a.PUT("/posts/:id", handlers.UpdatePost).WhereInt("id")
	a.PATCH("/posts/:id/publish", handlers.PublishPost).WhereInt("id")

	// Author endpoints
	a.GET("/authors", handlers.ListAuthors)
	a.GET("/authors/:id", handlers.GetAuthor).WhereInt("id")
	a.GET("/authors/:id/posts", handlers.GetAuthorPosts).WhereInt("id")

	// Comment endpoints
	if blogConfig.Blog.EnableComments {
		a.GET("/posts/:slug/comments", handlers.ListComments).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)
		a.POST("/posts/:slug/comments", handlers.CreateComment).WhereRegex("slug", `[a-z0-9]+(?:-[a-z0-9]+)*`)
	}

	// Versioned API (v1)
	v1 := a.Version("v1")
	v1.GET("/stats", handlers.GetBlogStats)
	v1.GET("/popular", handlers.GetPopularPosts)
}

func TestListPosts(t *testing.T) {
	a := setupTestApp(t)

	t.Run("default pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts", nil)
		resp, err := a.Test(req, app.WithTimeout(5*time.Second))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Posts   []handlers.PostResponse `json:"posts"`
			Total   int                     `json:"total"`
			Page    int                     `json:"page"`
			PerPage int                     `json:"perPage"`
		}
		app.ExpectJSON(t, resp, http.StatusOK, &result)

		if result.Page != 1 {
			t.Errorf("Expected page 1, got %d", result.Page)
		}
		if result.PerPage != 10 {
			t.Errorf("Expected perPage 10, got %d", result.PerPage)
		}
	})

	t.Run("with filters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts?status=published&page=1&perPage=5", nil)
		resp, err := a.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestGetPostBySlug(t *testing.T) {
	a := setupTestApp(t)

	tests := []struct {
		name       string
		slug       string
		wantStatus int
	}{
		{"valid slug", "getting-started-with-go", http.StatusOK},
		{"another valid slug", "building-rest-apis", http.StatusOK},
		{"not found", "non-existent-slug", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/posts/"+tt.slug, nil)
			resp, err := a.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.wantStatus == http.StatusOK {
				var post handlers.PostResponse
				app.ExpectJSON(t, resp, http.StatusOK, &post)
				if post.Slug != tt.slug {
					t.Errorf("Expected slug %q, got %q", tt.slug, post.Slug)
				}
			}
		})
	}
}

func TestCreatePost(t *testing.T) {
	a := setupTestApp(t)

	t.Run("valid post", func(t *testing.T) {
		body := handlers.CreatePostRequest{
			Title:    "My First Blog Post",
			Slug:     "my-first-blog-post",
			Content:  "# Hello World\n\nThis is my first post!",
			Excerpt:  "Introduction to my blog",
			AuthorID: 1,
			Status:   handlers.StatusDraft,
			Tags:     []string{"go", "programming"},
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
		if result.Status != handlers.StatusDraft {
			t.Errorf("Expected status draft, got %s", result.Status)
		}
		if result.Slug != body.Slug {
			t.Errorf("Expected slug %q, got %q", body.Slug, result.Slug)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		body := map[string]any{
			"title":    "Test Post",
			"slug":     "test-post-invalid",
			"content":  "Content here",
			"authorId": 1,
			"status":   "invalid-status",
		}

		resp, err := a.TestJSON(http.MethodPost, "/posts", body)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid slug format", func(t *testing.T) {
		body := handlers.CreatePostRequest{
			Title:    "Test Post",
			Slug:     "INVALID_SLUG!",
			Content:  "Content",
			AuthorID: 1,
		}

		resp, err := a.TestJSON(http.MethodPost, "/posts", body)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestPublishPost(t *testing.T) {
	a := setupTestApp(t)

	req := httptest.NewRequest(http.MethodPatch, "/posts/3/publish", nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var result handlers.PostResponse
	app.ExpectJSON(t, resp, http.StatusOK, &result)

	if result.Status != handlers.StatusPublished {
		t.Errorf("Expected status published, got %s", result.Status)
	}
	if result.PublishedAt == nil {
		t.Error("Expected publishedAt to be set")
	}
}

func TestListAuthors(t *testing.T) {
	a := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/authors", nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Authors []*handlers.Author `json:"authors"`
		Total   int                `json:"total"`
	}
	app.ExpectJSON(t, resp, http.StatusOK, &result)

	if result.Total == 0 {
		t.Error("Expected at least one author")
	}
}

func TestGetAuthor(t *testing.T) {
	a := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/authors/1", nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var author handlers.Author
	app.ExpectJSON(t, resp, http.StatusOK, &author)

	if author.ID != 1 {
		t.Errorf("Expected author ID 1, got %d", author.ID)
	}
}

func TestPostComments(t *testing.T) {
	a := setupTestApp(t)

	t.Run("list comments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/getting-started-with-go/comments", nil)
		resp, err := a.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("add comment", func(t *testing.T) {
		body := handlers.CreateCommentRequest{
			Content:     "Great post! Very informative.",
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
		}

		resp, err := a.TestJSON(http.MethodPost, "/posts/getting-started-with-go/comments", body)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var result handlers.CommentResponse
		app.ExpectJSON(t, resp, http.StatusCreated, &result)

		if result.Content != body.Content {
			t.Errorf("Expected content %q, got %q", body.Content, result.Content)
		}
		if result.AuthorName != body.AuthorName {
			t.Errorf("Expected author name %q, got %q", body.AuthorName, result.AuthorName)
		}
	})
}

func TestBlogStats(t *testing.T) {
	a := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var stats handlers.BlogStatsResponse
	app.ExpectJSON(t, resp, http.StatusOK, &stats)

	if stats.TotalPosts == 0 {
		t.Error("Expected at least one post in stats")
	}
}

func TestPopularPosts(t *testing.T) {
	a := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/popular?limit=5", nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Posts []handlers.PopularPostResponse `json:"posts"`
		Total int                            `json:"total"`
		Limit int                            `json:"limit"`
	}
	app.ExpectJSON(t, resp, http.StatusOK, &result)

	if result.Limit != 5 {
		t.Errorf("Expected limit 5, got %d", result.Limit)
	}
}

func TestWithContext(t *testing.T) {
	a := setupTestApp(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	resp, err := a.Test(req, app.WithContext(ctx), app.WithTimeout(-1))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
