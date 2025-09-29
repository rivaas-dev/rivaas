package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/rivaas-dev/rivaas/router"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserService struct {
	users  []User
	nextID int
}

func NewUserService() *UserService {
	return &UserService{
		users: []User{
			{ID: 1, Name: "John Doe", Email: "john@example.com"},
			{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
		},
		nextID: 3,
	}
}

func (s *UserService) GetUsers() []User {
	return s.users
}

func (s *UserService) GetUser(id int) *User {
	for _, user := range s.users {
		if user.ID == id {
			return &user
		}
	}
	return nil
}

func (s *UserService) CreateUser(name, email string) User {
	user := User{
		ID:    s.nextID,
		Name:  name,
		Email: email,
	}
	s.users = append(s.users, user)
	s.nextID++
	return user
}

func (s *UserService) UpdateUser(id int, name, email string) *User {
	for i, user := range s.users {
		if user.ID == id {
			s.users[i].Name = name
			s.users[i].Email = email
			return &s.users[i]
		}
	}
	return nil
}

func (s *UserService) DeleteUser(id int) bool {
	for i, user := range s.users {
		if user.ID == id {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return true
		}
	}
	return false
}

func main() {
	r := router.New()
	userService := NewUserService()

	// Global middleware
	r.Use(Logger(), Recovery(), CORS())

	// API routes
	api := r.Group("/api/v1")
	api.Use(JSONMiddleware())

	// User routes
	api.GET("/users", func(c *router.Context) {
		users := userService.GetUsers()
		c.JSON(http.StatusOK, map[string]interface{}{
			"users": users,
		})
	})

	api.GET("/users/:id", func(c *router.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		user := userService.GetUser(id)
		if user == nil {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	api.POST("/users", func(c *router.Context) {
		var userData struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(c.Request.Body).Decode(&userData); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid JSON",
			})
			return
		}

		user := userService.CreateUser(userData.Name, userData.Email)
		c.JSON(http.StatusCreated, user)
	})

	api.PUT("/users/:id", func(c *router.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		var userData struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(c.Request.Body).Decode(&userData); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid JSON",
			})
			return
		}

		user := userService.UpdateUser(id, userData.Name, userData.Email)
		if user == nil {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	api.DELETE("/users/:id", func(c *router.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		if !userService.DeleteUser(id) {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "User not found",
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "User deleted",
		})
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Logger middleware
func Logger() router.HandlerFunc {
	return func(c *router.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[%s] %s %s %v", c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr, time.Since(start))
	}
}

// Recovery middleware
func Recovery() router.HandlerFunc {
	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic: %v", err)
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// CORS middleware
func CORS() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.Status(http.StatusOK)
			return
		}

		c.Next()
	}
}

// JSON middleware
func JSONMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	}
}
