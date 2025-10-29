package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// customUUID is a custom type for testing encoding.TextUnmarshaler interface
type customUUID string

// UnmarshalText implements encoding.TextUnmarshaler
func (u *customUUID) UnmarshalText(text []byte) error {
	s := string(text)
	// Simple UUID validation (just check length, not full RFC4122)
	if len(s) != 36 {
		return errors.New("invalid UUID format: must be 36 characters")
	}
	*u = customUUID(s)
	return nil
}

// Test BindJSON
func TestBindJSON(t *testing.T) {
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	t.Run("valid JSON", func(t *testing.T) {
		body := `{"name":"John","email":"john@example.com","age":30}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindJSON(&user); err != nil {
			t.Fatalf("BindJSON failed: %v", err)
		}

		if user.Name != "John" || user.Email != "john@example.com" || user.Age != 30 {
			t.Errorf("BindJSON got %+v, want {John john@example.com 30}", user)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		body := `{"name":"John","age":"invalid"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindJSON(&user); err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})

	t.Run("nil body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindJSON(&user); err == nil {
			t.Error("Expected error for nil body, got nil")
		}
	})
}

// Test BindQuery
func TestBindQuery(t *testing.T) {
	type SearchParams struct {
		Query    string `query:"q"`
		Page     int    `query:"page"`
		PageSize int    `query:"page_size"`
		Active   bool   `query:"active"`
	}

	t.Run("all fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?q=golang&page=2&page_size=20&active=true", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SearchParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Query != "golang" || params.Page != 2 || params.PageSize != 20 || !params.Active {
			t.Errorf("BindQuery got %+v", params)
		}
	})

	t.Run("partial fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?q=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SearchParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Query != "test" || params.Page != 0 {
			t.Errorf("BindQuery got %+v", params)
		}
	})

	t.Run("invalid integer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?page=invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SearchParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid integer")
		}

		// Check it's a BindError
		var bindErr *BindError
		if !errors.As(err, &bindErr) {
			t.Errorf("Expected BindError, got %T", err)
		}
	})
}

// Test BindParams
func TestBindParams(t *testing.T) {
	type UserParams struct {
		ID     int    `params:"id"`
		Action string `params:"action"`
	}

	t.Run("valid params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/edit", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Simulate router setting params
		c.paramCount = 2
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"
		c.paramKeys[1] = "action"
		c.paramValues[1] = "edit"

		var params UserParams
		if err := c.BindParams(&params); err != nil {
			t.Fatalf("BindParams failed: %v", err)
		}

		if params.ID != 123 || params.Action != "edit" {
			t.Errorf("BindParams got %+v, want {123 edit}", params)
		}
	})

	t.Run("params from map (>8 params)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Simulate fallback to map
		c.Params = map[string]string{
			"id":     "456",
			"action": "view",
		}

		var params UserParams
		if err := c.BindParams(&params); err != nil {
			t.Fatalf("BindParams failed: %v", err)
		}

		if params.ID != 456 || params.Action != "view" {
			t.Errorf("BindParams got %+v", params)
		}
	})
}

// Test BindCookies
func TestBindCookies(t *testing.T) {
	type SessionCookies struct {
		SessionID  string `cookie:"session_id"`
		Theme      string `cookie:"theme"`
		RememberMe bool   `cookie:"remember_me"`
	}

	t.Run("valid cookies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})
		req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
		req.AddCookie(&http.Cookie{Name: "remember_me", Value: "true"})
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var cookies SessionCookies
		if err := c.BindCookies(&cookies); err != nil {
			t.Fatalf("BindCookies failed: %v", err)
		}

		if cookies.SessionID != "abc123" || cookies.Theme != "dark" || !cookies.RememberMe {
			t.Errorf("BindCookies got %+v", cookies)
		}
	})

	t.Run("URL encoded cookies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// Cookie with URL-encoded value
		req.AddCookie(&http.Cookie{Name: "session_id", Value: url.QueryEscape("value with spaces")})
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var cookies struct {
			SessionID string `cookie:"session_id"`
		}
		if err := c.BindCookies(&cookies); err != nil {
			t.Fatalf("BindCookies failed: %v", err)
		}

		if cookies.SessionID != "value with spaces" {
			t.Errorf("BindCookies got %q, want %q", cookies.SessionID, "value with spaces")
		}
	})
}

// Test BindHeaders
func TestBindHeaders(t *testing.T) {
	type RequestHeaders struct {
		UserAgent string `header:"User-Agent"`
		Token     string `header:"Authorization"`
		Accept    string `header:"Accept"`
	}

	t.Run("valid headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Authorization", "Bearer token123")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var headers RequestHeaders
		if err := c.BindHeaders(&headers); err != nil {
			t.Fatalf("BindHeaders failed: %v", err)
		}

		if headers.UserAgent != "Mozilla/5.0" || headers.Token != "Bearer token123" || headers.Accept != "application/json" {
			t.Errorf("BindHeaders got %+v", headers)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("user-agent", "Test")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var headers struct {
			UserAgent string `header:"User-Agent"`
		}
		if err := c.BindHeaders(&headers); err != nil {
			t.Fatalf("BindHeaders failed: %v", err)
		}

		if headers.UserAgent != "Test" {
			t.Errorf("BindHeaders got %q, want %q", headers.UserAgent, "Test")
		}
	})
}

