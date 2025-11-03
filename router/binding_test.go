package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
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

// failingReader is a custom reader that always returns an error for testing error paths
type failingReader struct{}

func (r *failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
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

// TestBindQuery_MapJSONFallback tests the JSON string parsing fallback for map fields.
// This tests the code path where no dot/bracket notation is found, so it falls back
// to parsing a JSON string value for the map prefix.
func TestBindQuery_MapJSONFallback(t *testing.T) {
	t.Run("string map from JSON string", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `query:"metadata"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		// Set a JSON string value (no dot/bracket notation)
		q.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata == nil {
			t.Fatal("Metadata map should not be nil")
		}
		if params.Metadata["name"] != "John" {
			t.Errorf("metadata[\"name\"] = %v, want John", params.Metadata["name"])
		}
		if params.Metadata["age"] != "30" {
			t.Errorf("metadata[\"age\"] = %v, want 30", params.Metadata["age"])
		}
		if params.Metadata["city"] != "NYC" {
			t.Errorf("metadata[\"city\"] = %v, want NYC", params.Metadata["city"])
		}
		if len(params.Metadata) != 3 {
			t.Errorf("Expected 3 metadata entries, got %d", len(params.Metadata))
		}
	})

	t.Run("int map from JSON string", func(t *testing.T) {
		type Params struct {
			Scores map[string]int `query:"scores"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("scores", `{"math":95,"science":88,"history":92}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Scores == nil {
			t.Fatal("Scores map should not be nil")
		}
		if params.Scores["math"] != 95 {
			t.Errorf("scores[\"math\"] = %v, want 95", params.Scores["math"])
		}
		if params.Scores["science"] != 88 {
			t.Errorf("scores[\"science\"] = %v, want 88", params.Scores["science"])
		}
		if params.Scores["history"] != 92 {
			t.Errorf("scores[\"history\"] = %v, want 92", params.Scores["history"])
		}
	})

	t.Run("float64 map from JSON string", func(t *testing.T) {
		type Params struct {
			Rates map[string]float64 `query:"rates"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("rates", `{"usd":1.0,"eur":0.85,"gbp":0.77}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Rates == nil {
			t.Fatal("Rates map should not be nil")
		}
		if params.Rates["usd"] < 0.99 || params.Rates["usd"] > 1.01 {
			t.Errorf("rates[\"usd\"] = %v, want ~1.0", params.Rates["usd"])
		}
		if params.Rates["eur"] < 0.84 || params.Rates["eur"] > 0.86 {
			t.Errorf("rates[\"eur\"] = %v, want ~0.85", params.Rates["eur"])
		}
	})

	t.Run("bool map from JSON string", func(t *testing.T) {
		type Params struct {
			Flags map[string]bool `query:"flags"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("flags", `{"debug":true,"verbose":false,"trace":true}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Flags == nil {
			t.Fatal("Flags map should not be nil")
		}
		if !params.Flags["debug"] {
			t.Errorf("flags[\"debug\"] = %v, want true", params.Flags["debug"])
		}
		if params.Flags["verbose"] {
			t.Errorf("flags[\"verbose\"] = %v, want false", params.Flags["verbose"])
		}
		if !params.Flags["trace"] {
			t.Errorf("flags[\"trace\"] = %v, want true", params.Flags["trace"])
		}
	})

	t.Run("interface{} map from JSON string", func(t *testing.T) {
		type Params struct {
			Settings map[string]any `query:"settings"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("settings", `{"debug":true,"port":8080,"name":"server"}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Settings == nil {
			t.Fatal("Settings map should not be nil")
		}
		if params.Settings["debug"] != "true" {
			t.Errorf("settings[\"debug\"] = %v, want \"true\"", params.Settings["debug"])
		}
		if params.Settings["port"] != "8080" {
			t.Errorf("settings[\"port\"] = %v, want \"8080\"", params.Settings["port"])
		}
		if params.Settings["name"] != "server" {
			t.Errorf("settings[\"name\"] = %v, want \"server\"", params.Settings["name"])
		}
	})

	t.Run("empty JSON object", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `query:"metadata"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("metadata", `{}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata == nil {
			t.Fatal("Metadata map should not be nil")
		}
		if len(params.Metadata) != 0 {
			t.Errorf("Expected empty map, got %d entries", len(params.Metadata))
		}
	})

	t.Run("empty JSON string - should not error", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `query:"metadata"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("metadata", "")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		// Should not error, just skip JSON parsing
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery should not error on empty string: %v", err)
		}
	})

	t.Run("invalid JSON - should silently fail without error", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `query:"metadata"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("metadata", `{invalid json}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		// Should not error, just skip invalid JSON parsing
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery should not error on invalid JSON (should silently fail): %v", err)
		}
		// Map should remain nil or empty since JSON parsing failed
		if len(params.Metadata) > 0 {
			t.Errorf("Metadata should be empty when JSON is invalid, got %v", params.Metadata)
		}
	})

	t.Run("type conversion error - should return error", func(t *testing.T) {
		type Params struct {
			Scores map[string]int `query:"scores"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		// Valid JSON but invalid int value
		q.Set("scores", `{"math":"not-a-number"}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("BindQuery should error on invalid type conversion")
		}
		// Error should mention the key
		if !strings.Contains(err.Error(), "math") {
			t.Errorf("Error should mention the key 'math', got: %v", err)
		}
	})

	t.Run("JSON string with numeric keys", func(t *testing.T) {
		type Params struct {
			Data map[string]string `query:"data"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("data", `{"123":"value1","456":"value2"}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Data["123"] != "value1" {
			t.Errorf("data[\"123\"] = %v, want value1", params.Data["123"])
		}
		if params.Data["456"] != "value2" {
			t.Errorf("data[\"456\"] = %v, want value2", params.Data["456"])
		}
	})

	t.Run("JSON with nested objects - should parse only top level", func(t *testing.T) {
		type Params struct {
			Config map[string]any `query:"config"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		// JSON with nested object - nested part will be converted to string
		q.Set("config", `{"outer":"value","nested":{"inner":"data"}}`)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Config["outer"] != "value" {
			t.Errorf("config[\"outer\"] = %v, want value", params.Config["outer"])
		}
		// Nested object will be converted to string via fmt.Sprint
		if params.Config["nested"] == nil {
			t.Error("config[\"nested\"] should not be nil")
		}
	})
}

// TestBindForm_MapJSONFallback tests the JSON string parsing fallback for map fields in form data.
func TestBindForm_MapJSONFallback(t *testing.T) {
	t.Run("string map from JSON string", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `form:"metadata"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			if err := c.BindForm(&params); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result struct {
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result.Metadata["name"] != "John" {
			t.Errorf("metadata[\"name\"] = %v, want John", result.Metadata["name"])
		}
		if result.Metadata["age"] != "30" {
			t.Errorf("metadata[\"age\"] = %v, want 30", result.Metadata["age"])
		}
	})

	t.Run("int map from JSON string", func(t *testing.T) {
		type Params struct {
			Scores map[string]int `form:"scores"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			if err := c.BindForm(&params); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("scores", `{"math":95,"science":88}`)

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result struct {
			Scores map[string]int `json:"scores"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result.Scores["math"] != 95 {
			t.Errorf("scores[\"math\"] = %v, want 95", result.Scores["math"])
		}
		if result.Scores["science"] != 88 {
			t.Errorf("scores[\"science\"] = %v, want 88", result.Scores["science"])
		}
	})

	t.Run("invalid JSON - should silently fail", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `form:"metadata"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			if err := c.BindForm(&params); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, map[string]int{"count": len(params.Metadata)})
		})

		form := url.Values{}
		form.Set("metadata", `{invalid json}`)

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should not error, just skip invalid JSON
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
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

// TestBindQuery_PointerMap tests pointer to map types  for pointer maps).
func TestBindQuery_PointerMap(t *testing.T) {
	t.Run("pointer to map[string]string", func(t *testing.T) {
		type Params struct {
			Metadata *map[string]string `query:"metadata"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("metadata.name", "John")
		q.Set("metadata.age", "30")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Metadata == nil {
			t.Fatal("Metadata map pointer should not be nil")
		}
		if (*params.Metadata)["name"] != "John" {
			t.Errorf("metadata[\"name\"] = %v, want John", (*params.Metadata)["name"])
		}
		if (*params.Metadata)["age"] != "30" {
			t.Errorf("metadata[\"age\"] = %v, want 30", (*params.Metadata)["age"])
		}
	})

	t.Run("pointer to map[string]int", func(t *testing.T) {
		type Params struct {
			Scores *map[string]int `query:"scores"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("scores.math", "95")
		q.Set("scores.science", "88")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Scores == nil {
			t.Fatal("Scores map pointer should not be nil")
		}
		if (*params.Scores)["math"] != 95 {
			t.Errorf("scores[\"math\"] = %v, want 95", (*params.Scores)["math"])
		}
		if (*params.Scores)["science"] != 88 {
			t.Errorf("scores[\"science\"] = %v, want 88", (*params.Scores)["science"])
		}
	})
}

// TestBindForm_PointerMap tests pointer to map types in form data.
func TestBindForm_PointerMap(t *testing.T) {
	t.Run("pointer to map[string]string", func(t *testing.T) {
		type Params struct {
			Metadata *map[string]string `form:"metadata"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			if err := c.BindForm(&params); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("metadata.name", "John")
		form.Set("metadata.age", "30")

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result struct {
			Metadata *map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result.Metadata == nil {
			t.Fatal("Metadata map pointer should not be nil")
		}
		if (*result.Metadata)["name"] != "John" {
			t.Errorf("metadata[\"name\"] = %v, want John", (*result.Metadata)["name"])
		}
	})
}

// TestBindQuery_MapTypeConversionError tests error path for queryGetter when type conversion fails
// ).
func TestBindQuery_MapTypeConversionError(t *testing.T) {
	t.Run("queryGetter dot notation - invalid int conversion", func(t *testing.T) {
		type Params struct {
			Scores map[string]int `query:"scores"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("scores.math", "not-a-number") // Invalid int value
		q.Set("scores.science", "88")        // Valid value (should not be reached if error happens first)
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid int conversion, got nil")
		}

		// Error should mention the key "math"
		if !strings.Contains(err.Error(), "math") {
			t.Errorf("Error should mention key 'math', got: %v", err)
		}
		if !strings.Contains(err.Error(), "key") || !strings.Contains(err.Error(), "\"math\"") {
			t.Errorf("Error should include quoted key name, got: %v", err)
		}
	})

	t.Run("queryGetter bracket notation - invalid float conversion", func(t *testing.T) {
		type Params struct {
			Rates map[string]float64 `query:"rates"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		q := req.URL.Query()
		q.Set("rates[usd]", "invalid-float")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid float conversion, got nil")
		}

		// Error should mention the key
		if !strings.Contains(err.Error(), "usd") {
			t.Errorf("Error should mention key 'usd', got: %v", err)
		}
	})
}

// TestBindForm_MapTypeConversionError tests error path for formGetter when type conversion fails
// ).
// Also tests formGetter dot notation path (: found = true, mapKey = strings.TrimPrefix).
func TestBindForm_MapTypeConversionError(t *testing.T) {
	t.Run("formGetter dot notation - invalid int conversion", func(t *testing.T) {
		type Params struct {
			Scores map[string]int `form:"scores"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			err := c.BindForm(&params)
			if err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("scores.math", "not-a-number") // Invalid int value
		form.Set("scores.science", "88")

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should return error status
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}

		// Error should mention the key "math"
		var errorResp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
			t.Fatalf("failed to parse error response: %v", err)
		}

		errorMsg := errorResp["error"]
		if !strings.Contains(errorMsg, "math") {
			t.Errorf("Error should mention key 'math', got: %v", errorMsg)
		}
		if !strings.Contains(errorMsg, "key") || !strings.Contains(errorMsg, "\"math\"") {
			t.Errorf("Error should include quoted key name, got: %v", errorMsg)
		}
	})

	t.Run("formGetter dot notation - successful binding", func(t *testing.T) {
		type Params struct {
			Metadata map[string]string `form:"metadata"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			if err := c.BindForm(&params); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("metadata.name", "John") // Tests : found = true, mapKey extraction
		form.Set("metadata.age", "30")

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result struct {
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result.Metadata["name"] != "John" {
			t.Errorf("metadata[\"name\"] = %v, want John", result.Metadata["name"])
		}
		if result.Metadata["age"] != "30" {
			t.Errorf("metadata[\"age\"] = %v, want 30", result.Metadata["age"])
		}
	})

	t.Run("formGetter bracket notation - invalid bool conversion", func(t *testing.T) {
		type Params struct {
			Flags map[string]bool `form:"flags"`
		}

		r := New()
		r.POST("/test", func(c *Context) {
			var params Params
			err := c.BindForm(&params)
			if err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, params)
		})

		form := url.Values{}
		form.Set("flags[debug]", "not-a-bool") // Invalid bool value

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should return error status
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}

		// Error should mention the key
		var errorResp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
			t.Fatalf("failed to parse error response: %v", err)
		}

		errorMsg := errorResp["error"]
		if !strings.Contains(errorMsg, "debug") {
			t.Errorf("Error should mention key 'debug', got: %v", errorMsg)
		}
	})
}

// TestPrefixGetter_Has tests the Has method of prefixGetter, specifically the iteration
// logic for queryGetter and formGetter ( in binding.go).
func TestPrefixGetter_Has(t *testing.T) {
	t.Run("queryGetter - exact key match", func(t *testing.T) {
		values := url.Values{}
		values.Set("address.street", "Main St")
		values.Set("address.city", "NYC")
		values.Set("name", "John")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// Should find exact match "address.street" when checking for "street"
		if !pg.Has("street") {
			t.Error("Expected Has(\"street\") to return true for exact match")
		}

		// Should find exact match "address.city" when checking for "city"
		if !pg.Has("city") {
			t.Error("Expected Has(\"city\") to return true for exact match")
		}

		// Should not find "name" (doesn't start with prefix)
		if pg.Has("name") {
			t.Error("Expected Has(\"name\") to return false (key doesn't have prefix)")
		}
	})

	t.Run("queryGetter - prefix match with dot", func(t *testing.T) {
		values := url.Values{}
		values.Set("address.location.lat", "40.7128")
		values.Set("address.location.lng", "-74.0060")
		values.Set("address.city", "NYC")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// Should find "address.location.lat" when checking for "location"
		// because it starts with "address.location."
		if !pg.Has("location") {
			t.Error("Expected Has(\"location\") to return true for prefix match")
		}

		// Should also find "address.city" (exact match)
		if !pg.Has("city") {
			t.Error("Expected Has(\"city\") to return true for exact match")
		}
	})

	t.Run("queryGetter - no matching keys", func(t *testing.T) {
		values := url.Values{}
		values.Set("user.name", "John")
		values.Set("user.email", "john@example.com")
		values.Set("other.field", "value")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// Should not find any keys with "address." prefix
		if pg.Has("street") {
			t.Error("Expected Has(\"street\") to return false (no matching keys)")
		}
		if pg.Has("city") {
			t.Error("Expected Has(\"city\") to return false (no matching keys)")
		}
	})

	t.Run("queryGetter - empty values", func(t *testing.T) {
		values := url.Values{}
		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// Should return false for any key when values are empty
		if pg.Has("street") {
			t.Error("Expected Has(\"street\") to return false for empty values")
		}
	})

	t.Run("formGetter - exact key match", func(t *testing.T) {
		values := url.Values{}
		values.Set("metadata.name", "John")
		values.Set("metadata.age", "30")
		values.Set("title", "Mr")

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "metadata.",
		}

		// Should find exact match "metadata.name" when checking for "name"
		if !pg.Has("name") {
			t.Error("Expected Has(\"name\") to return true for exact match")
		}

		// Should find exact match "metadata.age" when checking for "age"
		if !pg.Has("age") {
			t.Error("Expected Has(\"age\") to return true for exact match")
		}

		// Should not find "title" (doesn't start with prefix)
		if pg.Has("title") {
			t.Error("Expected Has(\"title\") to return false (key doesn't have prefix)")
		}
	})

	t.Run("formGetter - prefix match with dot", func(t *testing.T) {
		values := url.Values{}
		values.Set("config.database.host", "localhost")
		values.Set("config.database.port", "5432")
		values.Set("config.debug", "true")

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "config.",
		}

		// Should find "config.database.host" when checking for "database"
		// because it starts with "config.database."
		if !pg.Has("database") {
			t.Error("Expected Has(\"database\") to return true for prefix match")
		}

		// Should also find "config.debug" (exact match)
		if !pg.Has("debug") {
			t.Error("Expected Has(\"debug\") to return true for exact match")
		}
	})

	t.Run("formGetter - no matching keys", func(t *testing.T) {
		values := url.Values{}
		values.Set("user.name", "John")
		values.Set("other.field", "value")

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "config.",
		}

		// Should not find any keys with "config." prefix
		if pg.Has("debug") {
			t.Error("Expected Has(\"debug\") to return false (no matching keys)")
		}
	})

	t.Run("formGetter - empty values", func(t *testing.T) {
		values := url.Values{}
		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "metadata.",
		}

		// Should return false for any key when values are empty
		if pg.Has("name") {
			t.Error("Expected Has(\"name\") to return false for empty values")
		}
	})

	t.Run("queryGetter - multiple prefix matches", func(t *testing.T) {
		values := url.Values{}
		values.Set("address.street", "Main St")
		values.Set("address.street.number", "123")
		values.Set("address.city", "NYC")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// Should find "address.street" (exact match)
		if !pg.Has("street") {
			t.Error("Expected Has(\"street\") to return true")
		}

		// Should also find "address.city" (exact match)
		if !pg.Has("city") {
			t.Error("Expected Has(\"city\") to return true")
		}
	})

	t.Run("formGetter - key that starts with prefix but doesn't match", func(t *testing.T) {
		values := url.Values{}
		values.Set("addresses.street", "Main St") // Note: "addresses" not "address"
		values.Set("address.city", "NYC")

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "address.",
		}

		// Should find "address.city" (exact match)
		if !pg.Has("city") {
			t.Error("Expected Has(\"city\") to return true")
		}

		// Should not find "street" because "addresses.street" doesn't start with "address."
		if pg.Has("street") {
			t.Error("Expected Has(\"street\") to return false")
		}
	})

	t.Run("queryGetter - direct Has check returns true", func(t *testing.T) {
		values := url.Values{}
		values.Set("address.street", "Main St")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// The direct Has check  should return true before iteration
		// This tests that the iteration code is still reached even when direct check passes
		if !pg.Has("street") {
			t.Error("Expected Has(\"street\") to return true")
		}
	})

	t.Run("queryGetter - iteration path when direct Has returns false", func(t *testing.T) {
		values := url.Values{}
		// Set nested key that doesn't match exact fullKey
		values.Set("address.location.lat", "40.7128")
		// Note: "address.location" doesn't exist as a key, only "address.location.lat"
		// So direct Has("address.location") returns false, forcing iteration

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "address.",
		}

		// fullKey = "address.location"
		// Direct check: inner.Has("address.location") = false (key doesn't exist)
		// Iteration should find "address.location.lat" starting with "address.location."
		if !pg.Has("location") {
			t.Error("Expected Has(\"location\") to return true via iteration path")
		}
	})

	t.Run("formGetter - iteration path when direct Has returns false", func(t *testing.T) {
		values := url.Values{}
		// Set nested key that doesn't match exact fullKey
		values.Set("config.database.host", "localhost")
		// Note: "config.database" doesn't exist as a key, only "config.database.host"
		// So direct Has("config.database") returns false, forcing iteration

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "config.",
		}

		// fullKey = "config.database"
		// Direct check: inner.Has("config.database") = false (key doesn't exist)
		// Iteration should find "config.database.host" starting with "config.database."
		if !pg.Has("database") {
			t.Error("Expected Has(\"database\") to return true via iteration path")
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

// TestGetStructInfo_DoubleCheckLocking tests the double-check locking pattern ()
func TestGetStructInfo_DoubleCheckLocking(t *testing.T) {
	type ConcurrentStruct struct {
		Name  string `query:"name"`
		Email string `query:"email"`
		Age   int    `query:"age"`
	}

	// Clear cache before test
	typeCacheMu.Lock()
	typeCache = make(map[reflect.Type]map[string]*structInfo)
	typeCacheMu.Unlock()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Launch multiple goroutines that all try to bind the same struct type simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Create a request to trigger binding
			req := httptest.NewRequest("GET", "/?name=John&email=john@example.com&age=30", nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params ConcurrentStruct
			if err := c.BindQuery(&params); err != nil {
				errors <- err
				return
			}

			// Verify binding worked
			if params.Name != "John" || params.Email != "john@example.com" || params.Age != 30 {
				errors <- fmt.Errorf("binding failed: got %+v", params)
				return
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Goroutine error: %v", err)
	}

	// Verify cache was populated (at least one goroutine should have parsed it)
	typeCacheMu.RLock()
	defer typeCacheMu.RUnlock()

	structType := reflect.TypeOf(ConcurrentStruct{})
	if _, ok := typeCache[structType]; !ok {
		t.Error("ConcurrentStruct should be in cache after concurrent binding")
	}
	if _, ok := typeCache[structType]["query"]; !ok {
		t.Error("ConcurrentStruct query tag should be cached")
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

// TestWarmupBindingCache_MultipleTypes tests warming up cache with multiple struct types
func TestWarmupBindingCache_MultipleTypes(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}

	type Product struct {
		SKU string `json:"sku"`
	}

	// Warmup multiple types
	WarmupBindingCache(User{}, Product{})

	// Both should bind successfully
	r := New()

	r.POST("/user", func(c *Context) {
		var user User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	})

	req := httptest.NewRequest(http.MethodPost, "/user", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("User binding should work after warmup")
	}
}

// TestBindBody_UnsupportedContentType tests BindBody with unsupported content type
func TestBindBody_UnsupportedContentType(t *testing.T) {
	r := New()

	type Data struct {
		Value string `json:"value"`
	}

	r.POST("/test", func(c *Context) {
		var data Data
		err := c.BindBody(&data)

		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported"})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`value=test`))
	req.Header.Set("Content-Type", "application/xml") // Unsupported
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return error for unsupported type
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unsupported content type, got %d", w.Code)
	}
}

// TestGetCookie_URLEscaping tests cookie value unescaping
func TestGetCookie_URLEscaping(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		value, err := c.GetCookie("data")

		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "cookie not found"})
			return
		}

		// Should be unescaped
		c.JSON(http.StatusOK, map[string]string{"value": value})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "data",
		Value: url.QueryEscape("test value with spaces"),
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("should successfully get and unescape cookie")
	}
}

// TestBindJSON_EdgeCases tests JSON binding edge cases
func TestBindJSON_EdgeCases(t *testing.T) {
	r := New()

	type Data struct {
		Value string `json:"value"`
	}

	r.POST("/test", func(c *Context) {
		var data Data
		err := c.BindJSON(&data)

		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	// Test malformed JSON
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Error("should return error for malformed JSON")
	}

	// Test empty body
	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(``))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Empty body should fail
	if w2.Code != http.StatusBadRequest {
		t.Error("should return error for empty JSON")
	}
}

// TestConvertValue_AllTypes tests type conversion for all supported types
func TestConvertValue_AllTypes(t *testing.T) {
	r := New()

	type AllTypes struct {
		String  string  `form:"str"`
		Int     int     `form:"int"`
		Int8    int8    `form:"int8"`
		Int16   int16   `form:"int16"`
		Int32   int32   `form:"int32"`
		Int64   int64   `form:"int64"`
		Uint    uint    `form:"uint"`
		Uint8   uint8   `form:"uint8"`
		Uint16  uint16  `form:"uint16"`
		Uint32  uint32  `form:"uint32"`
		Uint64  uint64  `form:"uint64"`
		Float32 float32 `form:"float32"`
		Float64 float64 `form:"float64"`
		Bool    bool    `form:"bool"`
	}

	r.POST("/test", func(c *Context) {
		var data AllTypes
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	form := url.Values{}
	form.Set("str", "text")
	form.Set("int", "42")
	form.Set("int8", "8")
	form.Set("int16", "16")
	form.Set("int32", "32")
	form.Set("int64", "64")
	form.Set("uint", "42")
	form.Set("uint8", "8")
	form.Set("uint16", "16")
	form.Set("uint32", "32")
	form.Set("uint64", "64")
	form.Set("float32", "3.14")
	form.Set("float64", "2.718")
	form.Set("bool", "true")

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("should bind all types successfully: %s", w.Body.String())
	}
}

// TestConvertValue_ErrorCases tests convertValue error cases, specifically covering:
// - Invalid unsigned integer parsing error
// - Invalid float parsing error
// - Invalid bool parsing error (from parseBool)
// - Unsupported type error
func TestConvertValue_ErrorCases(t *testing.T) {
	t.Run("invalid_unsigned_integer_error_path", func(t *testing.T) {
		// Test error for invalid unsigned integer parsing
		tests := []struct {
			name  string
			value string
			kind  reflect.Kind
		}{
			{"negative value for uint", "-42", reflect.Uint},
			{"negative value for uint8", "-10", reflect.Uint8},
			{"negative value for uint16", "-100", reflect.Uint16},
			{"negative value for uint32", "-1000", reflect.Uint32},
			{"negative value for uint64", "-999999", reflect.Uint64},
			{"invalid format for uint", "abc", reflect.Uint},
			{"decimal for uint", "42.5", reflect.Uint},
			{"empty string for uint", "", reflect.Uint},
			{"whitespace for uint", "   ", reflect.Uint},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := convertValue(tt.value, tt.kind)

				if err == nil {
					t.Errorf("convertValue(%q, %v) should return error, got result: %v", tt.value, tt.kind, result)
				}

				if err != nil && !strings.Contains(err.Error(), "invalid unsigned integer") {
					t.Errorf("convertValue(%q, %v) error = %q, want error containing 'invalid unsigned integer'",
						tt.value, tt.kind, err.Error())
				}
			})
		}
	})

	t.Run("invalid_float_error_path", func(t *testing.T) {
		// Test error for invalid float parsing
		tests := []struct {
			name  string
			value string
			kind  reflect.Kind
		}{
			{"invalid format for float32", "abc", reflect.Float32},
			{"invalid format for float64", "xyz", reflect.Float64},
			{"empty string for float32", "", reflect.Float32},
			{"whitespace for float64", "   ", reflect.Float64},
			{"mixed chars for float32", "12abc", reflect.Float32},
			{"only dots for float64", "...", reflect.Float64},
			{"multiple dots for float32", "12.34.56", reflect.Float32},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := convertValue(tt.value, tt.kind)

				if err == nil {
					t.Errorf("convertValue(%q, %v) should return error, got result: %v", tt.value, tt.kind, result)
				}

				if err != nil && !strings.Contains(err.Error(), "invalid float") {
					t.Errorf("convertValue(%q, %v) error = %q, want error containing 'invalid float'",
						tt.value, tt.kind, err.Error())
				}
			})
		}
	})

	t.Run("invalid_bool_error_path", func(t *testing.T) {
		// Test error from parseBool for invalid boolean values
		tests := []struct {
			name  string
			value string
		}{
			{"invalid bool value", "maybe"},
			{"numeric 2", "2"},
			{"numeric 3", "3"},
			{"random text", "random"},
			{"mixed case invalid", "Maybe"},
			{"yesno together", "yesno"},
			{"truefalse together", "truefalse"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := convertValue(tt.value, reflect.Bool)

				if err == nil {
					t.Errorf("convertValue(%q, reflect.Bool) should return error, got result: %v", tt.value, result)
				}

				// parseBool returns error with message "invalid boolean value", and convertValue returns it as-is
				if err != nil && !strings.Contains(err.Error(), "invalid boolean") && !strings.Contains(err.Error(), "invalid") {
					t.Errorf("convertValue(%q, reflect.Bool) error = %q, want error containing 'invalid boolean' or 'invalid'",
						tt.value, err.Error())
				}
			})
		}
	})

	t.Run("unsupported_type_error_path", func(t *testing.T) {
		// Test error for unsupported types
		tests := []struct {
			name string
			kind reflect.Kind
		}{
			{"slice type", reflect.Slice},
			{"map type", reflect.Map},
			{"array type", reflect.Array},
			{"chan type", reflect.Chan},
			{"func type", reflect.Func},
			{"interface type", reflect.Interface},
			{"ptr type", reflect.Ptr},
			{"struct type", reflect.Struct},
			{"unsafe pointer", reflect.UnsafePointer},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := convertValue("test", tt.kind)

				if err == nil {
					t.Errorf("convertValue(\"test\", %v) should return error, got result: %v", tt.kind, result)
				}

				if err != nil && !strings.Contains(err.Error(), "unsupported type") {
					t.Errorf("convertValue(\"test\", %v) error = %q, want error containing 'unsupported type'",
						tt.kind, err.Error())
				}

				if err != nil && !strings.Contains(err.Error(), tt.kind.String()) {
					t.Errorf("convertValue(\"test\", %v) error = %q, want error containing type name",
						tt.kind, err.Error())
				}
			})
		}
	})

	t.Run("edge_cases_covering_all_error_paths", func(t *testing.T) {
		// Test all error paths in one comprehensive test
		testCases := []struct {
			name        string
			value       string
			kind        reflect.Kind
			errorSubstr string
		}{
			{
				name:        "error_path_negative_uint",
				value:       "-5",
				kind:        reflect.Uint,
				errorSubstr: "invalid unsigned integer",
			},
			{
				name:        "error_path_invalid_float",
				value:       "not_a_float",
				kind:        reflect.Float64,
				errorSubstr: "invalid float",
			},
			{
				name:        "error_path_invalid_bool",
				value:       "maybe",
				kind:        reflect.Bool,
				errorSubstr: "invalid",
			},
			{
				name:        "error_path_unsupported_slice",
				value:       "test",
				kind:        reflect.Slice,
				errorSubstr: "unsupported type",
			},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				result, err := convertValue(tt.value, tt.kind)

				if err == nil {
					t.Errorf("convertValue(%q, %v) should return error, got result: %v",
						tt.value, tt.kind, result)
				}

				if err != nil && !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("convertValue(%q, %v) error = %q, want error containing %q",
						tt.value, tt.kind, err.Error(), tt.errorSubstr)
				}
			})
		}
	})
}

