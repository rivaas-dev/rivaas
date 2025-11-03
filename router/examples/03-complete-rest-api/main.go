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

// Package main demonstrates a complete REST API with CRUD operations, request binding,
// structured error handling, and pagination.
package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/logger"
	"rivaas.dev/router/middleware/recovery"
)

// APIError represents a structured error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	Path    string `json:"path,omitempty"`
}

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// User represents a user in the system
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post represents a post belonging to a user
type Post struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// UserStore manages users in memory
type UserStore struct {
	mu     sync.RWMutex
	users  map[int]User
	nextID int
}

func NewUserStore() *UserStore {
	return &UserStore{
		users: map[int]User{
			1: {ID: 1, Name: "Alice Johnson", Email: "alice@example.com", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			2: {ID: 2, Name: "Bob Smith", Email: "bob@example.com", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
		nextID: 3,
	}
}

func (s *UserStore) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func (s *UserStore) Get(id int) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	return user, ok
}

func (s *UserStore) Create(name, email string) User {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := User{
		ID:        s.nextID,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.users[s.nextID] = user
	s.nextID++
	return user
}

func (s *UserStore) Update(id int, name, email string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return User{}, false
	}

	user.Name = name
	user.Email = email
	user.UpdatedAt = time.Now()
	s.users[id] = user
	return user, true
}

func (s *UserStore) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

// PostStore manages posts in memory
type PostStore struct {
	mu     sync.RWMutex
	posts  map[int]Post
	nextID int
}

func NewPostStore() *PostStore {
	return &PostStore{
		posts:  make(map[int]Post),
		nextID: 1,
	}
}

func (s *PostStore) Create(userID int, title, content string) Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	post := Post{
		ID:        s.nextID,
		UserID:    userID,
		Title:     title,
		Content:   content,
		CreatedAt: time.Now(),
	}
	s.posts[s.nextID] = post
	s.nextID++
	return post
}

func (s *PostStore) GetByUser(userID int) []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	posts := make([]Post, 0)
	for _, post := range s.posts {
		if post.UserID == userID {
			posts = append(posts, post)
		}
	}
	return posts
}