// Test BindBody with different content types
func TestBindBody(t *testing.T) {
	type User struct {
		Name  string `json:"name" form:"name"`
		Email string `json:"email" form:"email"`
		Age   int    `json:"age" form:"age"`
	}

	t.Run("JSON content type", func(t *testing.T) {
		body := `{"name":"Alice","email":"alice@example.com","age":25}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindBody(&user); err != nil {
			t.Fatalf("BindBody failed: %v", err)
		}

		if user.Name != "Alice" || user.Age != 25 {
			t.Errorf("BindBody got %+v", user)
		}
	})

	t.Run("form content type", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "Bob")
		form.Set("email", "bob@example.com")
		form.Set("age", "35")

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindBody(&user); err != nil {
			t.Fatalf("BindBody failed: %v", err)
		}

		if user.Name != "Bob" || user.Age != 35 {
			t.Errorf("BindBody got %+v", user)
		}
	})

	t.Run("default to JSON when no content type", func(t *testing.T) {
		body := `{"name":"Charlie","email":"charlie@example.com","age":40}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		// No Content-Type header
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		if err := c.BindBody(&user); err != nil {
			t.Fatalf("BindBody failed: %v", err)
		}

		if user.Name != "Charlie" {
			t.Errorf("BindBody got %+v", user)
		}
	})

	t.Run("unsupported content type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader("data"))
		req.Header.Set("Content-Type", "application/octet-stream")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var user User
		err := c.BindBody(&user)
		if err == nil {
			t.Error("Expected error for unsupported content type")
		}
		if !strings.Contains(err.Error(), "unsupported content type") {
			t.Errorf("Expected 'unsupported content type' error, got: %v", err)
		}
	})
}

// Test slice binding
func TestBindQuery_Slices(t *testing.T) {
	type TagRequest struct {
		Tags []string `query:"tags"`
		IDs  []int    `query:"ids"`
	}

	t.Run("string slice", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tags=go&tags=rust&tags=python", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TagRequest
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.Tags) != 3 {
			t.Fatalf("Expected 3 tags, got %d", len(params.Tags))
		}
		if params.Tags[0] != "go" || params.Tags[1] != "rust" || params.Tags[2] != "python" {
			t.Errorf("Tags = %v", params.Tags)
		}
	})

	t.Run("int slice", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?ids=1&ids=2&ids=3", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TagRequest
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.IDs) != 3 {
			t.Fatalf("Expected 3 IDs, got %d", len(params.IDs))
		}
		if params.IDs[0] != 1 || params.IDs[1] != 2 || params.IDs[2] != 3 {
			t.Errorf("IDs = %v", params.IDs)
		}
	})

	t.Run("invalid int in slice", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?ids=1&ids=invalid&ids=3", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TagRequest
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid integer in slice")
		}
	})
}

// Test pointer fields
func TestBindQuery_Pointers(t *testing.T) {
	type OptionalParams struct {
		Name   *string `query:"name"`
		Age    *int    `query:"age"`
		Active *bool   `query:"active"`
	}

	t.Run("all values present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=John&age=30&active=true", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params OptionalParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name == nil || *params.Name != "John" {
			t.Error("Name pointer not set correctly")
		}
		if params.Age == nil || *params.Age != 30 {
			t.Error("Age pointer not set correctly")
		}
		if params.Active == nil || !*params.Active {
			t.Error("Active pointer not set correctly")
		}
	})

	t.Run("missing values remain nil", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=John", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params OptionalParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name == nil || *params.Name != "John" {
			t.Error("Name should be set")
		}
		if params.Age != nil {
			t.Error("Age should be nil when not provided")
		}
	})

	t.Run("empty value remains nil", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=&age=", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params OptionalParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name != nil {
			t.Error("Name should be nil for empty value")
		}
		if params.Age != nil {
			t.Error("Age should be nil for empty value")
		}
	})
}

