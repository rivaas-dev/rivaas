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

// Package handlers provides post-related HTTP handlers for the blog API.
package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/app"
)

// In-memory storage for posts (for demo purposes)
var (
	posts       = make(map[int]*PostResponse)
	postsBySlug = make(map[string]*PostResponse)
	nextPostID  = 1
)

func init() {
	// Initialize with sample posts
	samplePosts := []struct {
		title   string
		slug    string
		content string
		excerpt string
		status  PostStatus
	}{
		{
			"Getting Started with Go",
			"getting-started-with-go",
			"# Introduction\n\nGo is a statically typed, compiled programming language...",
			"Learn the basics of Go programming",
			StatusPublished,
		},
		{
			"Building REST APIs",
			"building-rest-apis",
			"# Building REST APIs\n\nREST APIs are a fundamental part of modern web development...",
			"A comprehensive guide to building REST APIs",
			StatusPublished,
		},
		{
			"Draft Post Example",
			"draft-post-example",
			"# This is a draft\n\nThis post is not yet published...",
			"An example of a draft post",
			StatusDraft,
		},
	}

	for _, sample := range samplePosts {
		now := time.Now()
		var publishedAt *time.Time
		if sample.status == StatusPublished {
			publishedAt = &now
		}

		post := &PostResponse{
			ID:      nextPostID,
			Slug:    sample.slug,
			Title:   sample.title,
			Content: sample.content,
			Excerpt: sample.excerpt,
			Author: Author{
				ID:        1,
				Name:      "Jane Doe",
				Email:     "jane@example.com",
				Bio:       "Software engineer and Go enthusiast",
				AvatarURL: "https://example.com/avatars/jane.jpg",
			},
			Tags:        []string{"go", "programming"},
			Status:      sample.status,
			ViewCount:   0,
			PublishedAt: publishedAt,
			CreatedAt:   now.Add(-24 * time.Hour),
			UpdatedAt:   now,
		}
		posts[nextPostID] = post
		postsBySlug[sample.slug] = post
		nextPostID++
	}
}