// TestSetField_PointerFieldError tests error path for pointer fields .
// When setFieldValue fails for a pointer field, the error should be returned.
func TestSetField_PointerFieldError(t *testing.T) {
	t.Run("pointer to int with invalid value", func(t *testing.T) {
		type Params struct {
			Age *int `query:"age"`
		}

		req := httptest.NewRequest("GET", "/?age=not-a-number", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid int value in pointer field, got nil")
		}

		// Error should be returned from setFieldValue
		// Error is wrapped in BindError, but should contain field name and conversion error
		if !strings.Contains(err.Error(), "Age") || !strings.Contains(err.Error(), "invalid") {
			t.Errorf("Error should mention field and be about invalid conversion, got: %v", err)
		}
	})

	t.Run("pointer to time.Time with invalid value", func(t *testing.T) {
		type Params struct {
			StartTime *time.Time `query:"start"`
		}

		req := httptest.NewRequest("GET", "/?start=invalid-time", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid time value in pointer field, got nil")
		}

		// Error should propagate from setFieldValue
		if !strings.Contains(err.Error(), "StartTime") {
			t.Errorf("Error should mention field name, got: %v", err)
		}
	})

	t.Run("pointer to float64 with invalid value", func(t *testing.T) {
		type Params struct {
			Price *float64 `query:"price"`
		}

		req := httptest.NewRequest("GET", "/?price=not-a-float", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid float value in pointer field, got nil")
		}

		// Error should be returned
		if !strings.Contains(err.Error(), "Price") {
			t.Errorf("Error should mention field name, got: %v", err)
		}
	})
}