// Test various data types
func TestBindQuery_DataTypes(t *testing.T) {
	type AllTypes struct {
		String  string  `query:"string"`
		Int     int     `query:"int"`
		Int8    int8    `query:"int8"`
		Int16   int16   `query:"int16"`
		Int32   int32   `query:"int32"`
		Uint    uint    `query:"uint"`
		Float32 float32 `query:"float32"`
		Float64 float64 `query:"float64"`
		Bool    bool    `query:"bool"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	// Build query parameters programmatically to avoid URL length issues
	q := req.URL.Query()
	q.Set("string", "test")
	q.Set("int", "-42")
	q.Set("int8", "127")
	q.Set("int16", "32000")
	q.Set("int32", "2147483647")
	q.Set("uint", "42")
	q.Set("float32", "3.14")
	q.Set("float64", "2.718281828")
	q.Set("bool", "true")
	req.URL.RawQuery = q.Encode()

	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params AllTypes
	if err := c.BindQuery(&params); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}

	if params.String != "test" {
		t.Errorf("String = %v", params.String)
	}
	if params.Int != -42 {
		t.Errorf("Int = %v", params.Int)
	}
	if params.Int8 != 127 {
		t.Errorf("Int8 = %v", params.Int8)
	}
	if params.Bool != true {
		t.Errorf("Bool = %v", params.Bool)
	}
	if params.Float32 < 3.13 || params.Float32 > 3.15 {
		t.Errorf("Float32 = %v", params.Float32)
	}
}

// Test boolean parsing variations
func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		wantErr  bool
	}{
		// True values
		{"true", true, false},
		{"True", true, false},
		{"TRUE", true, false},
		{"1", true, false},
		{"yes", true, false},
		{"Yes", true, false},
		{"on", true, false},
		{"t", true, false},
		{"y", true, false},

		// False values
		{"false", false, false},
		{"False", false, false},
		{"0", false, false},
		{"no", false, false},
		{"off", false, false},
		{"f", false, false},
		{"n", false, false},
		{"", false, false},

		// Invalid values
		{"invalid", false, true},
		{"maybe", false, true},
		{"2", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Test BindForm
func TestBindForm(t *testing.T) {
	type LoginForm struct {
		Username string `form:"username"`
		Password string `form:"password"`
		Remember bool   `form:"remember"`
	}

	t.Run("urlencoded form", func(t *testing.T) {
		form := url.Values{}
		form.Set("username", "alice")
		form.Set("password", "secret123")
		form.Set("remember", "true")

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var login LoginForm
		if err := c.BindForm(&login); err != nil {
			t.Fatalf("BindForm failed: %v", err)
		}

		if login.Username != "alice" || login.Password != "secret123" || !login.Remember {
			t.Errorf("BindForm got %+v", login)
		}
	})
}

// Test error cases
func TestBind_Errors(t *testing.T) {
	t.Run("not a pointer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params struct {
			Name string `query:"name"`
		}
		err := c.BindQuery(params) // Not a pointer!
		if err == nil {
			t.Error("Expected error for non-pointer")
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params *struct {
			Name string `query:"name"`
		}
		err := c.BindQuery(params)
		if err == nil {
			t.Error("Expected error for nil pointer")
		}
	})

	t.Run("not a struct", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var str string
		err := c.BindQuery(&str)
		if err == nil {
			t.Error("Expected error for non-struct")
		}
	})
}

// Test type cache efficiency
func TestStructInfoCache(t *testing.T) {
	type TestStruct struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?name=test&age=25", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// First call - should populate cache
	var s1 TestStruct
	if err := c.BindQuery(&s1); err != nil {
		t.Fatalf("First BindQuery failed: %v", err)
	}

	// Second call - should use cache
	var s2 TestStruct
	if err := c.BindQuery(&s2); err != nil {
		t.Fatalf("Second BindQuery failed: %v", err)
	}

	// Both should have same values
	if s1.Name != s2.Name || s1.Age != s2.Age {
		t.Error("Cached binding produced different results")
	}
}

// Test BindError details
func TestBindError_Details(t *testing.T) {
	type Params struct {
		Age int `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?age=invalid", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params Params
	err := c.BindQuery(&params)

	if err == nil {
		t.Fatal("Expected BindError")
	}

	var bindErr *BindError
	if !errors.As(err, &bindErr) {
		t.Fatalf("Expected BindError, got %T", err)
	}

	if bindErr.Field != "Age" {
		t.Errorf("Field = %q, want %q", bindErr.Field, "Age")
	}
	if bindErr.Tag != "query" {
		t.Errorf("Tag = %q, want %q", bindErr.Tag, "query")
	}
	if bindErr.Value != "invalid" {
		t.Errorf("Value = %q, want %q", bindErr.Value, "invalid")
	}
}

// Test real-world scenarios
func TestBindQuery_RealWorld(t *testing.T) {
	t.Run("pagination", func(t *testing.T) {
		type Pagination struct {
			Page     int    `query:"page"`
			PageSize int    `query:"page_size"`
			Sort     string `query:"sort"`
			Order    string `query:"order"`
		}

		req := httptest.NewRequest("GET", "/?page=3&page_size=50&sort=created_at&order=desc", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var p Pagination
		if err := c.BindQuery(&p); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if p.Page != 3 || p.PageSize != 50 || p.Sort != "created_at" || p.Order != "desc" {
			t.Errorf("Pagination = %+v", p)
		}
	})

	t.Run("filters", func(t *testing.T) {
		type Filters struct {
			Status   []string `query:"status"`
			Category []string `query:"category"`
			MinPrice float64  `query:"min_price"`
			MaxPrice float64  `query:"max_price"`
		}

		req := httptest.NewRequest("GET", "/?status=active&status=pending&category=electronics&min_price=10.50&max_price=99.99", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var f Filters
		if err := c.BindQuery(&f); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(f.Status) != 2 || f.Status[0] != "active" || f.Status[1] != "pending" {
			t.Errorf("Status = %v", f.Status)
		}
		if f.MinPrice != 10.50 || f.MaxPrice != 99.99 {
			t.Errorf("Prices = %v, %v", f.MinPrice, f.MaxPrice)
		}
	})
}

// Test time.Time support
func TestBindQuery_TimeType(t *testing.T) {
	type EventParams struct {
		StartDate time.Time  `query:"start"`
		EndDate   time.Time  `query:"end"`
		Created   *time.Time `query:"created"`
	}

	t.Run("RFC3339 format", func(t *testing.T) {
		start := "2024-01-15T10:30:00Z"
		end := "2024-01-20T15:45:00Z"

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("start", start)
		q.Set("end", end)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params EventParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		expectedStart, _ := time.Parse(time.RFC3339, start)
		if !params.StartDate.Equal(expectedStart) {
			t.Errorf("StartDate = %v, want %v", params.StartDate, expectedStart)
		}
	})

	t.Run("date only format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?start=2024-01-15", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params EventParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		expected, _ := time.Parse("2006-01-02", "2024-01-15")
		if !params.StartDate.Equal(expected) {
			t.Errorf("StartDate = %v, want %v", params.StartDate, expected)
		}
	})

	t.Run("pointer time field", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?created=2024-01-15T10:00:00Z", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params EventParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Created == nil {
			t.Fatal("Created should not be nil")
		}

		expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
		if !params.Created.Equal(expected) {
			t.Errorf("Created = %v, want %v", *params.Created, expected)
		}
	})

	t.Run("invalid time format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?start=invalid-date", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params EventParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid time format")
		}
	})

	t.Run("time slice", func(t *testing.T) {
		type DateList struct {
			Dates []time.Time `query:"dates"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Add("dates", "2024-01-15")
		q.Add("dates", "2024-01-16")
		q.Add("dates", "2024-01-17T10:00:00Z")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params DateList
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.Dates) != 3 {
			t.Fatalf("Expected 3 dates, got %d", len(params.Dates))
		}
	})
}

// Test time.Duration support
func TestBindQuery_DurationType(t *testing.T) {
	type TimeoutParams struct {
		Timeout  time.Duration  `query:"timeout"`
		Interval time.Duration  `query:"interval"`
		TTL      *time.Duration `query:"ttl"`
	}

	t.Run("valid durations", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?timeout=5s&interval=10m&ttl=1h", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TimeoutParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want 5s", params.Timeout)
		}
		if params.Interval != 10*time.Minute {
			t.Errorf("Interval = %v, want 10m", params.Interval)
		}
		if params.TTL == nil || *params.TTL != time.Hour {
			t.Errorf("TTL = %v, want 1h", params.TTL)
		}
	})

	t.Run("complex duration", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?timeout=1h30m45s", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TimeoutParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		expected := time.Hour + 30*time.Minute + 45*time.Second
		if params.Timeout != expected {
			t.Errorf("Timeout = %v, want %v", params.Timeout, expected)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?timeout=invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TimeoutParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid duration")
		}
	})
}

// Test net.IP support
func TestBindQuery_IPType(t *testing.T) {
	type NetworkParams struct {
		AllowedIP net.IP   `query:"allowed_ip"`
		BlockedIP net.IP   `query:"blocked_ip"`
		IPs       []net.IP `query:"ips"`
	}

	t.Run("IPv4 address", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?allowed_ip=192.168.1.1", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		expected := net.ParseIP("192.168.1.1")
		if !params.AllowedIP.Equal(expected) {
			t.Errorf("AllowedIP = %v, want %v", params.AllowedIP, expected)
		}
	})

	t.Run("IPv6 address", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?allowed_ip=2001:0db8:85a3:0000:0000:8a2e:0370:7334", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.AllowedIP == nil {
			t.Error("AllowedIP should not be nil")
		}
	})

	t.Run("IP slice", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Add("ips", "192.168.1.1")
		q.Add("ips", "10.0.0.1")
		q.Add("ips", "172.16.0.1")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.IPs) != 3 {
			t.Fatalf("Expected 3 IPs, got %d", len(params.IPs))
		}
	})

	t.Run("invalid IP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?allowed_ip=invalid-ip", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid IP")
		}
	})
}

// Test url.URL support
func TestBindQuery_URLType(t *testing.T) {
	type WebhookParams struct {
		CallbackURL url.URL  `query:"callback"`
		RedirectURL url.URL  `query:"redirect"`
		OptionalURL *url.URL `query:"optional"`
	}

	t.Run("valid URL", func(t *testing.T) {
		callback := "https://example.com/webhook"
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("callback", callback)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params WebhookParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.CallbackURL.String() != callback {
			t.Errorf("CallbackURL = %v, want %v", params.CallbackURL.String(), callback)
		}
	})

	t.Run("URL with query params", func(t *testing.T) {
		callback := "https://example.com/hook?token=abc&id=123"
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("callback", callback)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params WebhookParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.CallbackURL.Host != "example.com" {
			t.Errorf("Host = %v, want example.com", params.CallbackURL.Host)
		}
	})
}

// Test encoding.TextUnmarshaler interface
func TestBindQuery_TextUnmarshaler(t *testing.T) {
	type Request struct {
		ID       customUUID  `query:"id"`
		TraceID  customUUID  `query:"trace_id"`
		Optional *customUUID `query:"optional"`
	}

	t.Run("valid custom type", func(t *testing.T) {
		id := "550e8400-e29b-41d4-a716-446655440000"
		traceID := "660e8400-e29b-41d4-a716-446655440001"

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("id", id)
		q.Set("trace_id", traceID)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Request
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if string(params.ID) != id {
			t.Errorf("ID = %v, want %v", params.ID, id)
		}
		if string(params.TraceID) != traceID {
			t.Errorf("TraceID = %v, want %v", params.TraceID, traceID)
		}
	})

	t.Run("invalid custom type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?id=invalid-uuid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Request
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid UUID")
		}
		if !strings.Contains(err.Error(), "invalid UUID format") {
			t.Errorf("Expected UUID format error, got: %v", err)
		}
	})

	t.Run("pointer to custom type", func(t *testing.T) {
		optional := "770e8400-e29b-41d4-a716-446655440002"
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("id", "550e8400-e29b-41d4-a716-446655440000")
		q.Set("optional", optional)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Request
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Optional == nil {
			t.Fatal("Optional should not be nil")
		}
		if string(*params.Optional) != optional {
			t.Errorf("Optional = %v, want %v", *params.Optional, optional)
		}
	})
}

// Test embedded struct support
func TestBindQuery_EmbeddedStruct(t *testing.T) {
	type Pagination struct {
		Page     int `query:"page"`
		PageSize int `query:"page_size"`
	}

	type SearchRequest struct {
		Pagination        // Embedded struct
		Query      string `query:"q"`
		Sort       string `query:"sort"`
	}

	t.Run("embedded fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?q=golang&page=2&page_size=20&sort=name", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SearchRequest
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Query != "golang" {
			t.Errorf("Query = %v", params.Query)
		}
		if params.Page != 2 {
			t.Errorf("Page = %v (from embedded struct)", params.Page)
		}
		if params.PageSize != 20 {
			t.Errorf("PageSize = %v (from embedded struct)", params.PageSize)
		}
	})

	t.Run("pointer to embedded struct", func(t *testing.T) {
		type AdvancedSearch struct {
			*Pagination
			Query string `query:"q"`
		}

		req := httptest.NewRequest("GET", "/?q=test&page=3&page_size=30", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		params := AdvancedSearch{
			Pagination: &Pagination{}, // Must initialize pointer
		}
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Query != "test" {
			t.Errorf("Query = %v", params.Query)
		}
		if params.Page != 3 {
			t.Errorf("Page = %v", params.Page)
		}
	})
}

// Test combined advanced types
func TestBindQuery_CombinedAdvancedTypes(t *testing.T) {
	type AdvancedParams struct {
		// Time types
		StartDate time.Time     `query:"start"`
		Timeout   time.Duration `query:"timeout"`

		// Network types
		AllowedIP net.IP  `query:"allowed_ip"`
		ProxyURL  url.URL `query:"proxy"`

		// Slices of advanced types
		Dates     []time.Time     `query:"dates"`
		Durations []time.Duration `query:"durations"`

		// Regular types
		Name   string `query:"name"`
		Active bool   `query:"active"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	q := req.URL.Query()
	q.Set("start", "2024-01-15T10:00:00Z")
	q.Set("timeout", "30s")
	q.Set("allowed_ip", "192.168.1.1")
	q.Set("proxy", "http://proxy.example.com:8080")
	q.Add("dates", "2024-01-15")
	q.Add("dates", "2024-01-16")
	q.Add("durations", "5s")
	q.Add("durations", "10m")
	q.Set("name", "test")
	q.Set("active", "true")
	req.URL.RawQuery = q.Encode()
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params AdvancedParams
	if err := c.BindQuery(&params); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}

	// Validate time
	expectedTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
	if !params.StartDate.Equal(expectedTime) {
		t.Errorf("StartDate = %v", params.StartDate)
	}

	// Validate duration
	if params.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", params.Timeout)
	}

	// Validate IP
	expectedIP := net.ParseIP("192.168.1.1")
	if !params.AllowedIP.Equal(expectedIP) {
		t.Errorf("AllowedIP = %v", params.AllowedIP)
	}

	// Validate URL
	if params.ProxyURL.Host != "proxy.example.com:8080" {
		t.Errorf("ProxyURL.Host = %v", params.ProxyURL.Host)
	}

	// Validate slices
	if len(params.Dates) != 2 {
		t.Errorf("Dates length = %d", len(params.Dates))
	}
	if len(params.Durations) != 2 || params.Durations[0] != 5*time.Second {
		t.Errorf("Durations = %v", params.Durations)
	}
}

