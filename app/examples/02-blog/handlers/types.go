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

// Package handlers provides request and response type definitions for the blog API.
package handlers

import (
	"regexp"
	"slices"
	"time"
)

// PostStatus represents the publication state of a blog post
type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
)

// ValidStatuses returns all valid post statuses
func ValidStatuses() []PostStatus {
	return []PostStatus{StatusDraft, StatusPublished, StatusArchived}
}

// IsValid checks if the status is a valid PostStatus
func (s PostStatus) IsValid() bool {
	return slices.Contains(ValidStatuses(), s)
}

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// CreatePostRequest represents a blog post creation request
type CreatePostRequest struct {
	Title    string     `json:"title" form:"title"`
	Slug     string     `json:"slug" form:"slug"`
	Content  string     `json:"content" form:"content"` // Markdown
	Excerpt  string     `json:"excerpt,omitempty" form:"excerpt"`
	AuthorID int        `json:"authorId" form:"authorId"`
	Tags     []string   `json:"tags,omitempty" form:"tags"`
	Status   PostStatus `json:"status" form:"status"`
}

// Validate validates the CreatePostRequest
func (r *CreatePostRequest) Validate() error {
	if r.Title == "" {
		return WrapError(ErrValidationFailed, "title is required")
	}
	if len(r.Title) > 200 {
		return WrapError(ErrValidationFailed, "title must be 200 characters or less")
	}
	if r.Slug == "" {
		return WrapError(ErrValidationFailed, "slug is required")
	}
	if !slugRegex.MatchString(r.Slug) {
		return WrapError(ErrValidationFailed, "slug must be lowercase alphanumeric with hyphens (e.g., 'my-post-title')")
	}
	if r.Content == "" {
		return WrapError(ErrValidationFailed, "content is required")
	}
	if len(r.Content) > 50000 {
		return WrapError(ErrValidationFailed, "content must be 50000 characters or less")
	}
	if r.AuthorID <= 0 {
		return WrapError(ErrValidationFailed, "authorId must be a positive integer")
	}
	if r.Status == "" {
		r.Status = StatusDraft // Default to draft
	}
	if !r.Status.IsValid() {
		return WrapError(ErrValidationFailed, "status must be one of: draft, published, archived")
	}
	return nil
}

// UpdatePostRequest represents a blog post update request
type UpdatePostRequest struct {
	Title   *string    `json:"title,omitempty" form:"title"`
	Content *string    `json:"content,omitempty" form:"content"`
	Excerpt *string    `json:"excerpt,omitempty" form:"excerpt"`
	Tags    []string   `json:"tags,omitempty" form:"tags"`
	Status  PostStatus `json:"status,omitempty" form:"status"`
}

// Validate validates the UpdatePostRequest
func (r *UpdatePostRequest) Validate() error {
	if r.Title != nil && *r.Title == "" {
		return WrapError(ErrValidationFailed, "title cannot be empty")
	}
	if r.Title != nil && len(*r.Title) > 200 {
		return WrapError(ErrValidationFailed, "title must be 200 characters or less")
	}
	if r.Content != nil && *r.Content == "" {
		return WrapError(ErrValidationFailed, "content cannot be empty")
	}
	if r.Content != nil && len(*r.Content) > 50000 {
		return WrapError(ErrValidationFailed, "content must be 50000 characters or less")
	}
	if r.Status != "" && !r.Status.IsValid() {
		return WrapError(ErrValidationFailed, "status must be one of: draft, published, archived")
	}
	return nil
}

// PostResponse represents a blog post in API responses
type PostResponse struct {
	ID          int        `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Excerpt     string     `json:"excerpt,omitempty"`
	Author      Author     `json:"author"`
	Tags        []string   `json:"tags"`
	Status      PostStatus `json:"status"`
	ViewCount   int        `json:"viewCount"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Author represents a blog author
type Author struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// CreateAuthorRequest represents an author creation request
type CreateAuthorRequest struct {
	Name      string `json:"name" form:"name"`
	Email     string `json:"email" form:"email"`
	Bio       string `json:"bio,omitempty" form:"bio"`
	AvatarURL string `json:"avatarUrl,omitempty" form:"avatarUrl"`
}

// Validate validates the CreateAuthorRequest
func (r *CreateAuthorRequest) Validate() error {
	if r.Name == "" {
		return WrapError(ErrValidationFailed, "name is required")
	}
	if len(r.Name) < 2 || len(r.Name) > 100 {
		return WrapError(ErrValidationFailed, "name must be between 2 and 100 characters")
	}
	if r.Email == "" {
		return WrapError(ErrValidationFailed, "email is required")
	}
	// Simple email validation
	if len(r.Email) < 3 || !regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`).MatchString(r.Email) {
		return WrapError(ErrValidationFailed, "email must be a valid email address")
	}
	return nil
}

// ListPostsParams represents query parameters for listing posts
type ListPostsParams struct {
	Page     int        `query:"page" default:"1"`
	PerPage  int        `query:"perPage" default:"10"`
	Status   PostStatus `query:"status"`
	Tag      string     `query:"tag"`
	AuthorID *int       `query:"authorId"`
	SortBy   string     `query:"sortBy" default:"date"`
}

// Validate validates the list posts params
func (p *ListPostsParams) Validate() error {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 || p.PerPage > 100 {
		p.PerPage = 10
	}
	if p.Status != "" && !p.Status.IsValid() {
		return WrapError(ErrValidationFailed, "status filter must be one of: draft, published, archived")
	}
	validSorts := []string{"date", "views", "title"}
	if p.SortBy != "" && !slices.Contains(validSorts, p.SortBy) {
		return WrapError(ErrValidationFailed, "sortBy must be one of: date, views, title")
	}
	return nil
}

// CreateCommentRequest represents a comment creation request
type CreateCommentRequest struct {
	Content     string `json:"content" form:"content"`
	AuthorName  string `json:"authorName,omitempty" form:"authorName"`   // For guest comments
	AuthorEmail string `json:"authorEmail,omitempty" form:"authorEmail"` // For guest comments
}

// Validate validates the CreateCommentRequest
func (r *CreateCommentRequest) Validate() error {
	if r.Content == "" {
		return WrapError(ErrValidationFailed, "content is required")
	}
	if len(r.Content) > 5000 {
		return WrapError(ErrValidationFailed, "content must be 5000 characters or less")
	}
	// If author name is provided, validate it
	if r.AuthorName != "" && len(r.AuthorName) > 100 {
		return WrapError(ErrValidationFailed, "authorName must be 100 characters or less")
	}
	return nil
}

// CommentResponse represents a comment on a post
type CommentResponse struct {
	ID          int       `json:"id"`
	PostID      int       `json:"postId"`
	Content     string    `json:"content"`
	AuthorName  string    `json:"authorName"`
	AuthorEmail string    `json:"authorEmail,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// BlogStatsResponse represents blog statistics
type BlogStatsResponse struct {
	TotalPosts     int `json:"totalPosts"`
	PublishedPosts int `json:"publishedPosts"`
	DraftPosts     int `json:"draftPosts"`
	ArchivedPosts  int `json:"archivedPosts"`
	TotalComments  int `json:"totalComments"`
	TotalAuthors   int `json:"totalAuthors"`
	TotalViews     int `json:"totalViews"`
}

// PopularPostResponse represents a popular post summary
type PopularPostResponse struct {
	ID        int    `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	ViewCount int    `json:"viewCount"`
}
