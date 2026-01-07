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

// Package handlers provides author-related HTTP handlers for the blog API.
package handlers

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/app"
)

// In-memory storage for authors (for demo purposes)
var (
	authors      = make(map[int]*Author)
	nextAuthorID = 1
)

func init() {
	// Initialize with sample authors
	sampleAuthors := []Author{
		{
			ID:        1,
			Name:      "Jane Doe",
			Email:     "jane@example.com",
			Bio:       "Software engineer and Go enthusiast with 10+ years of experience building scalable web applications.",
			AvatarURL: "https://example.com/avatars/jane.jpg",
		},
		{
			ID:        2,
			Name:      "John Smith",
			Email:     "john@example.com",
			Bio:       "Technical writer and developer advocate passionate about making complex topics accessible.",
			AvatarURL: "https://example.com/avatars/john.jpg",
		},
	}

	for _, author := range sampleAuthors {
		a := author // Create copy to avoid pointer issues
		authors[a.ID] = &a
		if a.ID >= nextAuthorID {
			nextAuthorID = a.ID + 1
		}
	}
}

// ListAuthors retrieves a list of all blog authors.
//
// Example:
//
//	GET /authors
func ListAuthors(c *app.Context) {
	c.SetSpanAttribute("operation", "list_authors")
	c.AddSpanEvent("listing_authors")

	// Convert map to slice
	authorList := make([]*Author, 0, len(authors))
	for _, author := range authors {
		authorList = append(authorList, author)
	}

	// Record metrics
	c.IncrementCounter("authors_list_total",
		attribute.Int("count", len(authorList)),
	)

	if err := c.JSON(http.StatusOK, map[string]any{
		"authors": authorList,
		"total":   len(authorList),
	}); err != nil {
		c.Logger().Error("failed to write response", "err", err)
	}
}

// GetAuthor retrieves a single author by ID.
//
// Path parameters:
//   - id: Author ID
//
// Example:
//
//	GET /authors/1
func GetAuthor(c *app.Context) {
	var pathParams struct {
		ID int `path:"id"`
	}
	if err := c.Bind(&pathParams); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("author.id", pathParams.ID)
	c.AddSpanEvent("fetching_author")

	author, exists := authors[pathParams.ID]
	if !exists {
		c.SetSpanAttribute("error", true)
		HandleError(c, ErrAuthorNotFound)
		return
	}

	// Record metrics
	c.IncrementCounter("author_views_total",
		attribute.Int("author_id", pathParams.ID),
	)

	if err := c.JSON(http.StatusOK, author); err != nil {
		c.Logger().Error("failed to write response", "err", err)
	}
}

// GetAuthorPosts retrieves all posts written by a specific author.
//
// Path parameters:
//   - id: Author ID
//
// Example:
//
//	GET /authors/1/posts
func GetAuthorPosts(c *app.Context) {
	var pathParams struct {
		ID int `path:"id"`
	}
	if err := c.Bind(&pathParams); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Check if author exists
	_, exists := authors[pathParams.ID]
	if !exists {
		HandleError(c, ErrAuthorNotFound)
		return
	}

	c.SetSpanAttribute("author.id", pathParams.ID)
	c.AddSpanEvent("fetching_author_posts")

	// Filter posts by author
	var authorPosts []*PostResponse
	for _, post := range posts {
		if post.Author.ID == pathParams.ID {
			authorPosts = append(authorPosts, post)
		}
	}

	// Record metrics
	c.IncrementCounter("author_posts_views_total",
		attribute.Int("author_id", pathParams.ID),
		attribute.Int("post_count", len(authorPosts)),
	)

	if err := c.JSON(http.StatusOK, map[string]any{
		"authorId": pathParams.ID,
		"posts":    authorPosts,
		"total":    len(authorPosts),
	}); err != nil {
		c.Logger().Error("failed to write response", "err", err)
	}
}