// Test parseTime function directly
func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2024-01-15T10:30:00Z", false},
		{"RFC3339 with timezone", "2024-01-15T10:30:00+02:00", false},
		{"Date only", "2024-01-15", false},
		{"DateTime", "2024-01-15 10:30:00", false},
		{"RFC1123", "Mon, 15 Jan 2024 10:30:00 MST", false},
		{"Empty string", "", true},
		{"Invalid format", "not-a-date", true},
		{"Invalid date", "2024-13-45", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// Test net.IPNet support
func TestBindQuery_IPNetType(t *testing.T) {
	type NetworkParams struct {
		Subnet        net.IPNet   `query:"subnet"`
		AllowedRanges []net.IPNet `query:"ranges"`
		OptionalCIDR  *net.IPNet  `query:"optional"`
	}

	t.Run("valid CIDR", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?subnet=192.168.1.0/24", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		_, expected, _ := net.ParseCIDR("192.168.1.0/24")
		if params.Subnet.String() != expected.String() {
			t.Errorf("Subnet = %v, want %v", params.Subnet.String(), expected.String())
		}
	})

	t.Run("IPv6 CIDR", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?subnet=2001:db8::/32", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Subnet.IP == nil {
			t.Error("Subnet IP should not be nil")
		}
	})

	t.Run("CIDR slice", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Add("ranges", "10.0.0.0/8")
		q.Add("ranges", "172.16.0.0/12")
		q.Add("ranges", "192.168.0.0/16")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.AllowedRanges) != 3 {
			t.Fatalf("Expected 3 ranges, got %d", len(params.AllowedRanges))
		}
	})

	t.Run("invalid CIDR", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?subnet=invalid-cidr", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params NetworkParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid CIDR")
		}
		if !strings.Contains(err.Error(), "invalid CIDR notation") {
			t.Errorf("Expected CIDR error, got: %v", err)
		}
	})
}

