package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
)

// User represents a user in the system
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

func main() {
	r := router.New()
	store := NewUserStore()

	// Global middleware
	r.Use(middleware.Logger(), middleware.Recovery(), middleware.CORS(middleware.WithAllowAllOrigins(true)))

	// API routes
	api := r.Group("/api/v1")
	api.Use(JSONMiddleware())

	// List all users
	api.GET("/users", func(c *router.Context) {
		users := store.List()
		c.JSON(http.StatusOK, map[string]interface{}{
			"users": users,
			"total": len(users),
		})
	})

	// Get single user
	api.GET("/users/:id", func(c *router.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		user, ok := store.Get(id)
		if !ok {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	// Create user
	api.POST("/users", func(c *router.Context) {
		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid JSON",
			})
			return
		}

		if input.Name == "" || input.Email == "" {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Name and email are required",
			})
			return
		}

		user := store.Create(input.Name, input.Email)
		c.JSON(http.StatusCreated, user)
	})

	// Update user
	api.PUT("/users/:id", func(c *router.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid JSON",
			})
			return
		}

		user, ok := store.Update(id, input.Name, input.Email)
		if !ok {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	// Delete user
	api.DELETE("/users/:id", func(c *router.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		if !store.Delete(id) {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "User deleted successfully",
		})
	})

	// Health check
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	log.Println("🚀 REST API Server starting on http://localhost:8080")
	log.Println("\n📝 Try these commands:")
	log.Println("   # List users")
	log.Println("   curl http://localhost:8080/api/v1/users")
	log.Println("\n   # Get user")
	log.Println("   curl http://localhost:8080/api/v1/users/1")
	log.Println("\n   # Create user")
	log.Println("   curl -X POST http://localhost:8080/api/v1/users -H 'Content-Type: application/json' -d '{\"name\":\"Charlie\",\"email\":\"charlie@example.com\"}'")
	log.Println("\n   # Update user")
	log.Println("   curl -X PUT http://localhost:8080/api/v1/users/1 -H 'Content-Type: application/json' -d '{\"name\":\"Alice Updated\",\"email\":\"alice.new@example.com\"}'")
	log.Println("\n   # Delete user")
	log.Println("   curl -X DELETE http://localhost:8080/api/v1/users/2")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// Middleware
func JSONMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	}
}