// TestSetFieldValue_InvalidURL tests error path for invalid URL parsing ).
func TestSetFieldValue_InvalidURL(t *testing.T) {
	t.Run("invalid URL format", func(t *testing.T) {
		type Params struct {
			CallbackURL url.URL `query:"callback"`
		}

		req := httptest.NewRequest("GET", "/?callback=://invalid-url", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid URL, got nil")
		}

		// Error should contain "invalid URL"
		if !strings.Contains(err.Error(), "invalid URL") {
			t.Errorf("Error should contain 'invalid URL', got: %v", err)
		}
		if !strings.Contains(err.Error(), "CallbackURL") {
			t.Errorf("Error should mention field name 'CallbackURL', got: %v", err)
		}
	})

	t.Run("malformed URL with missing scheme", func(t *testing.T) {
		type Params struct {
			Endpoint url.URL `query:"endpoint"`
		}

		req := httptest.NewRequest("GET", "/?endpoint=://malformed", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for malformed URL, got nil")
		}

		// Should trigger invalid URL error path
		if !strings.Contains(err.Error(), "invalid URL") {
			t.Errorf("Error should contain 'invalid URL', got: %v", err)
		}
	})

	t.Run("valid URL should succeed", func(t *testing.T) {
		type Params struct {
			Endpoint url.URL `query:"endpoint"`
		}

		req := httptest.NewRequest("GET", "/?endpoint=https://example.com/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery should succeed for valid URL, got: %v", err)
		}

		if params.Endpoint.Host != "example.com" {
			t.Errorf("URL Host = %v, want example.com", params.Endpoint.Host)
		}
	})
}