// Test regexp.Regexp support
func TestBindQuery_RegexpType(t *testing.T) {
	type PatternParams struct {
		Pattern       regexp.Regexp  `query:"pattern"`
		OptionalRegex *regexp.Regexp `query:"optional"`
	}

	t.Run("valid regexp", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("pattern", `^user-[0-9]+$`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params PatternParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Pattern.String() != `^user-[0-9]+$` {
			t.Errorf("Pattern = %v", params.Pattern.String())
		}

		// Test the regex works
		if !params.Pattern.MatchString("user-123") {
			t.Error("Pattern should match user-123")
		}
		if params.Pattern.MatchString("admin-123") {
			t.Error("Pattern should not match admin-123")
		}
	})

	t.Run("invalid regexp", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?pattern=[invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params PatternParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid regexp")
		}
		if !strings.Contains(err.Error(), "invalid regular expression") {
			t.Errorf("Expected regexp error, got: %v", err)
		}
	})
}

// Test bracket notation for maps
func TestBindQuery_BracketNotation(t *testing.T) {
	type MapParams struct {
		Metadata map[string]string `query:"metadata"`
		Scores   map[string]int    `query:"scores"`
	}

	t.Run("simple bracket notation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?metadata[name]=John&metadata[age]=30", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata == nil {
			t.Fatal("Metadata should not be nil")
		}
		if params.Metadata["name"] != "John" {
			t.Errorf("metadata[name] = %v, want John", params.Metadata["name"])
		}
		if params.Metadata["age"] != "30" {
			t.Errorf("metadata[age] = %v, want 30", params.Metadata["age"])
		}
	})

	t.Run("quoted keys with special characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		// Keys with dots and dashes need quotes
		q.Set(`metadata["user.name"]`, "John Doe")
		q.Set(`metadata['user-email']`, "john@example.com")
		q.Set(`metadata["org.id"]`, "12345")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata["user.name"] != "John Doe" {
			t.Errorf(`metadata["user.name"] = %v`, params.Metadata["user.name"])
		}
		if params.Metadata["user-email"] != "john@example.com" {
			t.Errorf(`metadata['user-email'] = %v`, params.Metadata["user-email"])
		}
		if params.Metadata["org.id"] != "12345" {
			t.Errorf(`metadata["org.id"] = %v`, params.Metadata["org.id"])
		}
	})

	t.Run("typed map with bracket notation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?scores[math]=95&scores[science]=88&scores[history]=92", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Scores["math"] != 95 {
			t.Errorf("scores[math] = %v, want 95", params.Scores["math"])
		}
		if params.Scores["science"] != 88 {
			t.Errorf("scores[science] = %v, want 88", params.Scores["science"])
		}
		if len(params.Scores) != 3 {
			t.Errorf("Expected 3 scores, got %d", len(params.Scores))
		}
	})

	t.Run("mixed dot and bracket notation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?metadata.key1=value1&metadata[key2]=value2&metadata.key3=value3", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.Metadata) != 3 {
			t.Fatalf("Expected 3 entries, got %d", len(params.Metadata))
		}
		if params.Metadata["key1"] != "value1" {
			t.Errorf("key1 = %v", params.Metadata["key1"])
		}
		if params.Metadata["key2"] != "value2" {
			t.Errorf("key2 = %v", params.Metadata["key2"])
		}
		if params.Metadata["key3"] != "value3" {
			t.Errorf("key3 = %v", params.Metadata["key3"])
		}
	})

	t.Run("invalid bracket - no closing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?metadata[unclosed=value", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for unclosed bracket")
		}
		if !strings.Contains(err.Error(), "invalid bracket notation") {
			t.Errorf("Expected bracket error, got: %v", err)
		}
	})

	t.Run("empty brackets rejected for maps", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?metadata[]=value", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		err := c.BindQuery(&params)
		// Empty brackets should fail for maps (array notation)
		if err == nil {
			t.Error("Expected error for empty brackets on map")
		}
	})

	t.Run("nested brackets rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?metadata[key1][key2]=value", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params MapParams
		err := c.BindQuery(&params)
		// Nested brackets should fail (array notation)
		if err == nil {
			t.Error("Expected error for nested brackets on map")
		}
	})
}

