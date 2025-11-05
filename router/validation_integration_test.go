package router

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestBindAndValidate_JSON(t *testing.T) {
	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req CreateUserRequest
		if err := c.BindAndValidate(&req); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	// Valid request
	body := `{"name": "John Doe", "email": "john@example.com", "age": 25}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid request - missing required field
	body2 := `{"email": "john@example.com", "age": 25}`
	req2 := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != 400 {
		t.Errorf("expected 400, got %d", w2.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "validation_failed" {
		t.Errorf("expected 'validation_failed', got %v", resp["error"])
	}
}

func TestBindAndValidateStrict_UnknownFields(t *testing.T) {
	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req CreateUserRequest
		if err := c.BindAndValidateStrict(&req); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	// Request with unknown field
	body := `{"name": "John", "email": "john@example.com", "typo_field": "value"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for unknown field, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMustBindAndValidate(t *testing.T) {
	type CreateUserRequest struct {
		Name string `json:"name" validate:"required"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req CreateUserRequest
		if !c.MustBindAndValidate(&req) {
			return // Error already written
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	// Invalid request
	body := `{}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// Valid request
	body2 := `{"name": "John"}`
	req2 := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestBindAndValidateInto(t *testing.T) {
	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		req, err := BindAndValidateInto[CreateUserRequest](c)
		if err != nil {
			c.ValidationError(err, 400)
			return
		}

		if req.Name == "" {
			t.Error("Name should be set")
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	body := `{"name": "John", "email": "john@example.com"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMustBindAndValidateInto(t *testing.T) {
	type CreateUserRequest struct {
		Name string `json:"name" validate:"required"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		req, ok := MustBindAndValidateInto[CreateUserRequest](c)
		if !ok {
			return // Error already written
		}

		if req.Name == "" {
			t.Error("Name should be set")
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	body := `{"name": "John"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBindAndValidate_PartialUpdate(t *testing.T) {
	type UpdateUserRequest struct {
		Name  string `json:"name" validate:"min=3"`
		Email string `json:"email" validate:"email"`
	}

	router := New()
	router.PATCH("/users/:id", func(c *Context) {
		var req UpdateUserRequest
		if err := c.BindAndValidate(&req, WithPartial(true)); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "updated"})
	})

	// PATCH with only name
	body := `{"name": "John"}`
	req := httptest.NewRequest("PATCH", "/users/123", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200 for partial update, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPresenceTracking(t *testing.T) {
	type User struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Address struct {
			City string `json:"city"`
			Zip  string `json:"zip"`
		} `json:"address"`
	}

	body := `{"name": "John", "address": {"city": "NYC"}}`
	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	c := NewContext(httptest.NewRecorder(), req)
	var user User
	if err := c.BindJSON(&user); err != nil {
		t.Fatalf("failed to bind: %v", err)
	}

	pm := c.Presence()
	if pm == nil {
		t.Fatal("presence map should not be nil")
	}

	if !pm.Has("name") {
		t.Error("should have 'name'")
	}
	if pm.Has("email") {
		t.Error("should not have 'email'")
	}
	if !pm.Has("address") {
		t.Error("should have 'address'")
	}
	if !pm.Has("address.city") {
		t.Error("should have 'address.city'")
	}
	if pm.Has("address.zip") {
		t.Error("should not have 'address.zip'")
	}
}

func TestBindAndValidate_LargePayload(t *testing.T) {
	type Item struct {
		Name  string `json:"name" validate:"required"`
		Price int    `json:"price" validate:"min=0"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"required,min=1"`
	}

	// Create a large payload with 1000 items
	items := make([]Item, 1000)
	for i := range items {
		items[i] = Item{
			Name:  "item",
			Price: i,
		}
	}
	order := Order{Items: items}

	orderJSON, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	router := New()
	router.POST("/orders", func(c *Context) {
		var req Order
		if err := c.BindAndValidate(&req); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]int{"items": len(req.Items)})
	})

	req := httptest.NewRequest("POST", "/orders", bytes.NewReader(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBindAndValidate_MalformedJSON(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req User
		if err := c.BindAndValidate(&req); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "ok"})
	})

	// Malformed JSON
	body := `{"name": "John", invalid}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for malformed JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBindAndValidate_ContentTypeEdgeCases(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req User
		if err := c.BindAndValidate(&req); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "ok"})
	})

	testCases := []struct {
		name        string
		contentType string
		body        string
		expectCode  int
	}{
		{
			name:        "standard JSON",
			contentType: "application/json",
			body:        `{"name": "John"}`,
			expectCode:  200,
		},
		{
			name:        "JSON with charset",
			contentType: "application/json; charset=utf-8",
			body:        `{"name": "John"}`,
			expectCode:  200,
		},
		{
			name:        "JSON with parameters",
			contentType: "application/json; charset=utf-8; boundary=something",
			body:        `{"name": "John"}`,
			expectCode:  200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected %d, got %d: %s", tc.expectCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestBindAndValidate_AllStrategiesIntegration(t *testing.T) {
	// Test integration with all validation strategies
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	router := New()
	router.POST("/users", func(c *Context) {
		var req User
		if err := c.BindAndValidate(&req,
			WithStrategy(ValidationTags),
			WithRunAll(true),
		); err != nil {
			c.ValidationError(err, 400)
			return
		}
		c.JSON(200, map[string]string{"status": "created"})
	})

	// Valid request
	body := `{"name": "John", "email": "john@example.com"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid request
	body2 := `{"name": "John"}`
	req2 := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != 400 {
		t.Errorf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}
}