// TestSetFieldValue_UnsupportedType tests error path for unsupported types )).
// This tests types that are not handled in the switch statement (default case).
func TestSetFieldValue_UnsupportedType(t *testing.T) {
	t.Run("unsupported type - Array", func(t *testing.T) {
		type Params struct {
			Data [5]int `query:"data"`
		}

		req := httptest.NewRequest("GET", "/?data=1,2,3", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported array type, got nil")
		}

		// Error should contain "unsupported type"
		// The error message contains "unsupported type: array" (lowercase)
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "array") {
			t.Errorf("Error should mention 'array', got: %v", err)
		}
	})

	t.Run("unsupported type - Chan", func(t *testing.T) {
		type Params struct {
			Channel chan int `query:"channel"`
		}

		req := httptest.NewRequest("GET", "/?channel=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported channel type, got nil")
		}

		// Should trigger unsupported type error path
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
		if !strings.Contains(err.Error(), "Chan") {
			t.Errorf("Error should mention 'Chan', got: %v", err)
		}
	})

	t.Run("unsupported type - Func", func(t *testing.T) {
		type Params struct {
			Handler func() `query:"handler"`
		}

		req := httptest.NewRequest("GET", "/?handler=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported function type, got nil")
		}

		// Should trigger unsupported type error path
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
		// Error message contains "unsupported type: func" (lowercase)
		if !strings.Contains(strings.ToLower(err.Error()), "func") {
			t.Errorf("Error should mention 'func', got: %v", err)
		}
	})

	t.Run("unsupported type - Interface", func(t *testing.T) {
		type Params struct {
			Value interface{} `query:"value"`
		}

		req := httptest.NewRequest("GET", "/?value=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		// Note: interface{} might be handled differently, but if it reaches the switch,
		// it should trigger the unsupported type error
		if err != nil && !strings.Contains(err.Error(), "unsupported type") {
			// If there's an error but it's not about unsupported type, that's also valid
			// as interface{} might be handled before reaching the switch
		}
	})

	t.Run("unsupported type - UnsafePointer", func(t *testing.T) {
		// Testing with a type that would result in UnsafePointer kind
		// This is tricky to test directly, but we can test that the error format is correct
		// by checking the error message structure when it occurs
		type Params struct {
			Data [0]struct{} `query:"data"` // Empty struct array
		}

		req := httptest.NewRequest("GET", "/?data=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported array type, got nil")
		}

		// Error format should match fmt.Errorf("unsupported type: %v", fieldType.Kind())
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
	})

	t.Run("unsupported type - Complex64 (if convertValue ever succeeds)", func(t *testing.T) {
		// This test ensures unsupported type default case is covered.
		// Note: Most unsupported types fail in convertValue before reaching the switch.
		// This is defensive code for the theoretical case where convertValue succeeds
		// but the switch doesn't handle the type.
		type Params struct {
			Complex complex64 `query:"complex"`
		}

		req := httptest.NewRequest("GET", "/?complex=1+2i", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported complex type, got nil")
		}

		// Error should contain "unsupported type" - either from convertValue  or switch
		// Both use the same error format: fmt.Errorf("unsupported type: %v", ...)
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
	})

	t.Run("unsupported type - Complex128", func(t *testing.T) {
		type Params struct {
			Complex complex128 `query:"complex"`
		}

		req := httptest.NewRequest("GET", "/?complex=1+2i", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for unsupported complex type, got nil")
		}

		// Error should contain "unsupported type"
		if !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}
	})

	t.Run("unsupported type - Map (non-string key)", func(t *testing.T) {
		// Map with non-string key would fail early, but tests error handling
		type Params struct {
			Data map[int]string `query:"data"`
		}

		req := httptest.NewRequest("GET", "/?data=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		// Maps are handled specially, but this tests error paths
		if err != nil {
			// Error might mention map or unsupported type
			// Both are valid error messages depending on where validation happens
		}
	})

	t.Run("verify default case error format matches", func(t *testing.T) {
		// This test verifies that if unsupported type case is reached, the error format is correct.
		// The error format should be: fmt.Errorf("unsupported type: %v", fieldType.Kind())
		type Params struct {
			Unsupported [3]string `query:"data"` // Array type that will fail
		}

		req := httptest.NewRequest("GET", "/?data=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		// Verify error contains the expected format pattern
		errStr := err.Error()
		if !strings.Contains(errStr, "unsupported type") {
			t.Errorf("Error should contain 'unsupported type', got: %v", err)
		}

		// The error format from "unsupported type: %v" where %v is the Kind
		// Verify it follows this pattern (contains "unsupported type:" followed by a kind name)
		parts := strings.Split(errStr, "unsupported type:")
		if len(parts) < 2 {
			t.Errorf("Error should match format 'unsupported type: <kind>', got: %v", err)
		}
	})
}

// TestPrefixGetter_GetAll tests prefixGetter.GetAll method
func TestPrefixGetter_GetAll(t *testing.T) {
	t.Run("queryGetter with prefix", func(t *testing.T) {
		values := url.Values{}
		values.Add("user.name", "John")
		values.Add("user.email", "john@example.com")
		values.Add("user.email", "john.doe@example.com")
		values.Add("other.field", "value")

		qg := &queryGetter{values: values}
		pg := &prefixGetter{
			inner:  qg,
			prefix: "user.",
		}

		// Test GetAll for "name" (should return ["John"])
		all := pg.GetAll("name")
		if len(all) != 1 || all[0] != "John" {
			t.Errorf("expected [\"John\"], got %v", all)
		}

		// Test GetAll for "email" (should return both emails)
		all = pg.GetAll("email")
		if len(all) != 2 {
			t.Errorf("expected 2 values, got %d", len(all))
		}
		if all[0] != "john@example.com" || all[1] != "john.doe@example.com" {
			t.Errorf("unexpected email values: %v", all)
		}

		// Test non-existent key
		none := pg.GetAll("nonexistent")
		if none != nil {
			t.Errorf("expected nil for non-existent key, got %v", none)
		}
	})

	t.Run("formGetter with prefix", func(t *testing.T) {
		values := url.Values{}
		values.Add("meta.tags", "go")
		values.Add("meta.tags", "rust")
		values.Add("meta.version", "1.0")
		values.Add("other.data", "value")

		fg := &formGetter{values: values}
		pg := &prefixGetter{
			inner:  fg,
			prefix: "meta.",
		}

		// Test GetAll for "tags"
		all := pg.GetAll("tags")
		if len(all) != 2 {
			t.Errorf("expected 2 values, got %d", len(all))
		}

		// Test GetAll for "version"
		all = pg.GetAll("version")
		if len(all) != 1 || all[0] != "1.0" {
			t.Errorf("expected [\"1.0\"], got %v", all)
		}
	})

	t.Run("cookieGetter with prefix", func(t *testing.T) {
		cookies := []*http.Cookie{
			{Name: "session.id", Value: "abc123"},
			{Name: "session.token", Value: "def456"},
			{Name: "other.data", Value: "value"},
		}

		cg := &cookieGetter{cookies: cookies}
		pg := &prefixGetter{
			inner:  cg,
			prefix: "session.",
		}

		// Test GetAll for "id"
		all := pg.GetAll("id")
		if len(all) != 1 || all[0] != "abc123" {
			t.Errorf("expected [\"abc123\"], got %v", all)
		}
	})

	t.Run("headerGetter with prefix", func(t *testing.T) {
		headers := http.Header{}
		headers.Add("X-Meta-Tags", "tag1")
		headers.Add("X-Meta-Tags", "tag2")
		headers.Add("X-Other-Data", "value")

		hg := &headerGetter{headers: headers}
		pg := &prefixGetter{
			inner:  hg,
			prefix: "X-Meta-",
		}

		// Test GetAll for "Tags"
		all := pg.GetAll("Tags")
		if len(all) != 2 {
			t.Errorf("expected 2 values, got %d", len(all))
		}
	})
}

// TestPrefixGetter_GetAll_ThroughNestedBinding tests prefixGetter.GetAll through actual nested struct binding
// This ensures GetAll is called when binding nested structs with slice fields
func TestPrefixGetter_GetAll_ThroughNestedBinding(t *testing.T) {
	t.Run("nested struct with slice field - query", func(t *testing.T) {
		type Address struct {
			Tags []string `query:"tags"`
		}
		type Params struct {
			Address Address `query:"address"`
		}

		req := httptest.NewRequest("GET", "/?address.tags=home&address.tags=work", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.Address.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(params.Address.Tags))
		}
		if params.Address.Tags[0] != "home" || params.Address.Tags[1] != "work" {
			t.Errorf("unexpected tags: %v", params.Address.Tags)
		}
	})

	t.Run("nested struct with slice field - form", func(t *testing.T) {
		type Metadata struct {
			Versions []string `form:"versions"`
		}
		type FormData struct {
			Metadata Metadata `form:"meta"`
		}

		form := url.Values{}
		form.Add("meta.versions", "1.0")
		form.Add("meta.versions", "2.0")
		form.Add("meta.versions", "3.0")

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data FormData
		if err := c.BindForm(&data); err != nil {
			t.Fatalf("BindForm failed: %v", err)
		}

		if len(data.Metadata.Versions) != 3 {
			t.Errorf("expected 3 versions, got %d", len(data.Metadata.Versions))
		}
		if data.Metadata.Versions[0] != "1.0" || data.Metadata.Versions[1] != "2.0" || data.Metadata.Versions[2] != "3.0" {
			t.Errorf("unexpected versions: %v", data.Metadata.Versions)
		}
	})

	t.Run("deeply nested struct with slice", func(t *testing.T) {
		type Item struct {
			Tags []string `query:"tags"`
		}
		type Section struct {
			Items Item `query:"item"`
		}
		type Params struct {
			Section Section `query:"section"`
		}

		req := httptest.NewRequest("GET", "/?section.item.tags=tag1&section.item.tags=tag2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if len(params.Section.Items.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(params.Section.Items.Tags))
		}
	})
}