func main() {
	r := router.New()
	userStore := NewUserStore()
	postStore := NewPostStore()

	// Global middleware
	r.Use(logger.New(), recovery.New(), cors.New(cors.WithAllowAllOrigins(true)))

	// API routes
	api := r.Group("/api/v1")

	// User Routes: CRUD operations with binding and error handling
	// All routes use structured APIError responses for consistent error handling

	// List users with pagination
	// Uses BindQuery to extract query parameters (e.g., ?page=1&page_size=10)
	api.GET("/users", func(c *router.Context) {
		type ListParams struct {
			Page     int `query:"page"`
			PageSize int `query:"page_size"`
		}

		var params ListParams
		if err := c.BindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_QUERY",
				Message: "Invalid query parameters",
				Details: err.Error(),
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Apply defaults if not provided
		if params.Page == 0 {
			params.Page = 1
		}
		if params.PageSize == 0 {
			params.PageSize = 10
		}

		users := userStore.List()
		c.JSON(http.StatusOK, map[string]interface{}{
			"users":     users,
			"total":     len(users),
			"page":      params.Page,
			"page_size": params.PageSize,
		})
	})

	// Get user by ID
	// Uses BindParams to extract path parameters from URL (e.g., /users/123)
	api.GET("/users/:id", func(c *router.Context) {
		type PathParams struct {
			ID int `params:"id"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_USER_ID",
				Message: "Invalid user ID format",
				Path:    c.Request.URL.Path,
			})
			return
		}

		user, ok := userStore.Get(params.ID)
		if !ok {
			c.JSON(http.StatusNotFound, APIError{
				Code:    "USER_NOT_FOUND",
				Message: fmt.Sprintf("User with ID %d not found", params.ID),
				Path:    c.Request.URL.Path,
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	// Create user with validation
	// Uses BindBody to extract and parse JSON request body
	api.POST("/users", func(c *router.Context) {
		type CreateUserRequest struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var req CreateUserRequest
		if err := c.BindBody(&req); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_JSON",
				Message: "Invalid JSON body",
				Details: err.Error(),
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Business logic validation
		var validationErrors []ValidationError
		if req.Name == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "name",
				Message: "Name is required",
			})
		}
		if req.Email == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "email",
				Message: "Email is required",
			})
		} else if !contains(req.Email, "@") {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "email",
				Message: "Invalid email format",
			})
		}

		if len(validationErrors) > 0 {
			c.JSON(http.StatusUnprocessableEntity, APIError{
				Code:    "VALIDATION_ERROR",
				Message: "Validation failed",
				Details: validationErrors,
				Path:    c.Request.URL.Path,
			})
			return
		}

		user := userStore.Create(req.Name, req.Email)
		c.JSON(http.StatusCreated, user)
	})

	// Update user
	// Uses BindParams for path parameter and BindBody for request body
	api.PUT("/users/:id", func(c *router.Context) {
		type PathParams struct {
			ID int `params:"id"`
		}

		type UpdateUserRequest struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_USER_ID",
				Message: "Invalid user ID format",
				Path:    c.Request.URL.Path,
			})
			return
		}

		var req UpdateUserRequest
		if err := c.BindBody(&req); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_JSON",
				Message: "Invalid JSON body",
				Details: err.Error(),
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Business logic validation
		var validationErrors []ValidationError
		if req.Name == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "name",
				Message: "Name is required",
			})
		}
		if req.Email == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "email",
				Message: "Email is required",
			})
		}

		if len(validationErrors) > 0 {
			c.JSON(http.StatusUnprocessableEntity, APIError{
				Code:    "VALIDATION_ERROR",
				Message: "Validation failed",
				Details: validationErrors,
				Path:    c.Request.URL.Path,
			})
			return
		}

		user, ok := userStore.Update(params.ID, req.Name, req.Email)
		if !ok {
			c.JSON(http.StatusNotFound, APIError{
				Code:    "USER_NOT_FOUND",
				Message: fmt.Sprintf("User with ID %d not found", params.ID),
				Path:    c.Request.URL.Path,
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	// Delete user
	// Uses BindParams to extract user ID from path
	api.DELETE("/users/:id", func(c *router.Context) {
		type PathParams struct {
			ID int `params:"id"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_USER_ID",
				Message: "Invalid user ID format",
				Path:    c.Request.URL.Path,
			})
			return
		}

		if !userStore.Delete(params.ID) {
			c.JSON(http.StatusNotFound, APIError{
				Code:    "USER_NOT_FOUND",
				Message: fmt.Sprintf("User with ID %d not found", params.ID),
				Path:    c.Request.URL.Path,
			})
			return
		}

		c.Status(http.StatusNoContent)
	})

	// Nested Resource: User Posts

	// Get all posts for a user
	api.GET("/users/:id/posts", func(c *router.Context) {
		type PathParams struct {
			UserID int `params:"id"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_USER_ID",
				Message: "Invalid user ID format",
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Verify user exists
		if _, ok := userStore.Get(params.UserID); !ok {
			c.JSON(http.StatusNotFound, APIError{
				Code:    "USER_NOT_FOUND",
				Message: fmt.Sprintf("User with ID %d not found", params.UserID),
				Path:    c.Request.URL.Path,
			})
			return
		}

		posts := postStore.GetByUser(params.UserID)
		c.JSON(http.StatusOK, map[string]interface{}{
			"user_id": params.UserID,
			"posts":   posts,
			"total":   len(posts),
		})
	})

	// Create post for a user
	api.POST("/users/:id/posts", func(c *router.Context) {
		type PathParams struct {
			UserID int `params:"id"`
		}

		type CreatePostRequest struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_USER_ID",
				Message: "Invalid user ID format",
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Verify user exists
		if _, ok := userStore.Get(params.UserID); !ok {
			c.JSON(http.StatusNotFound, APIError{
				Code:    "USER_NOT_FOUND",
				Message: fmt.Sprintf("User with ID %d not found", params.UserID),
				Path:    c.Request.URL.Path,
			})
			return
		}

		var req CreatePostRequest
		if err := c.BindBody(&req); err != nil {
			c.JSON(http.StatusBadRequest, APIError{
				Code:    "INVALID_JSON",
				Message: "Invalid JSON body",
				Details: err.Error(),
				Path:    c.Request.URL.Path,
			})
			return
		}

		// Business logic validation
		var validationErrors []ValidationError
		if req.Title == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "title",
				Message: "Title is required",
			})
		}
		if req.Content == "" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "content",
				Message: "Content is required",
			})
		}

		if len(validationErrors) > 0 {
			c.JSON(http.StatusUnprocessableEntity, APIError{
				Code:    "VALIDATION_ERROR",
				Message: "Validation failed",
				Details: validationErrors,
				Path:    c.Request.URL.Path,
			})
			return
		}

		post := postStore.Create(params.UserID, req.Title, req.Content)
		c.JSON(http.StatusCreated, post)
	})

	// Health check
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET    /api/v1/users              # List users with pagination")
	logger.Print("  GET    /api/v1/users/:id          # Get user by ID")
	logger.Print("  POST   /api/v1/users              # Create user")
	logger.Print("  PUT    /api/v1/users/:id          # Update user")
	logger.Print("  DELETE /api/v1/users/:id          # Delete user")
	logger.Print("  GET    /api/v1/users/:id/posts    # Get user posts")
	logger.Print("  POST   /api/v1/users/:id/posts    # Create post for user")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/api/v1/users")
	logger.Print("  curl http://localhost:8080/api/v1/users?page=1&page_size=5")
	logger.Print("  curl http://localhost:8080/api/v1/users/1")
	logger.Print(`  curl -X POST http://localhost:8080/api/v1/users \`)
	logger.Print(`    -H 'Content-Type: application/json' \`)
	logger.Print(`    -d '{"name":"Charlie","email":"charlie@example.com"}'`)
	logger.Print(`  curl -X PUT http://localhost:8080/api/v1/users/1 \`)
	logger.Print(`    -H 'Content-Type: application/json' \`)
	logger.Print(`    -d '{"name":"Alice Updated","email":"alice.new@example.com"}'`)
	logger.Print("  curl -X DELETE http://localhost:8080/api/v1/users/2")
	logger.Print(`  curl -X POST http://localhost:8080/api/v1/users/1/posts \`)
	logger.Print(`    -H 'Content-Type: application/json' \`)
	logger.Print(`    -d '{"title":"My Post","content":"Post content here"}'`)
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

// findSubstring performs substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
