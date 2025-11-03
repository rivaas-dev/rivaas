package handlers

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/router"
)

// GetUserByID retrieves a user by ID using path parameter binding.
func GetUserByID(c *router.Context) {
	var params UserPathParams
	if err := c.BindParams(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Add tracing attributes
	c.SetSpanAttribute("user.id", params.ID)
	c.SetSpanAttribute("operation.type", "read")
	c.AddSpanEvent("user_lookup_started")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	// Record custom metrics
	c.IncrementCounter("user_lookups_total",
		attribute.Int("user_id", params.ID),
		attribute.String("result", "success"),
	)

	// Simulate user not found scenario
	if params.ID == 999 {
		c.SetSpanAttribute("error", true)
		c.SetSpanAttribute("error.type", "user_not_found")
		HandleError(c, ErrUserNotFound)
		return
	}

	c.AddSpanEvent("user_found")
	c.JSON(http.StatusOK, UserResponse{
		ID:        params.ID,
		Name:      "John Doe",
		Email:     "john@example.com",
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
	})
}

// CreateUser creates a new user using request body binding.
func CreateUser(c *router.Context) {
	var req CreateUserRequest
	if err := c.BindBody(&req); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "failed to parse request body: %v", err))
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Add tracing
	c.SetSpanAttribute("operation", "create_user")
	c.SetSpanAttribute("user.email", req.Email)
	c.AddSpanEvent("user_creation_started")

	// Simulate user creation
	time.Sleep(20 * time.Millisecond)

	// Record metrics
	c.IncrementCounter("users_created_total",
		attribute.String("result", "success"),
	)

	c.AddSpanEvent("user_created")
	c.JSON(http.StatusCreated, UserResponse{
		ID:        GenerateUserID(),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	})
}

// GetUserOrders retrieves orders for a user.
func GetUserOrders(c *router.Context) {
	var params UserPathParams
	if err := c.BindParams(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("user.id", params.ID)
	c.SetSpanAttribute("operation", "list_user_orders")

	// Simulate database query
	time.Sleep(15 * time.Millisecond)

	c.JSON(http.StatusOK, map[string]any{
		"user_id": params.ID,
		"orders":  []OrderResponse{}, // Empty for demo
		"total":   0,
	})
}