// Test map support with dot notation
func TestBindQuery_Maps(t *testing.T) {
	type FilterParams struct {
		Metadata map[string]string `query:"metadata"`
		Tags     map[string]string `query:"tags"`
		Settings map[string]any    `query:"settings"`
	}

	t.Run("string map with dot notation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("metadata.name", "John")
		q.Set("metadata.age", "30")
		q.Set("metadata.city", "NYC")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params FilterParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata == nil {
			t.Fatal("Metadata map should not be nil")
		}
		if params.Metadata["name"] != "John" {
			t.Errorf("metadata.name = %v, want John", params.Metadata["name"])
		}
		if params.Metadata["age"] != "30" {
			t.Errorf("metadata.age = %v, want 30", params.Metadata["age"])
		}
		if len(params.Metadata) != 3 {
			t.Errorf("Expected 3 metadata entries, got %d", len(params.Metadata))
		}
	})

	t.Run("map with interface{} values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("settings.debug", "true")
		q.Set("settings.port", "8080")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params FilterParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Settings == nil {
			t.Fatal("Settings map should not be nil")
		}
		if params.Settings["debug"] != "true" {
			t.Errorf("settings.debug = %v", params.Settings["debug"])
		}
	})

	t.Run("empty map", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params FilterParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		// Maps without values should remain nil (not cause error)
		if len(params.Metadata) > 0 {
			t.Error("Metadata should be empty")
		}
	})

	t.Run("typed map values", func(t *testing.T) {
		type TypedMapParams struct {
			Scores map[string]int     `query:"scores"`
			Rates  map[string]float64 `query:"rates"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("scores.math", "95")
		q.Set("scores.science", "88")
		q.Set("rates.usd", "1.0")
		q.Set("rates.eur", "0.85")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TypedMapParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Scores["math"] != 95 {
			t.Errorf("scores.math = %v, want 95", params.Scores["math"])
		}
		if params.Rates["eur"] < 0.84 || params.Rates["eur"] > 0.86 {
			t.Errorf("rates.eur = %v, want ~0.85", params.Rates["eur"])
		}
	})
}

// Test nested struct support with dot notation
func TestBindQuery_NestedStructs(t *testing.T) {
	type Address struct {
		Street  string `query:"street"`
		City    string `query:"city"`
		ZipCode string `query:"zip_code"`
	}

	type UserRequest struct {
		Name    string  `query:"name"`
		Email   string  `query:"email"`
		Address Address `query:"address"`
	}

	t.Run("nested struct with dot notation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("name", "John")
		q.Set("email", "john@example.com")
		q.Set("address.street", "123 Main St")
		q.Set("address.city", "NYC")
		q.Set("address.zip_code", "10001")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params UserRequest
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name != "John" {
			t.Errorf("Name = %v", params.Name)
		}
		if params.Address.Street != "123 Main St" {
			t.Errorf("Address.Street = %v", params.Address.Street)
		}
		if params.Address.City != "NYC" {
			t.Errorf("Address.City = %v", params.Address.City)
		}
		if params.Address.ZipCode != "10001" {
			t.Errorf("Address.ZipCode = %v", params.Address.ZipCode)
		}
	})

	t.Run("deeply nested structs", func(t *testing.T) {
		type Location struct {
			Lat float64 `query:"lat"`
			Lng float64 `query:"lng"`
		}

		type FullAddress struct {
			Street   string   `query:"street"`
			Location Location `query:"location"`
		}

		type ComplexRequest struct {
			Name    string      `query:"name"`
			Address FullAddress `query:"address"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("name", "Test")
		q.Set("address.street", "Main St")
		q.Set("address.location.lat", "40.7128")
		q.Set("address.location.lng", "-74.0060")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params ComplexRequest
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Address.Street != "Main St" {
			t.Errorf("Address.Street = %v", params.Address.Street)
		}
		if params.Address.Location.Lat < 40.71 || params.Address.Location.Lat > 40.72 {
			t.Errorf("Address.Location.Lat = %v", params.Address.Location.Lat)
		}
	})
}