// ListPosts retrieves a paginated list of blog posts with optional filtering.
//
// Query parameters:
//   - page: Page number (default: 1)
//   - perPage: Items per page (default: 10, max: 100)
//   - status: Filter by status (draft, published, archived)
//   - tag: Filter by tag
//   - authorId: Filter by author ID
//   - sortBy: Sort order (date, views, title)
//
// Example:
//
//	GET /posts?page=1&perPage=10&status=published&sortBy=date
func ListPosts(c *app.Context) {
	var params ListPostsParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Validate params
	if err := params.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Add tracing
	c.SetSpanAttribute("query.page", params.Page)
	c.SetSpanAttribute("query.perPage", params.PerPage)
	if params.Status != "" {
		c.SetSpanAttribute("query.status", string(params.Status))
	}
	c.AddSpanEvent("listing_posts")

	// Filter posts
	var filteredPosts []*PostResponse
	for _, post := range posts {
		// Apply filters
		if params.Status != "" && post.Status != params.Status {
			continue
		}
		if params.Tag != "" {
			hasTag := false
			for _, tag := range post.Tags {
				if tag == params.Tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		if params.AuthorID != nil && post.Author.ID != *params.AuthorID {
			continue
		}
		filteredPosts = append(filteredPosts, post)
	}

	// Calculate pagination
	total := len(filteredPosts)
	start := (params.Page - 1) * params.PerPage
	end := start + params.PerPage

	if start >= total {
		start = total
		end = total
	}
	if end > total {
		end = total
	}

	paginatedPosts := filteredPosts[start:end]

	// Record metrics
	c.IncrementCounter("posts_list_total",
		attribute.Int("page", params.Page),
		attribute.Int("results", len(paginatedPosts)),
	)

	if err := c.JSON(http.StatusOK, map[string]any{
		"posts":   paginatedPosts,
		"total":   total,
		"page":    params.Page,
		"perPage": params.PerPage,
	}); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// GetPostBySlug retrieves a single blog post by its slug.
//
// Path parameters:
//   - slug: Post slug (lowercase-hyphen format)
//
// Example:
//
//	GET /posts/getting-started-with-go
func GetPostBySlug(c *app.Context) {
	slug := c.Param("slug")
	if slug == "" {
		HandleError(c, WrapError(ErrInvalidInput, "slug is required"))
		return
	}

	c.SetSpanAttribute("post.slug", slug)
	c.AddSpanEvent("fetching_post_by_slug")

	post, exists := postsBySlug[slug]
	if !exists {
		c.SetSpanAttribute("error", true)
		HandleError(c, ErrPostNotFound)
		return
	}

	// Increment view count
	post.ViewCount++

	// Record metrics
	c.IncrementCounter("post_views_total",
		attribute.String("slug", slug),
		attribute.Int("post_id", post.ID),
	)

	if err := c.JSON(http.StatusOK, post); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// CreatePost creates a new blog post.
//
// Request body:
//   - title: Post title (required, max 200 chars)
//   - slug: URL-friendly slug (required, lowercase-hyphen)
//   - content: Post content in Markdown (required, max 50000 chars)
//   - excerpt: Short summary (optional)
//   - authorId: Author ID (required)
//   - tags: Array of tags (optional)
//   - status: Post status (draft, published, archived)
//
// Example:
//
//	POST /posts
//	Body: {"title": "My Post", "slug": "my-post", "content": "...", "authorId": 1}
func CreatePost(c *app.Context) {
	var req CreatePostRequest
	if err := c.Bind(&req); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "failed to parse request body: %v", err))
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Check if slug already exists
	if _, exists := postsBySlug[req.Slug]; exists {
		HandleError(c, ErrSlugTaken)
		return
	}

	// Add tracing
	c.SetSpanAttribute("operation", "create_post")
	c.SetSpanAttribute("post.slug", req.Slug)
	c.SetSpanAttribute("post.status", string(req.Status))
	c.AddSpanEvent("post_creation_started")

	// Create post
	now := time.Now()
	var publishedAt *time.Time
	if req.Status == StatusPublished {
		publishedAt = &now
	}

	post := &PostResponse{
		ID:      nextPostID,
		Slug:    req.Slug,
		Title:   req.Title,
		Content: req.Content,
		Excerpt: req.Excerpt,
		Author: Author{
			ID:    req.AuthorID,
			Name:  "Author " + fmt.Sprintf("%d", req.AuthorID),
			Email: fmt.Sprintf("author%d@example.com", req.AuthorID),
		},
		Tags:        req.Tags,
		Status:      req.Status,
		ViewCount:   0,
		PublishedAt: publishedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	posts[nextPostID] = post
	postsBySlug[req.Slug] = post
	nextPostID++

	// Record metrics
	c.IncrementCounter("posts_created_total",
		attribute.String("status", string(req.Status)),
	)

	c.AddSpanEvent("post_created")
	if err := c.JSON(http.StatusCreated, post); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// UpdatePost updates an existing blog post.
//
// Path parameters:
//   - id: Post ID
//
// Request body: (all fields optional)
//   - title: Updated title
//   - content: Updated content
//   - excerpt: Updated excerpt
//   - tags: Updated tags
//   - status: Updated status
//
// Example:
//
//	PUT /posts/1
//	Body: {"title": "Updated Title", "status": "published"}
func UpdatePost(c *app.Context) {
	var pathParams struct {
		ID int `path:"id"`
	}
	if err := c.Bind(&pathParams); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	var req UpdatePostRequest
	if err := c.Bind(&req); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "failed to parse request body: %v", err))
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Find post
	post, exists := posts[pathParams.ID]
	if !exists {
		HandleError(c, ErrPostNotFound)
		return
	}

	c.SetSpanAttribute("post.id", pathParams.ID)
	c.AddSpanEvent("updating_post")

	// Update fields
	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}
	if req.Excerpt != nil {
		post.Excerpt = *req.Excerpt
	}
	if req.Tags != nil {
		post.Tags = req.Tags
	}
	if req.Status != "" && req.Status != post.Status {
		post.Status = req.Status
		if req.Status == StatusPublished && post.PublishedAt == nil {
			now := time.Now()
			post.PublishedAt = &now
		}
	}
	post.UpdatedAt = time.Now()

	// Record metrics
	c.IncrementCounter("posts_updated_total")

	if err := c.JSON(http.StatusOK, post); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// PublishPost publishes a draft post (status transition from draft to published).
//
// Path parameters:
//   - id: Post ID
//
// Example:
//
//	PATCH /posts/1/publish
func PublishPost(c *app.Context) {
	var pathParams struct {
		ID int `path:"id"`
	}
	if err := c.Bind(&pathParams); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Find post
	post, exists := posts[pathParams.ID]
	if !exists {
		HandleError(c, ErrPostNotFound)
		return
	}

	// Check if already published
	if post.Status == StatusPublished {
		HandleError(c, WrapError(ErrCannotPublish, "post is already published"))
		return
	}

	c.SetSpanAttribute("post.id", pathParams.ID)
	c.SetSpanAttribute("operation", "publish_post")
	c.AddSpanEvent("publishing_post")

	// Publish
	post.Status = StatusPublished
	now := time.Now()
	post.PublishedAt = &now
	post.UpdatedAt = now

	// Record metrics
	c.IncrementCounter("posts_published_total")

	if err := c.JSON(http.StatusOK, post); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}
