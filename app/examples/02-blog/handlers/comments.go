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

// Package handlers provides comment-related HTTP handlers for the blog API.
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/app"
)

// In-memory storage for comments (for demo purposes)
var (
	comments      = make(map[int]*CommentResponse)
	nextCommentID = 1
)

func init() {
	// Initialize with sample comments
	sampleComments := []CommentResponse{
		{
			ID:          1,
			PostID:      1,
			Content:     "Great introduction to Go! Very helpful for beginners.",
			AuthorName:  "Alice Johnson",
			AuthorEmail: "alice@example.com",
			CreatedAt:   time.Now().Add(-12 * time.Hour),
		},
		{
			ID:          2,
			PostID:      1,
			Content:     "Thanks for sharing this. I've been looking for a good Go tutorial.",
			AuthorName:  "Bob Wilson",
			AuthorEmail: "bob@example.com",
			CreatedAt:   time.Now().Add(-6 * time.Hour),
		},
		{
			ID:          3,
			PostID:      2,
			Content:     "This is exactly what I needed for my project. Thank you!",
			AuthorName:  "Carol Davis",
			AuthorEmail: "carol@example.com",
			CreatedAt:   time.Now().Add(-3 * time.Hour),
		},
	}

	for _, comment := range sampleComments {
		c := comment // Create copy to avoid pointer issues
		comments[c.ID] = &c
		if c.ID >= nextCommentID {
			nextCommentID = c.ID + 1
		}
	}
}

// ListComments retrieves all comments for a specific blog post.
//
// Path parameters:
//   - slug: Post slug
//
// Example:
//
//	GET /posts/getting-started-with-go/comments
func ListComments(c *app.Context) {
	slug := c.Param("slug")
	if slug == "" {
		HandleError(c, WrapError(ErrInvalidInput, "slug is required"))
		return
	}

	c.SetSpanAttribute("post.slug", slug)
	c.AddSpanEvent("listing_comments")

	// Find post
	post, exists := postsBySlug[slug]
	if !exists {
		HandleError(c, ErrPostNotFound)
		return
	}

	// Filter comments for this post
	var postComments []*CommentResponse
	for _, comment := range comments {
		if comment.PostID == post.ID {
			postComments = append(postComments, comment)
		}
	}

	// Record metrics
	c.IncrementCounter("comments_list_total",
		attribute.String("post_slug", slug),
		attribute.Int("count", len(postComments)),
	)

	if err := c.JSON(http.StatusOK, map[string]any{
		"postId":   post.ID,
		"comments": postComments,
		"total":    len(postComments),
	}); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// CreateComment adds a new comment to a blog post.
//
// Path parameters:
//   - slug: Post slug
//
// Request body:
//   - content: Comment content (required, max 5000 chars)
//   - authorName: Commenter name (optional, for guest comments)
//   - authorEmail: Commenter email (optional, for guest comments)
//
// Example:
//
//	POST /posts/getting-started-with-go/comments
//	Body: {"content": "Great post!", "authorName": "John Doe", "authorEmail": "john@example.com"}
func CreateComment(c *app.Context) {
	slug := c.Param("slug")
	if slug == "" {
		HandleError(c, WrapError(ErrInvalidInput, "slug is required"))
		return
	}

	var req CreateCommentRequest
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
	post, exists := postsBySlug[slug]
	if !exists {
		HandleError(c, ErrPostNotFound)
		return
	}

	c.SetSpanAttribute("post.slug", slug)
	c.SetSpanAttribute("post.id", post.ID)
	c.AddSpanEvent("creating_comment")

	// Set default author name if not provided
	authorName := req.AuthorName
	if authorName == "" {
		authorName = "Anonymous"
	}

	// Create comment
	comment := &CommentResponse{
		ID:          nextCommentID,
		PostID:      post.ID,
		Content:     req.Content,
		AuthorName:  authorName,
		AuthorEmail: req.AuthorEmail,
		CreatedAt:   time.Now(),
	}

	comments[nextCommentID] = comment
	nextCommentID++

	// Record metrics
	c.IncrementCounter("comments_created_total",
		attribute.String("post_slug", slug),
		attribute.Int("post_id", post.ID),
	)

	c.AddSpanEvent("comment_created")
	if err := c.JSON(http.StatusCreated, comment); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}