// Test enum validation
func TestBindQuery_EnumValidation(t *testing.T) {
	type StatusParams struct {
		Status   string `query:"status" enum:"active,inactive,pending"`
		Role     string `query:"role" enum:"admin,user,guest"`
		Priority string `query:"priority" enum:"low,medium,high"`
	}

	t.Run("valid enum values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?status=active&role=admin&priority=high", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params StatusParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Status != "active" || params.Role != "admin" || params.Priority != "high" {
			t.Errorf("Enum values = %+v", params)
		}
	})

	t.Run("invalid enum value", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?status=invalid-status", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params StatusParams
		err := c.BindQuery(&params)
		if err == nil {
			t.Error("Expected error for invalid enum value")
		}
		if !strings.Contains(err.Error(), "not in allowed values") {
			t.Errorf("Expected enum error, got: %v", err)
		}
	})

	t.Run("empty value passes enum validation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?role=admin", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params StatusParams
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v (empty values should pass)", err)
		}

		if params.Role != "admin" {
			t.Errorf("Role = %v", params.Role)
		}
		if params.Status != "" {
			t.Errorf("Status should be empty")
		}
	})

	t.Run("enum with whitespace handling", func(t *testing.T) {
		type SpacedEnum struct {
			Value string `query:"value" enum:" option1 , option2 , option3 "`
		}

		req := httptest.NewRequest("GET", "/?value=option2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SpacedEnum
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery should handle whitespace in enum: %v", err)
		}
	})
}

// Test combined complex features
func TestBindQuery_AllComplexTypes(t *testing.T) {
	type ComplexParams struct {
		// Basic types
		Name string `query:"name"`
		Age  int    `query:"age"`

		// Time types
		StartDate time.Time     `query:"start"`
		Timeout   time.Duration `query:"timeout"`

		// Network types
		AllowedIP   net.IP    `query:"allowed_ip"`
		Subnet      net.IPNet `query:"subnet"`
		CallbackURL url.URL   `query:"callback"`

		// Regex
		Pattern regexp.Regexp `query:"pattern"`

		// Maps
		Metadata map[string]string `query:"metadata"`
		Settings map[string]any    `query:"settings"`

		// Nested struct
		Address struct {
			Street string `query:"street"`
			City   string `query:"city"`
		} `query:"address"`

		// Enum validation
		Status string `query:"status" enum:"active,inactive"`

		// Slices of complex types
		Dates []time.Time `query:"dates"`
		IPs   []net.IP    `query:"ips"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	q := req.URL.Query()

	// Basic
	q.Set("name", "John")
	q.Set("age", "30")

	// Time
	q.Set("start", "2024-01-15T10:00:00Z")
	q.Set("timeout", "30s")

	// Network
	q.Set("allowed_ip", "192.168.1.1")
	q.Set("subnet", "10.0.0.0/8")
	q.Set("callback", "https://example.com/hook")

	// Regex
	q.Set("pattern", `^\w+$`)

	// Maps
	q.Set("metadata.key1", "value1")
	q.Set("metadata.key2", "value2")
	q.Set("settings.debug", "true")

	// Nested struct
	q.Set("address.street", "Main St")
	q.Set("address.city", "NYC")

	// Enum
	q.Set("status", "active")

	// Slices
	q.Add("dates", "2024-01-15")
	q.Add("dates", "2024-01-16")
	q.Add("ips", "192.168.1.1")
	q.Add("ips", "10.0.0.1")

	req.URL.RawQuery = q.Encode()
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params ComplexParams
	if err := c.BindQuery(&params); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}

	// Validate all fields
	if params.Name != "John" {
		t.Errorf("Name = %v", params.Name)
	}
	if params.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", params.Timeout)
	}
	if params.Metadata["key1"] != "value1" {
		t.Errorf("metadata.key1 = %v", params.Metadata["key1"])
	}
	if params.Address.Street != "Main St" {
		t.Errorf("address.street = %v", params.Address.Street)
	}
	if params.Status != "active" {
		t.Errorf("Status = %v", params.Status)
	}
	if len(params.Dates) != 2 {
		t.Errorf("Dates length = %d", len(params.Dates))
	}
	if len(params.IPs) != 2 {
		t.Errorf("IPs length = %d", len(params.IPs))
	}
}

// Test default values
func TestBindQuery_DefaultValues(t *testing.T) {
	type ParamsWithDefaults struct {
		Page     int    `query:"page" default:"1"`
		PageSize int    `query:"page_size" default:"10"`
		Sort     string `query:"sort" default:"created_at"`
		Order    string `query:"order" default:"desc"`
		Active   bool   `query:"active" default:"true"`
		Limit    int    `query:"limit" default:"100"`
	}

	t.Run("all defaults applied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil) // No query params
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params ParamsWithDefaults
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Page != 1 {
			t.Errorf("Page = %v, want 1 (default)", params.Page)
		}
		if params.PageSize != 10 {
			t.Errorf("PageSize = %v, want 10 (default)", params.PageSize)
		}
		if params.Sort != "created_at" {
			t.Errorf("Sort = %v, want created_at (default)", params.Sort)
		}
		if params.Order != "desc" {
			t.Errorf("Order = %v, want desc (default)", params.Order)
		}
		if !params.Active {
			t.Errorf("Active = %v, want true (default)", params.Active)
		}
		if params.Limit != 100 {
			t.Errorf("Limit = %v, want 100 (default)", params.Limit)
		}
	})

	t.Run("user values override defaults", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?page=5&page_size=50&active=false", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params ParamsWithDefaults
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Page != 5 {
			t.Errorf("Page = %v, want 5 (user value)", params.Page)
		}
		if params.PageSize != 50 {
			t.Errorf("PageSize = %v, want 50 (user value)", params.PageSize)
		}
		if params.Active {
			t.Errorf("Active = %v, want false (user value)", params.Active)
		}
		// Fields without user values should use defaults
		if params.Sort != "created_at" {
			t.Errorf("Sort = %v, want created_at (default)", params.Sort)
		}
	})

	t.Run("partial user values with defaults", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?page=3", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params ParamsWithDefaults
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Page != 3 {
			t.Errorf("Page = %v, want 3 (user value)", params.Page)
		}
		if params.PageSize != 10 {
			t.Errorf("PageSize = %v, want 10 (default)", params.PageSize)
		}
	})

	t.Run("default time values", func(t *testing.T) {
		type TimeDefaults struct {
			Timeout time.Duration `query:"timeout" default:"30s"`
			Created time.Time     `query:"created" default:"2024-01-01T00:00:00Z"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TimeDefaults
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s (default)", params.Timeout)
		}
		if params.Created.IsZero() {
			t.Error("Created should have default value")
		}
	})
}