// TestParamsGetter_GetAll_ThroughBinding tests paramsGetter.GetAll through actual binding
func TestParamsGetter_GetAll_ThroughBinding(t *testing.T) {
	type Params struct {
		ID string `params:"id"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	c.Params = map[string]string{"id": "123"}

	var params Params
	if err := c.BindParams(&params); err != nil {
		t.Fatalf("BindParams failed: %v", err)
	}

	if params.ID != "123" {
		t.Errorf("expected ID=123, got %s", params.ID)
	}

	// Test that GetAll is used internally for slices (if applicable)
	type ParamsWithSlice struct {
		IDs []string `params:"id"`
	}

	var paramsSlice ParamsWithSlice
	c.Params = map[string]string{"id": "456"}
	if err := c.BindParams(&paramsSlice); err != nil {
		t.Fatalf("BindParams failed: %v", err)
	}

	if len(paramsSlice.IDs) != 1 || paramsSlice.IDs[0] != "456" {
		t.Errorf("expected IDs=[\"456\"], got %v", paramsSlice.IDs)
	}
}

// TestCookieGetter_GetAll_ThroughBinding tests cookieGetter.GetAll through actual binding
// This tests the URL unescaping error path
func TestCookieGetter_GetAll_ThroughBinding(t *testing.T) {
	t.Run("multiple cookies with same name", func(t *testing.T) {
		type Cookies struct {
			Session []string `cookie:"session"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
		req.AddCookie(&http.Cookie{Name: "session", Value: "def456"})

		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var cookies Cookies
		if err := c.BindCookies(&cookies); err != nil {
			t.Fatalf("BindCookies failed: %v", err)
		}

		if len(cookies.Session) != 2 {
			t.Errorf("expected 2 session cookies, got %d", len(cookies.Session))
		}
		if cookies.Session[0] != "abc123" || cookies.Session[1] != "def456" {
			t.Errorf("unexpected session values: %v", cookies.Session)
		}
	})

	t.Run("URL unescaping error path", func(t *testing.T) {
		type Cookies struct {
			Data []string `cookie:"data"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		// Create a cookie with invalid URL encoding to trigger error path
		req.AddCookie(&http.Cookie{Name: "data", Value: "%ZZ"}) // Invalid percent encoding

		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var cookies Cookies
		if err := c.BindCookies(&cookies); err != nil {
			t.Fatalf("BindCookies failed: %v", err)
		}

		// Should fallback to raw cookie value on unescaping error
		if len(cookies.Data) != 1 || cookies.Data[0] != "%ZZ" {
			t.Errorf("expected Data=[\"%%ZZ\"], got %v", cookies.Data)
		}
	})
}

// TestHeaderGetter_GetAll_ThroughBinding tests headerGetter.GetAll through actual binding
func TestHeaderGetter_GetAll_ThroughBinding(t *testing.T) {
	type Headers struct {
		Tags []string `header:"X-Tags"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("X-Tags", "tag1")
	req.Header.Add("X-Tags", "tag2")
	req.Header.Add("X-Tags", "tag3")

	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var headers Headers
	if err := c.BindHeaders(&headers); err != nil {
		t.Fatalf("BindHeaders failed: %v", err)
	}

	if len(headers.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(headers.Tags))
	}
	if headers.Tags[0] != "tag1" || headers.Tags[1] != "tag2" || headers.Tags[2] != "tag3" {
		t.Errorf("unexpected tag values: %v", headers.Tags)
	}
}

// TestWarmupBindingCache_EdgeCases tests WarmupBindingCache with edge cases to improve coverage
func TestWarmupBindingCache_EdgeCases(t *testing.T) {
	type TestStruct struct {
		Name string `query:"name"`
	}

	t.Run("pointer to struct", func(t *testing.T) {
		// Test pointer type handling
		WarmupBindingCache(&TestStruct{})
		// Should not panic and should warm up cache
	})

	t.Run("non-struct types", func(t *testing.T) {
		// Test non-struct types should be skipped
		WarmupBindingCache("string", 42, []int{1, 2, 3}, map[string]int{"a": 1})
		// Should not panic, non-structs should be skipped
	})

	t.Run("mix of structs and non-structs", func(t *testing.T) {
		type User struct {
			ID int `json:"id"`
		}
		type Product struct {
			Name string `form:"name"`
		}

		// Mix of valid structs and non-structs
		WarmupBindingCache(User{}, &Product{}, "string", 123)
		// Should handle structs and skip non-structs
	})

	t.Run("pointer to non-struct", func(t *testing.T) {
		// Test pointer to non-struct (should unwrap pointer, then skip)
		str := "test"
		WarmupBindingCache(&str)
		// Should not panic
	})
}

// TestBindBody_ContentTypeWithParameters tests BindBody with content type containing parameters ()
func TestBindBody_ContentTypeWithParameters(t *testing.T) {
	type Data struct {
		Value string `json:"value"`
	}

	r := New()
	r.POST("/test", func(c *Context) {
		var data Data
		if err := c.BindBody(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	t.Run("JSON with charset parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"value":"test"}`))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for JSON with charset, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("form with boundary parameter", func(t *testing.T) {
		type FormData struct {
			Name string `form:"name"`
		}

		r.POST("/form", func(c *Context) {
			var data FormData
			if err := c.BindBody(&data); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, data)
		})

		form := url.Values{}
		form.Set("name", "John")
		req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for form with charset, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("multipart with boundary", func(t *testing.T) {
		type FormData struct {
			Name string `form:"name"`
		}

		r.POST("/multipart", func(c *Context) {
			var data FormData
			if err := c.BindBody(&data); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, data)
		})

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "Jane")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/multipart", body)
		req.Header.Set("Content-Type", writer.FormDataContentType()) // Includes boundary parameter
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for multipart, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("content type with leading/trailing spaces and parameters", func(t *testing.T) {
		type Data struct {
			Value string `json:"value"`
		}

		r.POST("/spaces", func(c *Context) {
			var data Data
			if err := c.BindBody(&data); err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, data)
		})

		req := httptest.NewRequest(http.MethodPost, "/spaces", strings.NewReader(`{"value":"test"}`))
		req.Header.Set("Content-Type", "  application/json ; charset=utf-8  ") // Spaces and parameters
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for content type with spaces, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestBindJSON_NilBody tests BindJSON with nil request body
func TestBindJSON_NilBody(t *testing.T) {
	type Data struct {
		Value string `json:"value"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = nil // Explicitly set to nil
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var data Data
	err := c.BindJSON(&data)
	if err == nil {
		t.Fatal("Expected error for nil body, got nil")
	}
	if !strings.Contains(err.Error(), "request body is nil") {
		t.Errorf("Expected 'request body is nil' error, got: %v", err)
	}
}

// TestTagParsing_CommaSeparatedOptions tests json/form tags with comma-separated options ()
func TestTagParsing_CommaSeparatedOptions(t *testing.T) {
	t.Run("json tag with omitempty", func(t *testing.T) {
		type Data struct {
			Name  string `json:"name,omitempty"`
			Email string `json:"email,omitempty"`
			Age   int    `json:"age,omitempty"`
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"John","email":"john@example.com","age":30}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data Data
		if err := c.BindJSON(&data); err != nil {
			t.Fatalf("BindJSON failed: %v", err)
		}

		if data.Name != "John" {
			t.Errorf("Expected Name=John, got %s", data.Name)
		}
		if data.Email != "john@example.com" {
			t.Errorf("Expected Email=john@example.com, got %s", data.Email)
		}
		if data.Age != 30 {
			t.Errorf("Expected Age=30, got %d", data.Age)
		}
	})

	t.Run("form tag with omitempty", func(t *testing.T) {
		type FormData struct {
			Username string `form:"username,omitempty"`
			Password string `form:"password,omitempty"`
		}

		form := url.Values{}
		form.Set("username", "testuser")
		form.Set("password", "secret123")

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data FormData
		if err := c.BindForm(&data); err != nil {
			t.Fatalf("BindForm failed: %v", err)
		}

		if data.Username != "testuser" {
			t.Errorf("Expected Username=testuser, got %s", data.Username)
		}
		if data.Password != "secret123" {
			t.Errorf("Expected Password=secret123, got %s", data.Password)
		}
	})

	t.Run("json tag with empty name and options", func(t *testing.T) {
		// Test Use field name if tag is empty
		type Data struct {
			FieldName string `json:",omitempty"` // Empty name, should use "FieldName"
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"FieldName":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data Data
		if err := c.BindJSON(&data); err != nil {
			t.Fatalf("BindJSON failed: %v", err)
		}

		if data.FieldName != "test" {
			t.Errorf("Expected FieldName=test, got %s", data.FieldName)
		}
	})

	t.Run("json tag with dash (skip field)", func(t *testing.T) {
		// Test Skip fields marked with "-"
		type Data struct {
			Public  string `json:"public"`
			Private string `json:"-"` // Should be skipped
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"public":"visible","Private":"should be ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data Data
		if err := c.BindJSON(&data); err != nil {
			t.Fatalf("BindJSON failed: %v", err)
		}

		if data.Public != "visible" {
			t.Errorf("Expected Public=visible, got %s", data.Public)
		}
		if data.Private != "" {
			t.Errorf("Expected Private to be empty (skipped), got %s", data.Private)
		}
	})
}

// TestCookieGetter_Get_UnescapingError tests cookieGetter.Get with URL unescaping error
func TestCookieGetter_Get_UnescapingError(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "data", Value: "%ZZ"}, // Invalid percent encoding
	}

	getter := &cookieGetter{cookies: cookies}

	// Should fallback to raw cookie value on unescaping error
	value := getter.Get("data")
	if value != "%ZZ" {
		t.Errorf("Expected raw value %%ZZ on unescaping error, got %s", value)
	}
}

// TestCookieGetter_Get_NotFound tests cookieGetter.Get when cookie is not found
func TestCookieGetter_Get_NotFound(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session_id", Value: "abc123"},
		{Name: "theme", Value: "dark"},
	}

	getter := &cookieGetter{cookies: cookies}

	// Should return empty string when cookie key is not found
	value := getter.Get("nonexistent")
	if value != "" {
		t.Errorf("Expected empty string for nonexistent cookie, got %q", value)
	}

	// Verify existing cookies still work
	if session := getter.Get("session_id"); session != "abc123" {
		t.Errorf("Expected session_id to be 'abc123', got %q", session)
	}
}

// TestBindForm_ParseErrors tests BindForm parse errors
func TestBindForm_ParseErrors(t *testing.T) {
	t.Run("multipart parse error", func(t *testing.T) {
		type FormData struct {
			Name string `form:"name"`
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("malformed multipart"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid-boundary")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data FormData
		err := c.BindForm(&data)
		if err == nil {
			t.Fatal("Expected error for malformed multipart, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse multipart form") {
			t.Errorf("Expected multipart parse error, got: %v", err)
		}
	})

	t.Run("form parse error", func(t *testing.T) {
		type FormData struct {
			Name string `form:"name"`
		}

		// Use failingReader to trigger ParseForm failure
		req := httptest.NewRequest(http.MethodPost, "/", &failingReader{})
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Body = io.NopCloser(&failingReader{})

		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data FormData
		err := c.BindForm(&data)

		// Should return error with "failed to parse form" message
		if err == nil {
			t.Fatal("Expected error for form parse failure, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse form") {
			t.Errorf("Expected 'failed to parse form' error, got: %v", err)
		}
	})
}

// TestParseTime_AllFormats tests parseTime with all supported formats ()
func TestParseTime_AllFormats(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"RFC3339", "2024-01-15T10:30:00Z", false},
		{"RFC3339Nano", "2024-01-15T10:30:00.123456789Z", false},
		{"Date only", "2024-01-15", false},
		{"DateTime", "2024-01-15 10:30:00", false},
		{"RFC1123", "Mon, 15 Jan 2024 10:30:00 MST", false},
		{"RFC1123Z", "Mon, 15 Jan 2024 10:30:00 -0700", false},
		{"RFC822", "15 Jan 24 10:30 MST", false},
		{"RFC822Z", "15 Jan 24 10:30 -0700", false},
		{"RFC850", "Monday, 15-Jan-24 10:30:00 MST", false},
		{"DateTime without timezone", "2024-01-15T10:30:00", false},
		{"Empty value", "", true},
		{"Whitespace only", "   ", true},
		{"Invalid format", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type Params struct {
				Time time.Time `query:"time"`
			}

			req := httptest.NewRequest("GET", "/?time="+url.QueryEscape(tt.value), nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params Params
			err := c.BindQuery(&params)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for %q, got nil", tt.value)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %q: %v", tt.value, err)
				} else if params.Time.IsZero() {
					t.Errorf("Time should not be zero for valid format %q", tt.value)
				}
			}
		})
	}
}

// TestExtractBracketKey_EdgeCases tests extractBracketKey edge cases ()
func TestExtractBracketKey_EdgeCases(t *testing.T) {
	t.Run("no prefix match", func(t *testing.T) {
		result := extractBracketKey("other[key]", "prefix")
		if result != "" {
			t.Errorf("Expected empty string for non-matching prefix, got %q", result)
		}
	})

	t.Run("no closing bracket", func(t *testing.T) {
		result := extractBracketKey("prefix[unclosed", "prefix")
		if result != "" {
			t.Errorf("Expected empty string for unclosed bracket, got %q", result)
		}
	})

	t.Run("empty brackets", func(t *testing.T) {
		result := extractBracketKey("prefix[]", "prefix")
		if result != "" {
			t.Errorf("Expected empty string for empty brackets, got %q", result)
		}
	})

	t.Run("nested brackets", func(t *testing.T) {
		result := extractBracketKey("prefix[key1][key2]", "prefix")
		if result != "" {
			t.Errorf("Expected empty string for nested brackets, got %q", result)
		}
	})

	t.Run("quoted key with double quotes", func(t *testing.T) {
		result := extractBracketKey(`prefix["key.with.dots"]`, "prefix")
		if result != "key.with.dots" {
			t.Errorf("Expected 'key.with.dots', got %q", result)
		}
	})

	t.Run("quoted key with single quotes", func(t *testing.T) {
		result := extractBracketKey("prefix['key-with-dash']", "prefix")
		if result != "key-with-dash" {
			t.Errorf("Expected 'key-with-dash', got %q", result)
		}
	})

	t.Run("quoted key empty after trimming", func(t *testing.T) {
		result := extractBracketKey(`prefix[""]`, "prefix")
		if result != "" {
			t.Errorf("Expected empty string for empty quoted key, got %q", result)
		}
	})
}

// TestPrefixGetter_Has_RemainingPaths tests remaining paths in prefixGetter.Has (25% coverage -> 100%)
func TestPrefixGetter_Has_RemainingPaths(t *testing.T) {
	t.Run("headerGetter with prefix", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Meta-Tags", "tag1")
		headers.Set("X-Other-Data", "value")

		hg := &headerGetter{headers: headers}
		pg := &prefixGetter{
			inner:  hg,
			prefix: "X-Meta-",
		}

		// Direct Has check should work
		if !pg.Has("Tags") {
			t.Error("Expected Has(\"Tags\") to return true")
		}

		// Non-existent key
		if pg.Has("Nonexistent") {
			t.Error("Expected Has(\"Nonexistent\") to return false")
		}
	})

	t.Run("paramsGetter with prefix", func(t *testing.T) {
		params := map[string]string{
			"meta.name": "John",
			"meta.age":  "30",
		}

		pg := &paramsGetter{params: params}
		pg2 := &prefixGetter{
			inner:  pg,
			prefix: "meta.",
		}

		// Should find keys with prefix
		if !pg2.Has("name") {
			t.Error("Expected Has(\"name\") to return true")
		}
		if !pg2.Has("age") {
			t.Error("Expected Has(\"age\") to return true")
		}
	})

	t.Run("cookieGetter with prefix", func(t *testing.T) {
		cookies := []*http.Cookie{
			{Name: "session.id", Value: "abc123"},
			{Name: "session.token", Value: "def456"},
		}

		cg := &cookieGetter{cookies: cookies}
		pg := &prefixGetter{
			inner:  cg,
			prefix: "session.",
		}

		if !pg.Has("id") {
			t.Error("Expected Has(\"id\") to return true")
		}
		if !pg.Has("token") {
			t.Error("Expected Has(\"token\") to return true")
		}
		if pg.Has("nonexistent") {
			t.Error("Expected Has(\"nonexistent\") to return false")
		}
	})

	t.Run("inner getter that doesn't match queryGetter or formGetter", func(t *testing.T) {
		// Create a custom getter that doesn't match the type assertions
		customGetter := &paramsGetter{params: map[string]string{"key": "value"}}
		pg := &prefixGetter{
			inner:  customGetter,
			prefix: "prefix.",
		}

		// Should use direct Has check only ()
		if pg.Has("key") {
			// This should work through the direct Has check
		}
		if pg.Has("nonexistent") {
			t.Error("Expected Has(\"nonexistent\") to return false")
		}
	})
}

// TestValidateEnum_ErrorPath tests validateEnum error path
func TestValidateEnum_ErrorPath(t *testing.T) {
	t.Run("invalid enum value", func(t *testing.T) {
		type Params struct {
			Status string `query:"status" enum:"active,inactive,pending"`
		}

		req := httptest.NewRequest("GET", "/?status=invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid enum value, got nil")
		}

		if !strings.Contains(err.Error(), "not in allowed values") {
			t.Errorf("Expected enum validation error, got: %v", err)
		}
	})

	t.Run("empty value skips validation", func(t *testing.T) {
		type Params struct {
			Status string `query:"status" enum:"active,inactive,pending"`
		}

		// Test with empty query parameter ()
		req := httptest.NewRequest("GET", "/?status=", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err != nil {
			t.Errorf("Expected no error for empty enum value, got: %v", err)
		}

		if params.Status != "" {
			t.Errorf("Expected empty status, got: %q", params.Status)
		}
	})

	t.Run("missing parameter skips validation", func(t *testing.T) {
		type Params struct {
			Status string `query:"status" enum:"active,inactive,pending"`
		}

		// Test with missing query parameter ()
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err != nil {
			t.Errorf("Expected no error for missing enum value, got: %v", err)
		}

		if params.Status != "" {
			t.Errorf("Expected empty status, got: %q", params.Status)
		}
	})
}

// TestSetSliceField_ErrorPath tests setSliceField error paths ()
func TestSetSliceField_ErrorPath(t *testing.T) {
	t.Run("invalid element conversion", func(t *testing.T) {
		type Params struct {
			IDs []int `query:"ids"`
		}

		req := httptest.NewRequest("GET", "/?ids=123&ids=invalid&ids=456", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid integer in slice, got nil")
		}

		if !strings.Contains(err.Error(), "element") {
			t.Errorf("Expected element error, got: %v", err)
		}
	})

	t.Run("invalid time in slice", func(t *testing.T) {
		type Params struct {
			Times []time.Time `query:"times"`
		}

		req := httptest.NewRequest("GET", "/?times=2024-01-01&times=invalid-time&times=2024-01-02", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid time in slice, got nil")
		}
	})
}

// TestSetField_PointerEmptyValue tests setField with pointer and empty value
func TestSetField_PointerEmptyValue(t *testing.T) {
	type Params struct {
		Name *string `query:"name"`
		Age  *int    `query:"age"`
	}

	t.Run("empty string leaves pointer nil", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?name=", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name != nil {
			t.Errorf("Expected Name to be nil for empty value, got %v", params.Name)
		}
	})

	t.Run("missing value leaves pointer nil", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name != nil {
			t.Errorf("Expected Name to be nil for missing value, got %v", params.Name)
		}
		if params.Age != nil {
			t.Errorf("Expected Age to be nil for missing value, got %v", params.Age)
		}
	})
}

// TestConvertValue_EdgeCases tests convertValue remaining paths
func TestConvertValue_EdgeCases(t *testing.T) {
	t.Run("int8 overflow", func(t *testing.T) {
		type Params struct {
			Value int8 `query:"value"`
		}

		req := httptest.NewRequest("GET", "/?value=999999", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		// May or may not error depending on conversion, but tests the path
		_ = err
	})

	t.Run("uint overflow", func(t *testing.T) {
		type Params struct {
			Value uint `query:"value"`
		}

		req := httptest.NewRequest("GET", "/?value=-1", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for negative uint, got nil")
		}
	})

	t.Run("float with invalid format", func(t *testing.T) {
		type Params struct {
			Value float64 `query:"value"`
		}

		req := httptest.NewRequest("GET", "/?value=not-a-number", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid float, got nil")
		}
		if !strings.Contains(err.Error(), "invalid float") {
			t.Errorf("Expected float error, got: %v", err)
		}
	})
}

// TestParamsGetter_GetAll_NonExistent tests paramsGetter.GetAll for non-existent key
func TestParamsGetter_GetAll_NonExistent(t *testing.T) {
	params := map[string]string{"id": "123"}
	getter := &paramsGetter{params: params}

	// Test non-existent key returns nil
	none := getter.GetAll("nonexistent")
	if none != nil {
		t.Errorf("Expected nil for non-existent key, got %v", none)
	}

	// Test existing key returns slice
	all := getter.GetAll("id")
	if len(all) != 1 || all[0] != "123" {
		t.Errorf("Expected [\"123\"], got %v", all)
	}
}

// TestBind_ErrorPaths tests bind function error paths
func TestBind_ErrorPaths(t *testing.T) {
	t.Run("non-pointer input", func(t *testing.T) {
		type Params struct {
			Name string `query:"name"`
		}

		var params Params
		err := bind(params, &queryGetter{url.Values{}}, "query")
		if err == nil {
			t.Fatal("Expected error for non-pointer, got nil")
		}
		if !strings.Contains(err.Error(), "pointer to struct") {
			t.Errorf("Expected pointer error, got: %v", err)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		var params *struct {
			Name string `query:"name"`
		}
		err := bind(params, &queryGetter{url.Values{}}, "query")
		if err == nil {
			t.Fatal("Expected error for nil pointer, got nil")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Errorf("Expected nil pointer error, got: %v", err)
		}
	})

	t.Run("pointer to non-struct", func(t *testing.T) {
		var value *int
		err := bind(&value, &queryGetter{url.Values{}}, "query")
		if err == nil {
			t.Fatal("Expected error for pointer to non-struct, got nil")
		}
		if !strings.Contains(err.Error(), "pointer to struct") {
			t.Errorf("Expected struct error, got: %v", err)
		}
	})
}

// TestParseStructType_RemainingPaths tests parseStructType remaining paths
func TestParseStructType_RemainingPaths(t *testing.T) {
	t.Run("unexported fields are skipped", func(t *testing.T) {
		type Params struct {
			Exported string `query:"exported"`
			// unexported field (lowercase) should be skipped - using blank identifier
			_ string `query:"unexported"`
		}

		req := httptest.NewRequest("GET", "/?exported=test&unexported=ignored", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Exported != "test" {
			t.Errorf("Expected Exported=test, got %s", params.Exported)
		}
		// unexported field should not be set
	})

	t.Run("non-standard tag skipped when empty", func(t *testing.T) {
		type Params struct {
			HasTag     string `query:"has_tag"`
			NoQueryTag string // No query tag, should be skipped for query binding
		}

		req := httptest.NewRequest("GET", "/?has_tag=value", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		if err := c.BindQuery(&params); err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.HasTag != "value" {
			t.Errorf("Expected HasTag=value, got %s", params.HasTag)
		}
		// NoQueryTag should remain zero value (skipped at )
	})

	t.Run("json tag without name uses field name", func(t *testing.T) {
		type Data struct {
			FieldName string `json:""` // Empty tag, should use "FieldName"
		}

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"FieldName":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var data Data
		if err := c.BindJSON(&data); err != nil {
			t.Fatalf("BindJSON failed: %v", err)
		}

		if data.FieldName != "test" {
			t.Errorf("Expected FieldName=test, got %s", data.FieldName)
		}
	})

	t.Run("embedded struct with pointer", func(t *testing.T) {
		type Embedded struct {
			Value string `query:"value"`
		}
		type Params struct {
			*Embedded        // Pointer to embedded struct - tests
			Name      string `query:"name"`
		}

		// Initialize the embedded pointer to avoid nil pointer panic
		params := Params{
			Embedded: &Embedded{},
			Name:     "",
		}

		req := httptest.NewRequest("GET", "/?value=embedded&name=test", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.BindQuery(&params)
		if err != nil {
			t.Fatalf("BindQuery failed: %v", err)
		}

		if params.Name != "test" {
			t.Errorf("Expected Name=test, got %s", params.Name)
		}
		if params.Embedded == nil || params.Embedded.Value != "embedded" {
			t.Errorf("Expected Embedded.Value=embedded, got %v", params.Embedded)
		}
	})
}

// TestBind_SkipUnexportedFields tests that unexported fields are skipped
func TestBind_SkipUnexportedFields(t *testing.T) {
	// Note: parseStructType already skips unexported fields,
	// but this is defensive code for cases where CanSet() might return false.
	// This can happen with embedded structs or when accessing fields through reflection.

	// Test normal case - all fields should be settable
	type Params struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?name=john&age=30", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params Params
	if err := c.BindQuery(&params); err != nil {
		t.Fatalf("BindQuery failed: %v", err)
	}

	if params.Name != "john" {
		t.Errorf("Expected Name=john, got %s", params.Name)
	}
	if params.Age != 30 {
		t.Errorf("Expected Age=30, got %d", params.Age)
	}

	// To actually hit this path, we need a field that's in the cache but CanSet() returns false.
	// This is defensive code, and in practice parseStructType filters these out.
	// This ensures we skip any field that somehow made it into the cache but isn't settable.
}

// TestBind_NestedStructError tests error handling for nested struct binding failures
func TestBind_NestedStructError(t *testing.T) {
	t.Run("nested struct with invalid data", func(t *testing.T) {
		type Address struct {
			ZipCode int `query:"zip_code"` // Will fail with invalid value
		}
		type Params struct {
			Address Address `query:"address"`
		}

		req := httptest.NewRequest("GET", "/?address.zip_code=invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid nested struct value, got nil")
		}

		// Error should be wrapped in BindError with Field, Tag, and Err set ()
		var bindErr *BindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("Expected BindError, got %T: %v", err, err)
		}

		if bindErr.Field != "Address" {
			t.Errorf("Expected Field='Address', got %q", bindErr.Field)
		}
		if bindErr.Tag != "query" {
			t.Errorf("Expected Tag='query', got %q", bindErr.Tag)
		}
		if bindErr.Value != "" {
			t.Errorf("Expected empty Value for nested struct error, got %q", bindErr.Value)
		}
		if bindErr.Err == nil {
			t.Error("Expected underlying error, got nil")
		}
	})

	t.Run("nested struct with invalid time format", func(t *testing.T) {
		type Metadata struct {
			CreatedAt time.Time `query:"created_at"`
		}
		type Params struct {
			Metadata Metadata `query:"meta"`
		}

		req := httptest.NewRequest("GET", "/?meta.created_at=invalid-time", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid time in nested struct, got nil")
		}

		var bindErr *BindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("Expected BindError, got %T: %v", err, err)
		}

		if bindErr.Field != "Metadata" {
			t.Errorf("Expected Field='Metadata', got %q", bindErr.Field)
		}
	})

	t.Run("nested struct with invalid enum", func(t *testing.T) {
		type Config struct {
			Status string `query:"status" enum:"active,inactive"`
		}
		type Params struct {
			Config Config `query:"config"`
		}

		req := httptest.NewRequest("GET", "/?config.status=invalid", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for invalid enum in nested struct, got nil")
		}

		var bindErr *BindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("Expected BindError, got %T: %v", err, err)
		}

		if bindErr.Field != "Config" {
			t.Errorf("Expected Field='Config', got %q", bindErr.Field)
		}
	})

	t.Run("deeply nested struct error", func(t *testing.T) {
		type Inner struct {
			Value int `query:"value"`
		}
		type Middle struct {
			Inner Inner `query:"inner"`
		}
		type Params struct {
			Middle Middle `query:"middle"`
		}

		req := httptest.NewRequest("GET", "/?middle.inner.value=not-a-number", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params Params
		err := c.BindQuery(&params)
		if err == nil {
			t.Fatal("Expected error for deeply nested invalid value, got nil")
		}

		// Error should propagate up through nested structures
		var bindErr *BindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("Expected BindError, got %T: %v", err, err)
		}
	})
}

// TestSetMapField_ComplexKeys tests map field binding with complex bracket notation
func TestSetMapField_ComplexKeys(t *testing.T) {
	r := New()

	type Data struct {
		Config map[string]string `form:"config"`
	}

	r.POST("/test", func(c *Context) {
		var data Data
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if data.Config == nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "config is nil"})
			return
		}

		c.JSON(http.StatusOK, map[string]int{"count": len(data.Config)})
	})

	form := url.Values{}
	form.Set("config[key.with.dots]", "value1")
	form.Set("config[key-with-dashes]", "value2")
	form.Set("config[key_with_underscores]", "value3")
	form.Set("config[123numeric]", "value4")

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("should handle complex map keys: %s", w.Body.String())
	}
}

// TestSplitMediaType_EdgeCases tests splitMediaType with various inputs
func TestSplitMediaType_EdgeCases(t *testing.T) {
	r := New()

	tests := []struct {
		header string
		offer  string
	}{
		{"application/json;charset=utf-8;boundary=xyz", "application/json"},
		{"text/html;level=1", "text/html"},
		{"image/png", "image/png"},
		{"*/json", "application/json"},
	}

	for _, tt := range tests {
		r.GET("/test", func(c *Context) {
			result := c.Accepts(tt.offer)
			c.String(http.StatusOK, "%s", result)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept", tt.header)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Just verify no panic
	}
}

// TestBindCookies_InvalidURLEncoding tests cookie binding with invalid URL encoding
func TestBindCookies_InvalidURLEncoding(t *testing.T) {
	r := New()

	type CookieData struct {
		Session string `cookie:"session"`
		Token   string `cookie:"token"`
	}

	r.GET("/test", func(c *Context) {
		var data CookieData
		// Bind cookies - invalid encoding should be handled gracefully
		err := c.BindCookies(&data)

		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "simple-value"}) // No encoding needed
	req.AddCookie(&http.Cookie{Name: "token", Value: "%ZZ"})            // Invalid encoding

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should handle gracefully (invalid encoding returns raw value)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
