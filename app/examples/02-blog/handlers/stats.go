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

// Package handlers provides statistics and analytics handlers for the blog API.
package handlers

import (
	"log/slog"
	"net/http"
	"sort"

	"rivaas.dev/app"
)

// GetBlogStats returns overall blog statistics.
// This is a versioned endpoint (v1).
//
// Example:
//
//	GET /v1/stats
func GetBlogStats(c *app.Context) {
	c.SetSpanAttribute("operation", "get_blog_stats")
	c.SetSpanAttribute("api.version", c.Version())
	c.AddSpanEvent("calculating_blog_stats")

	slog.InfoContext(c.RequestContext(), "calculating blog stats")

	// Calculate statistics
	var totalPosts, publishedPosts, draftPosts, archivedPosts, totalViews int
	for _, post := range posts {
		totalPosts++
		totalViews += post.ViewCount
		switch post.Status {
		case StatusPublished:
			publishedPosts++
		case StatusDraft:
			draftPosts++
		case StatusArchived:
			archivedPosts++
		}
	}

	stats := BlogStatsResponse{
		TotalPosts:     totalPosts,
		PublishedPosts: publishedPosts,
		DraftPosts:     draftPosts,
		ArchivedPosts:  archivedPosts,
		TotalComments:  len(comments),
		TotalAuthors:   len(authors),
		TotalViews:     totalViews,
	}

	if err := c.JSON(http.StatusOK, stats); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}

// GetPopularPosts returns the most viewed blog posts.
// This is a versioned endpoint (v1).
//
// Query parameters:
//   - limit: Number of posts to return (default: 10, max: 50)
//
// Example:
//
//	GET /v1/popular?limit=5
func GetPopularPosts(c *app.Context) {
	var params struct {
		Limit int `query:"limit" default:"10"`
	}
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Validate limit
	if params.Limit < 1 {
		params.Limit = 10
	}
	if params.Limit > 50 {
		params.Limit = 50
	}

	c.SetSpanAttribute("operation", "get_popular_posts")
	c.SetSpanAttribute("api.version", c.Version())
	c.SetSpanAttribute("query.limit", params.Limit)
	c.AddSpanEvent("fetching_popular_posts")

	// Collect all published posts
	var publishedPosts []*PostResponse
	for _, post := range posts {
		if post.Status == StatusPublished {
			publishedPosts = append(publishedPosts, post)
		}
	}

	// Sort by view count (descending)
	sort.Slice(publishedPosts, func(i, j int) bool {
		return publishedPosts[i].ViewCount > publishedPosts[j].ViewCount
	})

	// Limit results
	if len(publishedPosts) > params.Limit {
		publishedPosts = publishedPosts[:params.Limit]
	}

	// Convert to popular post response
	popularPosts := make([]PopularPostResponse, len(publishedPosts))
	for i, post := range publishedPosts {
		popularPosts[i] = PopularPostResponse{
			ID:        post.ID,
			Slug:      post.Slug,
			Title:     post.Title,
			ViewCount: post.ViewCount,
		}
	}

	if err := c.JSON(http.StatusOK, map[string]any{
		"posts": popularPosts,
		"total": len(popularPosts),
		"limit": params.Limit,
	}); err != nil {
		slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
	}
}