// Test WarmupBindingCache
func TestWarmupBindingCache(t *testing.T) {
	type TestStruct1 struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type TestStruct2 struct {
		Email  string `json:"email"`
		Active bool   `json:"active"`
	}

	// Clear cache before test
	typeCacheMu.Lock()
	typeCache = make(map[reflect.Type]map[string]*structInfo)
	typeCacheMu.Unlock()

	// Warmup cache
	WarmupBindingCache(TestStruct1{}, TestStruct2{})

	// Verify cache is populated
	typeCacheMu.RLock()
	defer typeCacheMu.RUnlock()

	t1Type := reflect.TypeOf(TestStruct1{})
	if _, ok := typeCache[t1Type]; !ok {
		t.Error("TestStruct1 should be in cache")
	}
	if _, ok := typeCache[t1Type]["query"]; !ok {
		t.Error("TestStruct1 query tag should be cached")
	}

	t2Type := reflect.TypeOf(TestStruct2{})
	if _, ok := typeCache[t2Type]; !ok {
		t.Error("TestStruct2 should be in cache")
	}
	if _, ok := typeCache[t2Type]["json"]; !ok {
		t.Error("TestStruct2 json tag should be cached")
	}
}

// Benchmark binding performance
func BenchmarkBindJSON(b *testing.B) {
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	user := User{Name: "John", Email: "john@example.com", Age: 30}
	body, _ := json.Marshal(user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var result User
		if err := c.BindJSON(&result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBindQuery(b *testing.B) {
	type Params struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
		Page int    `query:"page"`
	}

	req := httptest.NewRequest("GET", "/?name=test&age=25&page=1", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewContext(w, req)
		var params Params
		if err := c.BindQuery(&params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBindParams(b *testing.B) {
	type Params struct {
		ID     int    `params:"id"`
		Action string `params:"action"`
	}

	req := httptest.NewRequest("GET", "/users/123/edit", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewContext(w, req)
		c.paramCount = 2
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"
		c.paramKeys[1] = "action"
		c.paramValues[1] = "edit"

		var params Params
		if err := c.BindParams(&params); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark cache hit performance
func BenchmarkBindQuery_Cached(b *testing.B) {
	type Params struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?name=test&age=25", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Warm up cache
	var warmup Params
	c.BindQuery(&warmup)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var params Params
		if err := c.BindQuery(&params); err != nil {
			b.Fatal(err)
		}
	}
}

// Test complex real-world binding
func TestBindQuery_ComplexStruct(t *testing.T) {
	type ComplexParams struct {
		// Strings
		Query  string   `query:"q"`
		Filter string   `query:"filter"`
		Tags   []string `query:"tags"`

		// Numbers
		Page     int       `query:"page"`
		PageSize int       `query:"page_size"`
		Limit    int       `query:"limit"`
		IDs      []int     `query:"ids"`
		Scores   []float64 `query:"scores"`

		// Booleans
		Active   bool `query:"active"`
		Verified bool `query:"verified"`

		// Pointers
		OptionalName *string `query:"optional_name"`
		OptionalAge  *int    `query:"optional_age"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	// Build query parameters programmatically
	q := req.URL.Query()
	q.Set("q", "search")
	q.Set("filter", "all")
	q.Add("tags", "go")
	q.Add("tags", "rust")
	q.Set("page", "2")
	q.Set("page_size", "20")
	q.Set("limit", "100")
	q.Add("ids", "1")
	q.Add("ids", "2")
	q.Add("ids", "3")
	q.Add("scores", "1.5")
	q.Add("scores", "2.5")
	q.Set("active", "true")
	q.Set("verified", "false")
	q.Set("optional_name", "John")
	req.URL.RawQuery = q.Encode()

	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params ComplexParams
	if err := c.BindQuery(&params); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}

	// Validate all fields
	if params.Query != "search" {
		t.Errorf("Query = %v", params.Query)
	}
	if len(params.Tags) != 2 {
		t.Errorf("Tags length = %d", len(params.Tags))
	}
	if len(params.IDs) != 3 {
		t.Errorf("IDs length = %d", len(params.IDs))
	}
	if params.OptionalName == nil || *params.OptionalName != "John" {
		t.Error("OptionalName not set correctly")
	}
	if params.OptionalAge != nil {
		t.Error("OptionalAge should be nil")
	}
}
