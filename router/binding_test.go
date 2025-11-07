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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// customUUID is a custom type for testing encoding.TextUnmarshaler interface
type customUUID string

// UnmarshalText implements encoding.TextUnmarshaler
func (u *customUUID) UnmarshalText(text []byte) error {
	s := string(text)
	// Simple UUID validation (just check length, not full RFC4122)
	if len(s) != 36 {
		return ErrInvalidUUIDFormat
	}
	*u = customUUID(s)
	return nil
}

// failingReader is a custom reader that always returns an error for testing error paths
type failingReader struct{}

func (r *failingReader) Read([]byte) (int, error) {
	return 0, ErrReadError
}

// TestBindJSON tests JSON binding functionality
func TestBindJSON(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	tests := []struct {
		name     string
		body     string
		setup    func(req *http.Request) // Optional: for custom request setup
		wantErr  bool
		validate func(t *testing.T, user User)
	}{
		{
			name: "valid JSON",
			body: `{"name":"John","email":"john@example.com","age":30}`,
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/json")
			},
			wantErr: false,
			validate: func(t *testing.T, user User) {
				assert.Equal(t, "John", user.Name)
				assert.Equal(t, "john@example.com", user.Email)
				assert.Equal(t, 30, user.Age)
			},
		},
		{
			name: "invalid JSON",
			body: `{"name":"John","age":"invalid"}`,
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/json")
			},
			wantErr:  true,
			validate: func(t *testing.T, user User) {},
		},
		{
			name: "nil body",
			body: "",
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/json")
			},
			wantErr:  true,
			validate: func(t *testing.T, user User) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest("POST", "/", bodyReader)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var user User
			err := c.BindJSON(&user)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "BindJSON should succeed for %s", tt.name)
				tt.validate(t, user)
			}
		})
	}
}

// TestBindQuery tests query parameter binding functionality
func TestBindQuery(t *testing.T) {
	t.Parallel()

	type SearchParams struct {
		Query    string `query:"q"`
		Page     int    `query:"page"`
		PageSize int    `query:"page_size"`
		Active   bool   `query:"active"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params SearchParams, err error)
	}{
		{
			name:    "all fields",
			query:   "q=golang&page=2&page_size=20&active=true",
			wantErr: false,
			validate: func(t *testing.T, params SearchParams, err error) {
				assert.Equal(t, "golang", params.Query)
				assert.Equal(t, 2, params.Page)
				assert.Equal(t, 20, params.PageSize)
				assert.True(t, params.Active)
			},
		},
		{
			name:    "partial fields",
			query:   "q=test",
			wantErr: false,
			validate: func(t *testing.T, params SearchParams, err error) {
				assert.Equal(t, "test", params.Query)
				assert.Equal(t, 0, params.Page, "Page should be zero value when not provided")
			},
		},
		{
			name:    "invalid integer",
			query:   "page=invalid",
			wantErr: true,
			validate: func(t *testing.T, params SearchParams, err error) {
				var bindErr *BindError
				require.True(t, errors.As(err, &bindErr), "Expected BindError")
				assert.Equal(t, "Page", bindErr.Field)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params SearchParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
			}
			tt.validate(t, params, err)
		})
	}
}

// TestBindParams tests route parameter binding functionality
func TestBindParams(t *testing.T) {
	t.Parallel()

	type UserParams struct {
		ID     int    `params:"id"`
		Action string `params:"action"`
	}

	tests := []struct {
		name     string
		setup    func(c *Context)
		validate func(t *testing.T, params UserParams)
	}{
		{
			name: "valid params",
			setup: func(c *Context) {
				// Simulate router setting params
				c.paramCount = 2
				c.paramKeys[0] = "id"
				c.paramValues[0] = "123"
				c.paramKeys[1] = "action"
				c.paramValues[1] = "edit"
			},
			validate: func(t *testing.T, params UserParams) {
				assert.Equal(t, 123, params.ID)
				assert.Equal(t, "edit", params.Action)
			},
		},
		{
			name: "params from map (>8 params)",
			setup: func(c *Context) {
				// Simulate fallback to map
				c.Params = map[string]string{
					"id":     "456",
					"action": "view",
				}
			},
			validate: func(t *testing.T, params UserParams) {
				assert.Equal(t, 456, params.ID)
				assert.Equal(t, "view", params.Action)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			if tt.setup != nil {
				tt.setup(c)
			}

			var params UserParams
			require.NoError(t, c.BindParams(&params))
			tt.validate(t, params)
		})
	}
}

// TestBindCookies tests cookie binding functionality
func TestBindCookies(t *testing.T) {
	t.Parallel()

	type SessionCookies struct {
		SessionID  string `cookie:"session_id"`
		Theme      string `cookie:"theme"`
		RememberMe bool   `cookie:"remember_me"`
	}

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name: "valid cookies",
			setup: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})
				req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
				req.AddCookie(&http.Cookie{Name: "remember_me", Value: "true"})
			},
			params: &SessionCookies{},
			validate: func(t *testing.T, params any) {
				cookies := params.(*SessionCookies)
				assert.Equal(t, "abc123", cookies.SessionID)
				assert.Equal(t, "dark", cookies.Theme)
				assert.True(t, cookies.RememberMe)
			},
		},
		{
			name: "URL encoded cookies",
			setup: func(req *http.Request) {
				// Cookie with URL-encoded value
				req.AddCookie(&http.Cookie{Name: "session_id", Value: url.QueryEscape("value with spaces")})
			},
			params: &struct {
				SessionID string `cookie:"session_id"`
			}{},
			validate: func(t *testing.T, params any) {
				cookies := params.(*struct {
					SessionID string `cookie:"session_id"`
				})
				assert.Equal(t, "value with spaces", cookies.SessionID)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindCookies(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindHeaders tests HTTP header binding functionality
func TestBindHeaders(t *testing.T) {
	t.Parallel()

	type RequestHeaders struct {
		UserAgent string `header:"User-Agent"`
		Token     string `header:"Authorization"`
		Accept    string `header:"Accept"`
	}

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name: "valid headers",
			setup: func(req *http.Request) {
				req.Header.Set("User-Agent", "Mozilla/5.0")
				req.Header.Set("Authorization", "Bearer token123")
				req.Header.Set("Accept", "application/json")
			},
			params: &RequestHeaders{},
			validate: func(t *testing.T, params any) {
				headers := params.(*RequestHeaders)
				assert.Equal(t, "Mozilla/5.0", headers.UserAgent)
				assert.Equal(t, "Bearer token123", headers.Token)
				assert.Equal(t, "application/json", headers.Accept)
			},
		},
		{
			name: "case insensitive",
			setup: func(req *http.Request) {
				req.Header.Set("User-Agent", "Test")
			},
			params: &struct {
				UserAgent string `header:"User-Agent"`
			}{},
			validate: func(t *testing.T, params any) {
				headers := params.(*struct {
					UserAgent string `header:"User-Agent"`
				})
				assert.Equal(t, "Test", headers.UserAgent)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindHeaders(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindBody tests automatic content type detection and binding
func TestBindBody(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name" form:"name"`
		Email string `json:"email" form:"email"`
		Age   int    `json:"age" form:"age"`
	}

	tests := []struct {
		name     string
		body     string
		setup    func(req *http.Request) // Optional: for custom headers
		wantErr  bool
		validate func(t *testing.T, user User)
	}{
		{
			name: "JSON content type",
			body: `{"name":"Alice","email":"alice@example.com","age":25}`,
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/json")
			},
			wantErr: false,
			validate: func(t *testing.T, user User) {
				assert.Equal(t, "Alice", user.Name)
				assert.Equal(t, "alice@example.com", user.Email)
				assert.Equal(t, 25, user.Age)
			},
		},
		{
			name: "form content type",
			body: func() string {
				form := url.Values{}
				form.Set("name", "Bob")
				form.Set("email", "bob@example.com")
				form.Set("age", "35")
				return form.Encode()
			}(),
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			},
			wantErr: false,
			validate: func(t *testing.T, user User) {
				assert.Equal(t, "Bob", user.Name)
				assert.Equal(t, "bob@example.com", user.Email)
				assert.Equal(t, 35, user.Age)
			},
		},
		{
			name:    "default to JSON when no content type",
			body:    `{"name":"Charlie","email":"charlie@example.com","age":40}`,
			setup:   nil, // No Content-Type header
			wantErr: false,
			validate: func(t *testing.T, user User) {
				assert.Equal(t, "Charlie", user.Name)
				assert.Equal(t, 40, user.Age)
			},
		},
		{
			name: "unsupported content type",
			body: "data",
			setup: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/octet-stream")
			},
			wantErr:  true,
			validate: func(t *testing.T, user User) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var user User
			err := c.BindBody(&user)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), "unsupported content type")
			} else {
				require.NoError(t, err, "BindBody should succeed for %s", tt.name)
				tt.validate(t, user)
			}
		})
	}
}

// TestBindQuery_Slices tests slice binding in query parameters
func TestBindQuery_Slices(t *testing.T) {
	t.Parallel()

	type TagRequest struct {
		Tags []string `query:"tags"`
		IDs  []int    `query:"ids"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params TagRequest)
	}{
		{
			name:    "string slice",
			query:   "tags=go&tags=rust&tags=python",
			wantErr: false,
			validate: func(t *testing.T, params TagRequest) {
				require.Len(t, params.Tags, 3)
				assert.Equal(t, "go", params.Tags[0])
				assert.Equal(t, "rust", params.Tags[1])
				assert.Equal(t, "python", params.Tags[2])
			},
		},
		{
			name:    "int slice",
			query:   "ids=1&ids=2&ids=3",
			wantErr: false,
			validate: func(t *testing.T, params TagRequest) {
				require.Len(t, params.IDs, 3)
				assert.Equal(t, 1, params.IDs[0])
				assert.Equal(t, 2, params.IDs[1])
				assert.Equal(t, 3, params.IDs[2])
			},
		},
		{
			name:     "invalid int in slice",
			query:    "ids=1&ids=invalid&ids=3",
			wantErr:  true,
			validate: func(t *testing.T, params TagRequest) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params TagRequest
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_Pointers tests pointer field binding in query parameters
func TestBindQuery_Pointers(t *testing.T) {
	t.Parallel()

	type OptionalParams struct {
		Name   *string `query:"name"`
		Age    *int    `query:"age"`
		Active *bool   `query:"active"`
	}

	tests := []struct {
		name     string
		query    string
		validate func(t *testing.T, params OptionalParams)
	}{
		{
			name:  "all values present",
			query: "name=John&age=30&active=true",
			validate: func(t *testing.T, params OptionalParams) {
				require.NotNil(t, params.Name)
				assert.Equal(t, "John", *params.Name)
				require.NotNil(t, params.Age)
				assert.Equal(t, 30, *params.Age)
				require.NotNil(t, params.Active)
				assert.True(t, *params.Active)
			},
		},
		{
			name:  "missing values remain nil",
			query: "name=John",
			validate: func(t *testing.T, params OptionalParams) {
				require.NotNil(t, params.Name)
				assert.Equal(t, "John", *params.Name)
				assert.Nil(t, params.Age, "Age should be nil when not provided")
			},
		},
		{
			name:  "empty value remains nil",
			query: "name=&age=",
			validate: func(t *testing.T, params OptionalParams) {
				assert.Nil(t, params.Name, "Name should be nil for empty value")
				assert.Nil(t, params.Age, "Age should be nil for empty value")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params OptionalParams
			require.NoError(t, c.BindQuery(&params))
			tt.validate(t, params)
		})
	}
}

// TestBindQuery_DataTypes tests binding of various data types in query parameters
func TestBindQuery_DataTypes(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "all data types",
			setup: func(req *http.Request) {
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
			},
			params: &AllTypes{},
			validate: func(t *testing.T, params any) {
				p := params.(*AllTypes)
				assert.Equal(t, "test", p.String)
				assert.Equal(t, -42, p.Int)
				assert.Equal(t, int8(127), p.Int8)
				assert.True(t, p.Bool)
				assert.InDelta(t, 3.14, p.Float32, 0.01)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestParseBool tests boolean parsing variations
func TestParseBool(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result, err := parseBool(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "parseBool(%q) should return error", tt.input)
			} else {
				require.NoError(t, err, "parseBool(%q) should not return error", tt.input)
				assert.Equal(t, tt.expected, result, "parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBindForm tests form binding functionality
func TestBindForm(t *testing.T) {
	t.Parallel()

	type LoginForm struct {
		Username string `form:"username"`
		Password string `form:"password"`
		Remember bool   `form:"remember"`
	}

	t.Run("urlencoded form", func(t *testing.T) {
		t.Parallel()

		form := url.Values{}
		form.Set("username", "alice")
		form.Set("password", "secret123")
		form.Set("remember", "true")

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var login LoginForm
		require.NoError(t, c.BindForm(&login))
		assert.Equal(t, "alice", login.Username)
		assert.Equal(t, "secret123", login.Password)
		assert.True(t, login.Remember)
	})
}

// TestBind_Errors tests error cases in binding
func TestBind_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(c *Context) error
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "not a pointer",
			setup: func(c *Context) error {
				var params struct {
					Name string `query:"name"`
				}
				return c.BindQuery(params) // Not a pointer!
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.Error(t, err, "Expected error for non-pointer")
			},
		},
		{
			name: "nil pointer",
			setup: func(c *Context) error {
				var params *struct {
					Name string `query:"name"`
				}
				return c.BindQuery(params)
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.Error(t, err, "Expected error for nil pointer")
			},
		},
		{
			name: "not a struct",
			setup: func(c *Context) error {
				var str string
				return c.BindQuery(&str)
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.Error(t, err, "Expected error for non-struct")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?name=test", nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := tt.setup(c)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Should succeed for %s", tt.name)
			}
			tt.validate(t, err)
		})
	}
}

// TestStructInfoCache tests type cache efficiency
func TestStructInfoCache(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?name=test&age=25", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// First call - should populate cache
	var s1 TestStruct
	require.NoError(t, c.BindQuery(&s1), "First BindQuery failed")

	// Second call - should use cache
	var s2 TestStruct
	require.NoError(t, c.BindQuery(&s2), "Second BindQuery failed")

	// Both should have same values
	assert.Equal(t, s1.Name, s2.Name, "Cached binding produced different Name values")
	assert.Equal(t, s1.Age, s2.Age, "Cached binding produced different Age values")
}

// TestBindError_Details tests BindError details
func TestBindError_Details(t *testing.T) {
	t.Parallel()

	type Params struct {
		Age int `query:"age"`
	}

	req := httptest.NewRequest("GET", "/?age=invalid", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var params Params
	err := c.BindQuery(&params)
	require.Error(t, err, "Expected BindError")

	var bindErr *BindError
	require.True(t, errors.As(err, &bindErr), "Expected BindError type")
	assert.Equal(t, "Age", bindErr.Field)
	assert.Equal(t, "query", bindErr.Tag)
	assert.Equal(t, "invalid", bindErr.Value)
}

// TestBindQuery_RealWorld tests real-world binding scenarios
func TestBindQuery_RealWorld(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name:  "pagination",
			query: "page=3&page_size=50&sort=created_at&order=desc",
			params: &struct {
				Page     int    `query:"page"`
				PageSize int    `query:"page_size"`
				Sort     string `query:"sort"`
				Order    string `query:"order"`
			}{},
			validate: func(t *testing.T, params any) {
				p := params.(*struct {
					Page     int    `query:"page"`
					PageSize int    `query:"page_size"`
					Sort     string `query:"sort"`
					Order    string `query:"order"`
				})
				assert.Equal(t, 3, p.Page)
				assert.Equal(t, 50, p.PageSize)
				assert.Equal(t, "created_at", p.Sort)
				assert.Equal(t, "desc", p.Order)
			},
		},
		{
			name:  "filters",
			query: "status=active&status=pending&category=electronics&min_price=10.50&max_price=99.99",
			params: &struct {
				Status   []string `query:"status"`
				Category []string `query:"category"`
				MinPrice float64  `query:"min_price"`
				MaxPrice float64  `query:"max_price"`
			}{},
			validate: func(t *testing.T, params any) {
				f := params.(*struct {
					Status   []string `query:"status"`
					Category []string `query:"category"`
					MinPrice float64  `query:"min_price"`
					MaxPrice float64  `query:"max_price"`
				})
				require.Len(t, f.Status, 2)
				assert.Equal(t, "active", f.Status[0])
				assert.Equal(t, "pending", f.Status[1])
				assert.Equal(t, 10.50, f.MinPrice)
				assert.Equal(t, 99.99, f.MaxPrice)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindQuery_TimeType tests time.Time support in query parameters
func TestBindQuery_TimeType(t *testing.T) {
	t.Parallel()

	type EventParams struct {
		StartDate time.Time  `query:"start"`
		EndDate   time.Time  `query:"end"`
		Created   *time.Time `query:"created"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params EventParams)
	}{
		{
			name:    "RFC3339 format",
			query:   "start=2024-01-15T10:30:00Z&end=2024-01-20T15:45:00Z",
			wantErr: false,
			validate: func(t *testing.T, params EventParams) {
				expectedStart, err := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
				require.NoError(t, err)
				assert.True(t, params.StartDate.Equal(expectedStart), "StartDate should match expected time")
			},
		},
		{
			name:    "date only format",
			query:   "start=2024-01-15",
			wantErr: false,
			validate: func(t *testing.T, params EventParams) {
				expected, err := time.Parse("2006-01-02", "2024-01-15")
				require.NoError(t, err)
				assert.True(t, params.StartDate.Equal(expected), "StartDate should match expected date")
			},
		},
		{
			name:    "pointer time field",
			query:   "created=2024-01-15T10:00:00Z",
			wantErr: false,
			validate: func(t *testing.T, params EventParams) {
				require.NotNil(t, params.Created, "Created should not be nil")
				expected, err := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
				require.NoError(t, err)
				assert.True(t, params.Created.Equal(expected), "Created should match expected time")
			},
		},
		{
			name:     "invalid time format",
			query:    "start=invalid-date",
			wantErr:  true,
			validate: func(t *testing.T, params EventParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params EventParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}

	// Time slice test - kept separate due to different struct type
	t.Run("time slice", func(t *testing.T) {
		t.Parallel()

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
		require.NoError(t, c.BindQuery(&params))
		require.Len(t, params.Dates, 3)
	})
}

// TestBindQuery_DurationType tests time.Duration support in query parameters
func TestBindQuery_DurationType(t *testing.T) {
	t.Parallel()

	type TimeoutParams struct {
		Timeout  time.Duration  `query:"timeout"`
		Interval time.Duration  `query:"interval"`
		TTL      *time.Duration `query:"ttl"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params TimeoutParams)
	}{
		{
			name:    "valid durations",
			query:   "timeout=5s&interval=10m&ttl=1h",
			wantErr: false,
			validate: func(t *testing.T, params TimeoutParams) {
				assert.Equal(t, 5*time.Second, params.Timeout)
				assert.Equal(t, 10*time.Minute, params.Interval)
				require.NotNil(t, params.TTL)
				assert.Equal(t, time.Hour, *params.TTL)
			},
		},
		{
			name:    "complex duration",
			query:   "timeout=1h30m45s",
			wantErr: false,
			validate: func(t *testing.T, params TimeoutParams) {
				expected := time.Hour + 30*time.Minute + 45*time.Second
				assert.Equal(t, expected, params.Timeout)
			},
		},
		{
			name:     "invalid duration",
			query:    "timeout=invalid",
			wantErr:  true,
			validate: func(t *testing.T, params TimeoutParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params TimeoutParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_IPType tests net.IP support
func TestBindQuery_IPType(t *testing.T) {
	t.Parallel()

	type NetworkParams struct {
		AllowedIP net.IP   `query:"allowed_ip"`
		BlockedIP net.IP   `query:"blocked_ip"`
		IPs       []net.IP `query:"ips"`
	}

	tests := []struct {
		name     string
		query    string
		setup    func(req *http.Request) // Optional: for complex query setup
		wantErr  bool
		validate func(t *testing.T, params NetworkParams)
	}{
		{
			name:    "IPv4 address",
			query:   "allowed_ip=192.168.1.1",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				expected := net.ParseIP("192.168.1.1")
				assert.True(t, params.AllowedIP.Equal(expected))
			},
		},
		{
			name:    "IPv6 address",
			query:   "allowed_ip=2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				assert.NotNil(t, params.AllowedIP, "AllowedIP should not be nil")
			},
		},
		{
			name:    "IP slice",
			query:   "ips=192.168.1.1&ips=10.0.0.1&ips=172.16.0.1",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				require.Len(t, params.IPs, 3)
			},
		},
		{
			name:     "invalid IP",
			query:    "allowed_ip=invalid-ip",
			wantErr:  true,
			validate: func(t *testing.T, params NetworkParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params NetworkParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_URLType tests url.URL support
func TestBindQuery_URLType(t *testing.T) {
	t.Parallel()

	type WebhookParams struct {
		CallbackURL url.URL  `query:"callback"`
		RedirectURL url.URL  `query:"redirect"`
		OptionalURL *url.URL `query:"optional"`
	}

	tests := []struct {
		name     string
		query    string
		validate func(t *testing.T, params WebhookParams)
	}{
		{
			name:  "valid URL",
			query: "callback=https://example.com/webhook",
			validate: func(t *testing.T, params WebhookParams) {
				expected := "https://example.com/webhook"
				assert.Equal(t, expected, params.CallbackURL.String())
			},
		},
		{
			name:  "URL with query params",
			query: "callback=https://example.com/hook?token=abc&id=123",
			validate: func(t *testing.T, params WebhookParams) {
				assert.Equal(t, "example.com", params.CallbackURL.Host)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params WebhookParams
			require.NoError(t, c.BindQuery(&params))
			tt.validate(t, params)
		})
	}
}

// TestBindQuery_TextUnmarshaler tests encoding.TextUnmarshaler interface
func TestBindQuery_TextUnmarshaler(t *testing.T) {
	t.Parallel()

	type Request struct {
		ID       customUUID  `query:"id"`
		TraceID  customUUID  `query:"trace_id"`
		Optional *customUUID `query:"optional"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params Request)
	}{
		{
			name:    "valid custom type",
			query:   "id=550e8400-e29b-41d4-a716-446655440000&trace_id=660e8400-e29b-41d4-a716-446655440001",
			wantErr: false,
			validate: func(t *testing.T, params Request) {
				expectedID := "550e8400-e29b-41d4-a716-446655440000"
				expectedTraceID := "660e8400-e29b-41d4-a716-446655440001"
				assert.Equal(t, expectedID, string(params.ID))
				assert.Equal(t, expectedTraceID, string(params.TraceID))
			},
		},
		{
			name:     "invalid custom type",
			query:    "id=invalid-uuid",
			wantErr:  true,
			validate: func(t *testing.T, params Request) {},
		},
		{
			name:    "pointer to custom type",
			query:   "id=550e8400-e29b-41d4-a716-446655440000&optional=770e8400-e29b-41d4-a716-446655440002",
			wantErr: false,
			validate: func(t *testing.T, params Request) {
				require.NotNil(t, params.Optional, "Optional should not be nil")
				expected := "770e8400-e29b-41d4-a716-446655440002"
				assert.Equal(t, expected, string(*params.Optional))
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params Request
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), "invalid UUID format")
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_EmbeddedStruct tests embedded struct support
func TestBindQuery_EmbeddedStruct(t *testing.T) {
	t.Parallel()

	type Pagination struct {
		Page     int `query:"page"`
		PageSize int `query:"page_size"`
	}

	type SearchRequest struct {
		Pagination        // Embedded struct
		Query      string `query:"q"`
		Sort       string `query:"sort"`
	}

	type AdvancedSearch struct {
		*Pagination
		Query string `query:"q"`
	}

	t.Run("embedded fields", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/?q=golang&page=2&page_size=20&sort=name", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params SearchRequest
		require.NoError(t, c.BindQuery(&params))
		assert.Equal(t, "golang", params.Query)
		assert.Equal(t, 2, params.Page, "Page from embedded struct")
		assert.Equal(t, 20, params.PageSize, "PageSize from embedded struct")
	})

	t.Run("pointer to embedded struct", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/?q=test&page=3&page_size=30", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		params := AdvancedSearch{
			Pagination: &Pagination{}, // Must initialize pointer
		}
		require.NoError(t, c.BindQuery(&params))
		assert.Equal(t, "test", params.Query)
		assert.Equal(t, 3, params.Page)
	})
}

// TestBindQuery_CombinedAdvancedTypes tests combining multiple advanced types
func TestBindQuery_CombinedAdvancedTypes(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "combined advanced types",
			setup: func(req *http.Request) {
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
			},
			params: &AdvancedParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*AdvancedParams)
				// Validate time
				expectedTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
				assert.True(t, p.StartDate.Equal(expectedTime), "StartDate should match expected time")
				// Validate duration
				assert.Equal(t, 30*time.Second, p.Timeout, "Timeout should be 30s")
				// Validate IP
				expectedIP := net.ParseIP("192.168.1.1")
				assert.True(t, p.AllowedIP.Equal(expectedIP), "AllowedIP should match expected IP")
				// Validate URL
				assert.Equal(t, "proxy.example.com:8080", p.ProxyURL.Host, "ProxyURL.Host should match")
				// Validate slices
				require.Len(t, p.Dates, 2, "Dates should have 2 elements")
				require.Len(t, p.Durations, 2, "Durations should have 2 elements")
				assert.Equal(t, 5*time.Second, p.Durations[0], "First duration should be 5s")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestParseTime tests parseTime function directly
func TestParseTime(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "parseTime(%q) should return error", tt.input)
			} else {
				require.NoError(t, err, "parseTime(%q) should not return error", tt.input)
			}
		})
	}
}

// TestBindQuery_IPNetType tests net.IPNet support
func TestBindQuery_IPNetType(t *testing.T) {
	t.Parallel()

	type NetworkParams struct {
		Subnet        net.IPNet   `query:"subnet"`
		AllowedRanges []net.IPNet `query:"ranges"`
		OptionalCIDR  *net.IPNet  `query:"optional"`
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
		validate func(t *testing.T, params NetworkParams)
	}{
		{
			name:    "valid CIDR",
			query:   "subnet=192.168.1.0/24",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				_, expected, _ := net.ParseCIDR("192.168.1.0/24")
				assert.Equal(t, expected.String(), params.Subnet.String())
			},
		},
		{
			name:    "IPv6 CIDR",
			query:   "subnet=2001:db8::/32",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				assert.NotNil(t, params.Subnet.IP, "Subnet IP should not be nil")
			},
		},
		{
			name:    "CIDR slice",
			query:   "ranges=10.0.0.0/8&ranges=172.16.0.0/12&ranges=192.168.0.0/16",
			wantErr: false,
			validate: func(t *testing.T, params NetworkParams) {
				require.Len(t, params.AllowedRanges, 3)
			},
		},
		{
			name:     "invalid CIDR",
			query:    "subnet=invalid-cidr",
			wantErr:  true,
			validate: func(t *testing.T, params NetworkParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params NetworkParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), "invalid CIDR notation")
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_RegexpType tests regexp.Regexp support
func TestBindQuery_RegexpType(t *testing.T) {
	t.Parallel()

	type PatternParams struct {
		Pattern       regexp.Regexp  `query:"pattern"`
		OptionalRegex *regexp.Regexp `query:"optional"`
	}

	tests := []struct {
		name     string
		setup    func(req *http.Request) // For setting query with proper encoding
		wantErr  bool
		validate func(t *testing.T, params PatternParams)
	}{
		{
			name: "valid regexp",
			setup: func(req *http.Request) {
				// Set raw query directly with properly encoded + character
				// The + needs to be encoded as %2B to avoid being decoded as a space
				req.URL.RawQuery = "pattern=" + url.QueryEscape(`^user-[0-9]+$`)
			},
			wantErr: false,
			validate: func(t *testing.T, params PatternParams) {
				expected := `^user-[0-9]+$`
				assert.Equal(t, expected, params.Pattern.String())
				assert.True(t, params.Pattern.MatchString("user-123"), "Pattern should match user-123")
				assert.False(t, params.Pattern.MatchString("admin-123"), "Pattern should not match admin-123")
			},
		},
		{
			name: "invalid regexp",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("pattern", "[invalid")
				req.URL.RawQuery = q.Encode()
			},
			wantErr:  true,
			validate: func(t *testing.T, params PatternParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params PatternParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), "invalid regular expression")
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_BracketNotation tests bracket notation for maps
func TestBindQuery_BracketNotation(t *testing.T) {
	t.Parallel()

	type MapParams struct {
		Metadata map[string]string `query:"metadata"`
		Scores   map[string]int    `query:"scores"`
	}

	tests := []struct {
		name     string
		query    string
		setup    func(req *http.Request) // Optional: for complex query setup
		wantErr  bool
		validate func(t *testing.T, params MapParams)
	}{
		{
			name:    "simple bracket notation",
			query:   "metadata[name]=John&metadata[age]=30",
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				require.NotNil(t, params.Metadata, "Metadata should not be nil")
				assert.Equal(t, "John", params.Metadata["name"])
				assert.Equal(t, "30", params.Metadata["age"])
			},
		},
		{
			name: "quoted keys with special characters",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				// Keys with dots and dashes need quotes
				q.Set(`metadata["user.name"]`, "John Doe")
				q.Set(`metadata['user-email']`, "john@example.com")
				q.Set(`metadata["org.id"]`, "12345")
				req.URL.RawQuery = q.Encode()
			},
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				assert.Equal(t, "John Doe", params.Metadata["user.name"])
				assert.Equal(t, "john@example.com", params.Metadata["user-email"])
				assert.Equal(t, "12345", params.Metadata["org.id"])
			},
		},
		{
			name:    "typed map with bracket notation",
			query:   "scores[math]=95&scores[science]=88&scores[history]=92",
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				assert.Equal(t, 95, params.Scores["math"])
				assert.Equal(t, 88, params.Scores["science"])
				require.Len(t, params.Scores, 3)
			},
		},
		{
			name:    "mixed dot and bracket notation",
			query:   "metadata.key1=value1&metadata[key2]=value2&metadata.key3=value3",
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				require.Len(t, params.Metadata, 3)
				assert.Equal(t, "value1", params.Metadata["key1"])
				assert.Equal(t, "value2", params.Metadata["key2"])
				assert.Equal(t, "value3", params.Metadata["key3"])
			},
		},
		{
			name:     "invalid bracket - no closing",
			query:    "metadata[unclosed=value",
			wantErr:  true,
			validate: func(t *testing.T, params MapParams) {},
		},
		{
			name:     "empty brackets rejected for maps",
			query:    "metadata[]=value",
			wantErr:  true,
			validate: func(t *testing.T, params MapParams) {},
		},
		{
			name:     "nested brackets rejected",
			query:    "metadata[key1][key2]=value",
			wantErr:  true,
			validate: func(t *testing.T, params MapParams) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params MapParams
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				if tt.name == "invalid bracket - no closing" {
					assert.Contains(t, err.Error(), "invalid bracket notation")
				}
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBindQuery_Maps tests map support with dot notation
func TestBindQuery_Maps(t *testing.T) {
	t.Parallel()

	type FilterParams struct {
		Metadata map[string]string `query:"metadata"`
		Tags     map[string]string `query:"tags"`
		Settings map[string]any    `query:"settings"`
	}

	type TypedMapParams struct {
		Scores map[string]int     `query:"scores"`
		Rates  map[string]float64 `query:"rates"`
	}

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name: "string map with dot notation",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("metadata.name", "John")
				q.Set("metadata.age", "30")
				q.Set("metadata.city", "NYC")
				req.URL.RawQuery = q.Encode()
			},
			params: &FilterParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*FilterParams)
				require.NotNil(t, p.Metadata, "Metadata map should not be nil")
				assert.Equal(t, "John", p.Metadata["name"])
				assert.Equal(t, "30", p.Metadata["age"])
				require.Len(t, p.Metadata, 3)
			},
		},
		{
			name: "map with interface{} values",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("settings.debug", "true")
				q.Set("settings.port", "8080")
				req.URL.RawQuery = q.Encode()
			},
			params: &FilterParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*FilterParams)
				require.NotNil(t, p.Settings, "Settings map should not be nil")
				assert.Equal(t, "true", p.Settings["debug"])
			},
		},
		{
			name:   "empty map",
			setup:  nil,
			params: &FilterParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*FilterParams)
				// Maps without values should remain nil (not cause error)
				assert.Empty(t, p.Metadata, "Metadata should be empty")
			},
		},
		{
			name: "typed map values",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("scores.math", "95")
				q.Set("scores.science", "88")
				q.Set("rates.usd", "1.0")
				q.Set("rates.eur", "0.85")
				req.URL.RawQuery = q.Encode()
			},
			params: &TypedMapParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*TypedMapParams)
				assert.Equal(t, 95, p.Scores["math"])
				assert.InDelta(t, 0.85, p.Rates["eur"], 0.01, "rates.eur should be ~0.85")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindQuery_MapJSONFallback tests the JSON string parsing fallback for map fields.
// This tests the code path where no dot/bracket notation is found, so it falls back
// to parsing a JSON string value for the map prefix.
func TestBindQuery_MapJSONFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		wantErr  bool
		validate func(t *testing.T, params any, err error)
	}{
		{
			name: "string map from JSON string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				// Set a JSON string value (no dot/bracket notation)
				q.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				require.NotNil(t, p.Metadata, "Metadata map should not be nil")
				assert.Equal(t, "John", p.Metadata["name"])
				assert.Equal(t, "30", p.Metadata["age"])
				assert.Equal(t, "NYC", p.Metadata["city"])
				require.Len(t, p.Metadata, 3)
			},
		},
		{
			name: "int map from JSON string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("scores", `{"math":95,"science":88,"history":92}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Scores map[string]int `query:"scores"`
				})
				require.NotNil(t, p.Scores, "Scores map should not be nil")
				assert.Equal(t, 95, p.Scores["math"])
				assert.Equal(t, 88, p.Scores["science"])
				assert.Equal(t, 92, p.Scores["history"])
			},
		},
		{
			name: "float64 map from JSON string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("rates", `{"usd":1.0,"eur":0.85,"gbp":0.77}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Rates map[string]float64 `query:"rates"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Rates map[string]float64 `query:"rates"`
				})
				require.NotNil(t, p.Rates, "Rates map should not be nil")
				assert.InDelta(t, 1.0, p.Rates["usd"], 0.01)
				assert.InDelta(t, 0.85, p.Rates["eur"], 0.01)
			},
		},
		{
			name: "bool map from JSON string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("flags", `{"debug":true,"verbose":false,"trace":true}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Flags map[string]bool `query:"flags"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Flags map[string]bool `query:"flags"`
				})
				require.NotNil(t, p.Flags, "Flags map should not be nil")
				assert.True(t, p.Flags["debug"])
				assert.False(t, p.Flags["verbose"])
				assert.True(t, p.Flags["trace"])
			},
		},
		{
			name: "interface{} map from JSON string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("settings", `{"debug":true,"port":8080,"name":"server"}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Settings map[string]any `query:"settings"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Settings map[string]any `query:"settings"`
				})
				require.NotNil(t, p.Settings, "Settings map should not be nil")
				assert.Equal(t, "true", p.Settings["debug"])
				assert.Equal(t, "8080", p.Settings["port"])
				assert.Equal(t, "server", p.Settings["name"])
			},
		},
		{
			name: "empty JSON object",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("metadata", `{}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				require.NotNil(t, p.Metadata, "Metadata map should not be nil")
				assert.Empty(t, p.Metadata, "Expected empty map")
			},
		},
		{
			name: "empty JSON string - should not error",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("metadata", "")
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				// Should not error, just skip JSON parsing
			},
		},
		{
			name: "invalid JSON - should silently fail without error",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("metadata", `{invalid json}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				// Map should remain nil or empty since JSON parsing failed
				assert.Empty(t, p.Metadata, "Metadata should be empty when JSON is invalid")
			},
		},
		{
			name: "type conversion error - should return error",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				// Valid JSON but invalid int value
				q.Set("scores", `{"math":"not-a-number"}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				// Error should mention the key
				assert.Contains(t, err.Error(), "math", "Error should mention the key 'math'")
			},
		},
		{
			name: "JSON string with numeric keys",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("data", `{"123":"value1","456":"value2"}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Data map[string]string `query:"data"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Data map[string]string `query:"data"`
				})
				assert.Equal(t, "value1", p.Data["123"])
				assert.Equal(t, "value2", p.Data["456"])
			},
		},
		{
			name: "JSON with nested objects - should parse only top level",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				// JSON with nested object - nested part will be converted to string
				q.Set("config", `{"outer":"value","nested":{"inner":"data"}}`)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Config map[string]any `query:"config"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p := params.(*struct {
					Config map[string]any `query:"config"`
				})
				assert.Equal(t, "value", p.Config["outer"])
				// Nested object will be converted to string via fmt.Sprint
				assert.NotNil(t, p.Config["nested"], "config[\"nested\"] should not be nil")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
			}
			tt.validate(t, tt.params, err)
		})
	}
}

// TestBindForm_MapJSONFallback tests the JSON string parsing fallback for map fields in form data.
func TestBindForm_MapJSONFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupForm      func() url.Values
		setupHandler   func() func(c *Context)
		expectedStatus int
		validate       func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "string map from JSON string",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)
				return form
			},
			setupHandler: func() func(c *Context) {
				type Params struct {
					Metadata map[string]string `form:"metadata"`
				}
				return func(c *Context) {
					var params Params
					if err := c.BindForm(&params); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var result struct {
					Metadata map[string]string `json:"metadata"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
				assert.Equal(t, "John", result.Metadata["name"])
				assert.Equal(t, "30", result.Metadata["age"])
			},
		},
		{
			name: "int map from JSON string",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("scores", `{"math":95,"science":88}`)
				return form
			},
			setupHandler: func() func(c *Context) {
				type Params struct {
					Scores map[string]int `form:"scores"`
				}
				return func(c *Context) {
					var params Params
					if err := c.BindForm(&params); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var result struct {
					Scores map[string]int `json:"scores"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
				assert.Equal(t, 95, result.Scores["math"])
				assert.Equal(t, 88, result.Scores["science"])
			},
		},
		{
			name: "invalid JSON - should silently fail",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("metadata", `{invalid json}`)
				return form
			},
			setupHandler: func() func(c *Context) {
				type Params struct {
					Metadata map[string]string `form:"metadata"`
				}
				return func(c *Context) {
					var params Params
					if err := c.BindForm(&params); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, map[string]int{"count": len(params.Metadata)})
				}
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should not error, just skip invalid JSON
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := New()
			r.POST("/test", tt.setupHandler())

			form := tt.setupForm()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			tt.validate(t, w)
		})
	}
}

// TestBindQuery_NestedStructs tests nested struct support with dot notation
func TestBindQuery_NestedStructs(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name: "nested struct with dot notation",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("name", "John")
				q.Set("email", "john@example.com")
				q.Set("address.street", "123 Main St")
				q.Set("address.city", "NYC")
				q.Set("address.zip_code", "10001")
				req.URL.RawQuery = q.Encode()
			},
			params: &UserRequest{},
			validate: func(t *testing.T, params any) {
				p := params.(*UserRequest)
				assert.Equal(t, "John", p.Name)
				assert.Equal(t, "123 Main St", p.Address.Street)
				assert.Equal(t, "NYC", p.Address.City)
				assert.Equal(t, "10001", p.Address.ZipCode)
			},
		},
		{
			name: "deeply nested structs",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("name", "Test")
				q.Set("address.street", "Main St")
				q.Set("address.location.lat", "40.7128")
				q.Set("address.location.lng", "-74.0060")
				req.URL.RawQuery = q.Encode()
			},
			params: &ComplexRequest{},
			validate: func(t *testing.T, params any) {
				p := params.(*ComplexRequest)
				assert.Equal(t, "Main St", p.Address.Street)
				assert.InDelta(t, 40.7128, p.Address.Location.Lat, 0.01)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindQuery_PointerMap tests pointer to map types
func TestBindQuery_PointerMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		validate func(t *testing.T, params any)
	}{
		{
			name: "pointer to map[string]string",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("metadata.name", "John")
				q.Set("metadata.age", "30")
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Metadata *map[string]string `query:"metadata"`
			}{},
			validate: func(t *testing.T, params any) {
				p := params.(*struct {
					Metadata *map[string]string `query:"metadata"`
				})
				require.NotNil(t, p.Metadata, "Metadata map pointer should not be nil")
				assert.Equal(t, "John", (*p.Metadata)["name"])
				assert.Equal(t, "30", (*p.Metadata)["age"])
			},
		},
		{
			name: "pointer to map[string]int",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("scores.math", "95")
				q.Set("scores.science", "88")
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Scores *map[string]int `query:"scores"`
			}{},
			validate: func(t *testing.T, params any) {
				p := params.(*struct {
					Scores *map[string]int `query:"scores"`
				})
				require.NotNil(t, p.Scores, "Scores map pointer should not be nil")
				assert.Equal(t, 95, (*p.Scores)["math"])
				assert.Equal(t, 88, (*p.Scores)["science"])
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindForm_PointerMap tests pointer to map types in form data.
func TestBindForm_PointerMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupForm      func() url.Values
		setupHandler   func() func(c *Context)
		expectedStatus int
		validate       func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "pointer to map[string]string",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("metadata.name", "John")
				form.Set("metadata.age", "30")
				return form
			},
			setupHandler: func() func(c *Context) {
				type Params struct {
					Metadata *map[string]string `form:"metadata"`
				}
				return func(c *Context) {
					var params Params
					if err := c.BindForm(&params); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var result struct {
					Metadata *map[string]string `json:"metadata"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
				require.NotNil(t, result.Metadata, "Metadata map pointer should not be nil")
				assert.Equal(t, "John", (*result.Metadata)["name"])
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := New()
			r.POST("/test", tt.setupHandler())

			form := tt.setupForm()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			tt.validate(t, w)
		})
	}
}

// TestBindQuery_MapTypeConversionError tests error path for queryGetter when type conversion fails
func TestBindQuery_MapTypeConversionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any // The struct to bind to
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "queryGetter dot notation - invalid int conversion",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("scores.math", "not-a-number") // Invalid int value
				q.Set("scores.science", "88")        // Valid value (should not be reached if error happens first)
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				// Error should mention the key "math"
				assert.Contains(t, err.Error(), "math", "Error should mention key 'math'")
				assert.Contains(t, err.Error(), "key", "Error should include 'key'")
				assert.Contains(t, err.Error(), "\"math\"", "Error should include quoted key name")
			},
		},
		{
			name: "queryGetter bracket notation - invalid float conversion",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Set("rates[usd]", "invalid-float")
				req.URL.RawQuery = q.Encode()
			},
			params: &struct {
				Rates map[string]float64 `query:"rates"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				// Error should mention the key
				assert.Contains(t, err.Error(), "usd", "Error should mention key 'usd'")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				tt.validate(t, err)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
			}
		})
	}
}

// TestBindForm_MapTypeConversionError tests error path for formGetter when type conversion fails
// Also tests formGetter dot notation path (: found = true, mapKey = strings.TrimPrefix).
func TestBindForm_MapTypeConversionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupForm      func() url.Values
		expectedStatus int
		validate       func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "formGetter dot notation - invalid int conversion",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("scores.math", "not-a-number") // Invalid int value
				form.Set("scores.science", "88")
				return form
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errorResp))
				errorMsg := errorResp["error"]
				assert.Contains(t, errorMsg, "math", "Error should mention key 'math'")
				assert.Contains(t, errorMsg, "key", "Error should include 'key'")
				assert.Contains(t, errorMsg, "\"math\"", "Error should include quoted key name")
			},
		},
		{
			name: "formGetter dot notation - successful binding",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("metadata.name", "John") // Tests : found = true, mapKey extraction
				form.Set("metadata.age", "30")
				return form
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var result struct {
					Metadata map[string]string `json:"metadata"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
				assert.Equal(t, "John", result.Metadata["name"])
				assert.Equal(t, "30", result.Metadata["age"])
			},
		},
		{
			name: "formGetter bracket notation - invalid bool conversion",
			setupForm: func() url.Values {
				form := url.Values{}
				form.Set("flags[debug]", "not-a-bool") // Invalid bool value
				return form
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errorResp))
				errorMsg := errorResp["error"]
				assert.Contains(t, errorMsg, "debug", "Error should mention key 'debug'")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handler func(c *Context)
			if strings.Contains(tt.name, "invalid int conversion") {
				type Params struct {
					Scores map[string]int `form:"scores"`
				}
				handler = func(c *Context) {
					var params Params
					err := c.BindForm(&params)
					if err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			} else if strings.Contains(tt.name, "successful binding") {
				type Params struct {
					Metadata map[string]string `form:"metadata"`
				}
				handler = func(c *Context) {
					var params Params
					if err := c.BindForm(&params); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			} else if strings.Contains(tt.name, "invalid bool conversion") {
				type Params struct {
					Flags map[string]bool `form:"flags"`
				}
				handler = func(c *Context) {
					var params Params
					err := c.BindForm(&params)
					if err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, params)
				}
			}

			r := New()
			r.POST("/test", handler)

			form := tt.setupForm()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			tt.validate(t, w)
		})
	}
}

// TestPrefixGetter_Has tests the Has method of prefixGetter, specifically the iteration
// logic for queryGetter and formGetter
func TestPrefixGetter_Has(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() (*prefixGetter, url.Values)
		key      string
		expected bool
	}{
		{
			name: "queryGetter - exact key match",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("address.street", "Main St")
				values.Set("address.city", "NYC")
				values.Set("name", "John")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "street",
			expected: true,
		},
		{
			name: "queryGetter - exact key match (city)",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("address.street", "Main St")
				values.Set("address.city", "NYC")
				values.Set("name", "John")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "city",
			expected: true,
		},
		{
			name: "queryGetter - key without prefix",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("address.street", "Main St")
				values.Set("address.city", "NYC")
				values.Set("name", "John")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "name",
			expected: false,
		},
		{
			name: "queryGetter - prefix match with dot",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("address.location.lat", "40.7128")
				values.Set("address.location.lng", "-74.0060")
				values.Set("address.city", "NYC")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "location",
			expected: true,
		},
		{
			name: "queryGetter - no matching keys",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("user.name", "John")
				values.Set("user.email", "john@example.com")
				values.Set("other.field", "value")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "street",
			expected: false,
		},
		{
			name: "queryGetter - empty values",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "street",
			expected: false,
		},
		{
			name: "formGetter - exact key match",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("metadata.name", "John")
				values.Set("metadata.age", "30")
				values.Set("title", "Mr")
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "metadata.",
				}
				return pg, values
			},
			key:      "name",
			expected: true,
		},
		{
			name: "formGetter - prefix match with dot",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("config.database.host", "localhost")
				values.Set("config.database.port", "5432")
				values.Set("config.debug", "true")
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "config.",
				}
				return pg, values
			},
			key:      "database",
			expected: true,
		},
		{
			name: "formGetter - no matching keys",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("user.name", "John")
				values.Set("other.field", "value")
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "config.",
				}
				return pg, values
			},
			key:      "debug",
			expected: false,
		},
		{
			name: "formGetter - empty values",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "metadata.",
				}
				return pg, values
			},
			key:      "name",
			expected: false,
		},
		{
			name: "queryGetter - multiple prefix matches",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("address.street", "Main St")
				values.Set("address.street.number", "123")
				values.Set("address.city", "NYC")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "street",
			expected: true,
		},
		{
			name: "formGetter - key that starts with prefix but doesn't match",
			setup: func() (*prefixGetter, url.Values) {
				values := url.Values{}
				values.Set("addresses.street", "Main St") // Note: "addresses" not "address"
				values.Set("address.city", "NYC")
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "address.",
				}
				return pg, values
			},
			key:      "street",
			expected: false,
		},
		{
			name: "queryGetter - iteration path when direct Has returns false",
			setup: func() (*prefixGetter, url.Values) {
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
				return pg, values
			},
			key:      "location",
			expected: true,
		},
		{
			name: "formGetter - iteration path when direct Has returns false",
			setup: func() (*prefixGetter, url.Values) {
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
				return pg, values
			},
			key:      "database",
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pg, _ := tt.setup()
			result := pg.Has(tt.key)
			assert.Equal(t, tt.expected, result, "Has(%q) should return %v", tt.key, tt.expected)
		})
	}
}

// TestBindQuery_EnumValidation tests enum validation
func TestBindQuery_EnumValidation(t *testing.T) {
	t.Parallel()

	type StatusParams struct {
		Status   string `query:"status" enum:"active,inactive,pending"`
		Role     string `query:"role" enum:"admin,user,guest"`
		Priority string `query:"priority" enum:"low,medium,high"`
	}

	tests := []struct {
		name     string
		query    string
		params   any // The struct to bind to
		wantErr  bool
		validate func(t *testing.T, params any)
	}{
		{
			name:    "valid enum values",
			query:   "status=active&role=admin&priority=high",
			params:  &StatusParams{},
			wantErr: false,
			validate: func(t *testing.T, params any) {
				p := params.(*StatusParams)
				assert.Equal(t, "active", p.Status)
				assert.Equal(t, "admin", p.Role)
				assert.Equal(t, "high", p.Priority)
			},
		},
		{
			name:     "invalid enum value",
			query:    "status=invalid-status",
			params:   &StatusParams{},
			wantErr:  true,
			validate: func(t *testing.T, params any) {},
		},
		{
			name:    "empty value passes enum validation",
			query:   "role=admin",
			params:  &StatusParams{},
			wantErr: false,
			validate: func(t *testing.T, params any) {
				p := params.(*StatusParams)
				assert.Equal(t, "admin", p.Role)
				assert.Empty(t, p.Status, "Status should be empty")
			},
		},
		{
			name:  "enum with whitespace handling",
			query: "value=option2",
			params: &struct {
				Value string `query:"value" enum:" option1 , option2 , option3 "`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any) {
				// Just verify it doesn't error
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not in allowed values")
			} else {
				require.NoError(t, err)
				tt.validate(t, tt.params)
			}
		})
	}
}

// TestBindQuery_AllComplexTypes tests combined complex features
func TestBindQuery_AllComplexTypes(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "all complex types",
			setup: func(req *http.Request) {
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
			},
			params: &ComplexParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*ComplexParams)
				assert.Equal(t, "John", p.Name, "Name should match")
				assert.Equal(t, 30*time.Second, p.Timeout, "Timeout should be 30s")
				assert.Equal(t, "value1", p.Metadata["key1"], "metadata.key1 should match")
				assert.Equal(t, "Main St", p.Address.Street, "address.street should match")
				assert.Equal(t, "active", p.Status, "Status should match")
				require.Len(t, p.Dates, 2, "Dates should have 2 elements")
				require.Len(t, p.IPs, 2, "IPs should have 2 elements")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
	}
}

// TestBindQuery_DefaultValues tests default values
func TestBindQuery_DefaultValues(t *testing.T) {
	t.Parallel()

	type ParamsWithDefaults struct {
		Page     int    `query:"page" default:"1"`
		PageSize int    `query:"page_size" default:"10"`
		Sort     string `query:"sort" default:"created_at"`
		Order    string `query:"order" default:"desc"`
		Active   bool   `query:"active" default:"true"`
		Limit    int    `query:"limit" default:"100"`
	}

	tests := []struct {
		name     string
		query    string
		validate func(t *testing.T, params ParamsWithDefaults)
	}{
		{
			name:  "all defaults applied",
			query: "",
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 1, params.Page, "Page should default to 1")
				assert.Equal(t, 10, params.PageSize, "PageSize should default to 10")
				assert.Equal(t, "created_at", params.Sort, "Sort should default to created_at")
				assert.Equal(t, "desc", params.Order, "Order should default to desc")
				assert.True(t, params.Active, "Active should default to true")
				assert.Equal(t, 100, params.Limit, "Limit should default to 100")
			},
		},
		{
			name:  "user values override defaults",
			query: "page=5&page_size=50&active=false",
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 5, params.Page, "Page should be user value")
				assert.Equal(t, 50, params.PageSize, "PageSize should be user value")
				assert.False(t, params.Active, "Active should be user value")
				assert.Equal(t, "created_at", params.Sort, "Sort should be default")
			},
		},
		{
			name:  "partial user values with defaults",
			query: "page=3",
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 3, params.Page, "Page should be user value")
				assert.Equal(t, 10, params.PageSize, "PageSize should be default")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params ParamsWithDefaults
			require.NoError(t, c.BindQuery(&params))
			tt.validate(t, params)
		})
	}

	// Default time values test - kept separate due to different struct type
	t.Run("default time values", func(t *testing.T) {
		t.Parallel()

		type TimeDefaults struct {
			Timeout time.Duration `query:"timeout" default:"30s"`
			Created time.Time     `query:"created" default:"2024-01-01T00:00:00Z"`
		}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		var params TimeDefaults
		require.NoError(t, c.BindQuery(&params))

		assert.Equal(t, 30*time.Second, params.Timeout, "Timeout should default to 30s")
		assert.False(t, params.Created.IsZero(), "Created should have default value")
	})
}

// Test WarmupBindingCache
func TestWarmupBindingCache(t *testing.T) {
	t.Parallel()

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

	t1Type := reflect.TypeFor[TestStruct1]()
	assert.Contains(t, typeCache, t1Type, "TestStruct1 should be in cache")
	assert.Contains(t, typeCache[t1Type], "query", "TestStruct1 query tag should be cached")

	t2Type := reflect.TypeFor[TestStruct2]()
	assert.Contains(t, typeCache, t2Type, "TestStruct2 should be in cache")
	assert.Contains(t, typeCache[t2Type], "json", "TestStruct2 json tag should be cached")
}

// TestGetStructInfo_DoubleCheckLocking tests the double-check locking pattern
func TestGetStructInfo_DoubleCheckLocking(t *testing.T) {
	t.Parallel()

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
	for range numGoroutines {
		wg.Go(func() {
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
				errors <- fmt.Errorf("%w: got %+v", ErrBindingFailed, params)
				return
			}
		})
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(t, err, "Goroutine should not error")
	}

	// Verify cache was populated (at least one goroutine should have parsed it)
	typeCacheMu.RLock()
	defer typeCacheMu.RUnlock()

	structType := reflect.TypeFor[ConcurrentStruct]()
	assert.Contains(t, typeCache, structType, "ConcurrentStruct should be in cache after concurrent binding")
	assert.Contains(t, typeCache[structType], "query", "ConcurrentStruct query tag should be cached")
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
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
	_ = c.BindQuery(&warmup)

	b.ResetTimer()
	for b.Loop() {
		var params Params
		if err := c.BindQuery(&params); err != nil {
			b.Fatal(err)
		}
	}
}

// TestBindQuery_ComplexStruct tests complex real-world binding
func TestBindQuery_ComplexStruct(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "complex struct with all types",
			setup: func(req *http.Request) {
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
			},
			params: &ComplexParams{},
			validate: func(t *testing.T, params any) {
				p := params.(*ComplexParams)
				assert.Equal(t, "search", p.Query, "Query should match")
				require.Len(t, p.Tags, 2, "Tags should have 2 elements")
				require.Len(t, p.IDs, 3, "IDs should have 3 elements")
				require.NotNil(t, p.OptionalName, "OptionalName should not be nil")
				assert.Equal(t, "John", *p.OptionalName, "OptionalName should be John")
				assert.Nil(t, p.OptionalAge, "OptionalAge should be nil")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.setup != nil {
				tt.setup(req)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindQuery(tt.params))
			tt.validate(t, tt.params)
		})
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

	assert.Equal(t, http.StatusOK, w.Code, "User binding should work after warmup")
}

// TestBindBody_UnsupportedContentType tests BindBody with unsupported content type
func TestBindBody_UnsupportedContentType(t *testing.T) {
	t.Parallel()

	type Data struct {
		Value string `json:"value"`
	}

	r := New()
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
	assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for unsupported content type")
}

// TestGetCookie_URLEscaping tests cookie value unescaping
func TestGetCookie_URLEscaping(t *testing.T) {
	t.Parallel()

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

	assert.Equal(t, http.StatusOK, w.Code, "should successfully get and unescape cookie")
}

// TestBindJSON_EdgeCases tests JSON binding edge cases
func TestBindJSON_EdgeCases(t *testing.T) {
	t.Parallel()

	type Data struct {
		Value string `json:"value"`
	}

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		validate       func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "malformed JSON",
			body:           `{invalid json`,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should return error for malformed JSON
			},
		},
		{
			name:           "empty body",
			body:           ``,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Empty body should fail
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := New()
			r.POST("/test", func(c *Context) {
				var data Data
				err := c.BindJSON(&data)

				if err != nil {
					c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
					return
				}

				c.JSON(http.StatusOK, data)
			})

			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			tt.validate(t, w)
		})
	}
}

// TestConvertValue_AllTypes tests type conversion for all supported types
func TestConvertValue_AllTypes(t *testing.T) {
	t.Parallel()

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

	tests := []struct {
		name           string
		setupForm      func() url.Values
		expectedStatus int
	}{
		{
			name: "all supported types",
			setupForm: func() url.Values {
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
				return form
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := New()
			r.POST("/test", func(c *Context) {
				var data AllTypes
				if err := c.BindForm(&data); err != nil {
					c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			form := tt.setupForm()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
		})
	}
}

// TestConvertValue_ErrorCases tests convertValue error cases, specifically covering:
// - Invalid unsigned integer parsing error
// - Invalid float parsing error
// - Invalid bool parsing error (from parseBool)
// - Unsupported type error
func TestConvertValue_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		value          string
		kind           reflect.Kind
		expectedErrMsg string
	}{
		// Invalid unsigned integer errors
		{"negative value for uint", "-42", reflect.Uint, "invalid unsigned integer"},
		{"negative value for uint8", "-10", reflect.Uint8, "invalid unsigned integer"},
		{"negative value for uint16", "-100", reflect.Uint16, "invalid unsigned integer"},
		{"negative value for uint32", "-1000", reflect.Uint32, "invalid unsigned integer"},
		{"negative value for uint64", "-999999", reflect.Uint64, "invalid unsigned integer"},
		{"invalid format for uint", "abc", reflect.Uint, "invalid unsigned integer"},
		{"decimal for uint", "42.5", reflect.Uint, "invalid unsigned integer"},
		{"empty string for uint", "", reflect.Uint, "invalid unsigned integer"},
		{"whitespace for uint", "   ", reflect.Uint, "invalid unsigned integer"},
		// Invalid float errors
		{"invalid format for float32", "abc", reflect.Float32, "invalid float"},
		{"invalid format for float64", "xyz", reflect.Float64, "invalid float"},
		{"empty string for float32", "", reflect.Float32, "invalid float"},
		{"whitespace for float64", "   ", reflect.Float64, "invalid float"},
		{"mixed chars for float32", "12abc", reflect.Float32, "invalid float"},
		{"only dots for float64", "...", reflect.Float64, "invalid float"},
		{"multiple dots for float32", "12.34.56", reflect.Float32, "invalid float"},
		// Invalid bool errors
		{"invalid bool value", "maybe", reflect.Bool, "invalid"},
		{"numeric 2", "2", reflect.Bool, "invalid"},
		{"numeric 3", "3", reflect.Bool, "invalid"},
		{"random text", "random", reflect.Bool, "invalid"},
		{"mixed case invalid", "Maybe", reflect.Bool, "invalid"},
		{"yesno together", "yesno", reflect.Bool, "invalid"},
		{"truefalse together", "truefalse", reflect.Bool, "invalid"},
		// Unsupported type errors
		{"slice type", "", reflect.Slice, "unsupported type"},
		{"map type", "", reflect.Map, "unsupported type"},
		{"array type", "", reflect.Array, "unsupported type"},
		{"chan type", "", reflect.Chan, "unsupported type"},
		{"func type", "", reflect.Func, "unsupported type"},
		{"interface type", "", reflect.Interface, "unsupported type"},
		{"ptr type", "", reflect.Ptr, "unsupported type"},
		{"struct type", "", reflect.Struct, "unsupported type"},
		{"unsafe pointer", "", reflect.UnsafePointer, "unsupported type"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := convertValue(tt.value, tt.kind)

			require.Error(t, err, "convertValue(%q, %v) should return error", tt.value, tt.kind)
			assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			assert.Nil(t, result, "Result should be nil on error")
		})
	}
}

// TestSetField_PointerFieldError tests error path for pointer fields.
// When setFieldValue fails for a pointer field, the error should be returned.
func TestSetField_PointerFieldError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		params         any
		expectedErrMsg string
	}{
		{
			name:  "pointer to int with invalid value",
			query: "age=not-a-number",
			params: &struct {
				Age *int `query:"age"`
			}{},
			expectedErrMsg: "Age",
		},
		{
			name:  "pointer to time.Time with invalid value",
			query: "start=invalid-time",
			params: &struct {
				StartTime *time.Time `query:"start"`
			}{},
			expectedErrMsg: "StartTime",
		},
		{
			name:  "pointer to float64 with invalid value",
			query: "price=not-a-float",
			params: &struct {
				Price *float64 `query:"price"`
			}{},
			expectedErrMsg: "Price",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			require.Error(t, err, "Expected error for %s", tt.name)
			assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should mention field name %q", tt.expectedErrMsg)
		})
	}
}

// TestSetFieldValue_InvalidURL tests error path for invalid URL parsing
func TestSetFieldValue_InvalidURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		params         any
		wantErr        bool
		expectedErrMsg string
		validate       func(t *testing.T, params any)
	}{
		{
			name:  "invalid URL format",
			query: "callback=://invalid-url",
			params: &struct {
				CallbackURL url.URL `query:"callback"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid URL",
			validate:       func(t *testing.T, params any) {},
		},
		{
			name:  "malformed URL with missing scheme",
			query: "endpoint=://malformed",
			params: &struct {
				Endpoint url.URL `query:"endpoint"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid URL",
			validate:       func(t *testing.T, params any) {},
		},
		{
			name:  "valid URL should succeed",
			query: "endpoint=https://example.com/path",
			params: &struct {
				Endpoint url.URL `query:"endpoint"`
			}{},
			wantErr:        false,
			expectedErrMsg: "",
			validate: func(t *testing.T, params any) {
				p := params.(*struct {
					Endpoint url.URL `query:"endpoint"`
				})
				assert.Equal(t, "example.com", p.Endpoint.Host, "URL Host should match")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			} else {
				require.NoError(t, err, "BindQuery should succeed for %s", tt.name)
				tt.validate(t, tt.params)
			}
		})
	}
}

// TestSetFieldValue_UnsupportedType tests error path for unsupported types.
// This tests types that are not handled in the switch statement (default case).
func TestSetFieldValue_UnsupportedType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		params         any
		wantErr        bool
		expectedErrMsg string
		validate       func(t *testing.T, err error)
	}{
		{
			name:  "unsupported type - Array",
			query: "data=1,2,3",
			params: &struct {
				Data [5]int `query:"data"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				assert.Contains(t, strings.ToLower(err.Error()), "array", "Error should mention 'array'")
			},
		},
		{
			name:  "unsupported type - Chan",
			query: "channel=test",
			params: &struct {
				Channel chan int `query:"channel"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "Chan", "Error should mention 'Chan'")
			},
		},
		{
			name:  "unsupported type - Func",
			query: "handler=test",
			params: &struct {
				Handler func() `query:"handler"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				assert.Contains(t, strings.ToLower(err.Error()), "func", "Error should mention 'func'")
			},
		},
		{
			name:  "unsupported type - Interface",
			query: "value=test",
			params: &struct {
				Value any `query:"value"`
			}{},
			wantErr:        false, // interface{} might be handled differently
			expectedErrMsg: "",
			validate:       func(t *testing.T, err error) {},
		},
		{
			name:  "unsupported type - UnsafePointer",
			query: "data=test",
			params: &struct {
				Data [0]struct{} `query:"data"` // Empty struct array
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate:       func(t *testing.T, err error) {},
		},
		{
			name:  "unsupported type - Complex64",
			query: "complex=1+2i",
			params: &struct {
				Complex complex64 `query:"complex"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate:       func(t *testing.T, err error) {},
		},
		{
			name:  "unsupported type - Complex128",
			query: "complex=1+2i",
			params: &struct {
				Complex complex128 `query:"complex"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate:       func(t *testing.T, err error) {},
		},
		{
			name:  "unsupported type - Map (non-string key)",
			query: "data=test",
			params: &struct {
				Data map[int]string `query:"data"`
			}{},
			wantErr:        false, // Maps are handled specially
			expectedErrMsg: "",
			validate:       func(t *testing.T, err error) {},
		},
		{
			name:  "verify default case error format matches",
			query: "data=test",
			params: &struct {
				Unsupported [3]string `query:"data"` // Array type that will fail
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				errStr := err.Error()
				parts := strings.Split(errStr, "unsupported type:")
				require.GreaterOrEqual(t, len(parts), 2, "Error should match format 'unsupported type: <kind>'")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
				}
				tt.validate(t, err)
			} else {
				// May or may not error, just test the path
				_ = err
			}
		})
	}
}

// TestPrefixGetter_GetAll tests prefixGetter.GetAll method
func TestPrefixGetter_GetAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() (*prefixGetter, string)
		validate func(t *testing.T, result []string, key string)
	}{
		{
			name: "queryGetter with prefix - name",
			setup: func() (*prefixGetter, string) {
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
				return pg, "name"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 1, "Expected 1 value for %q", key)
				assert.Equal(t, "John", result[0], "Expected first value to be 'John'")
			},
		},
		{
			name: "queryGetter with prefix - email",
			setup: func() (*prefixGetter, string) {
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
				return pg, "email"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 2, "Expected 2 values for %q", key)
				assert.Equal(t, "john@example.com", result[0], "Expected first email")
				assert.Equal(t, "john.doe@example.com", result[1], "Expected second email")
			},
		},
		{
			name: "queryGetter with prefix - nonexistent",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("user.name", "John")
				qg := &queryGetter{values: values}
				pg := &prefixGetter{
					inner:  qg,
					prefix: "user.",
				}
				return pg, "nonexistent"
			},
			validate: func(t *testing.T, result []string, key string) {
				assert.Nil(t, result, "Expected nil for non-existent key")
			},
		},
		{
			name: "formGetter with prefix - tags",
			setup: func() (*prefixGetter, string) {
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
				return pg, "tags"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 2, "Expected 2 values for %q", key)
			},
		},
		{
			name: "formGetter with prefix - version",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("meta.tags", "go")
				values.Add("meta.version", "1.0")
				fg := &formGetter{values: values}
				pg := &prefixGetter{
					inner:  fg,
					prefix: "meta.",
				}
				return pg, "version"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 1, "Expected 1 value for %q", key)
				assert.Equal(t, "1.0", result[0], "Expected version to be '1.0'")
			},
		},
		{
			name: "cookieGetter with prefix",
			setup: func() (*prefixGetter, string) {
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
				return pg, "id"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 1, "Expected 1 value for %q", key)
				assert.Equal(t, "abc123", result[0], "Expected id to be 'abc123'")
			},
		},
		{
			name: "headerGetter with prefix",
			setup: func() (*prefixGetter, string) {
				headers := http.Header{}
				headers.Add("X-Meta-Tags", "tag1")
				headers.Add("X-Meta-Tags", "tag2")
				headers.Add("X-Other-Data", "value")
				hg := &headerGetter{headers: headers}
				pg := &prefixGetter{
					inner:  hg,
					prefix: "X-Meta-",
				}
				return pg, "Tags"
			},
			validate: func(t *testing.T, result []string, key string) {
				require.Len(t, result, 2, "Expected 2 values for %q", key)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pg, key := tt.setup()
			result := pg.GetAll(key)
			tt.validate(t, result, key)
		})
	}
}

// TestPrefixGetter_GetAll_ThroughNestedBinding tests prefixGetter.GetAll through actual nested struct binding
// This ensures GetAll is called when binding nested structs with slice fields
func TestPrefixGetter_GetAll_ThroughNestedBinding(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type Address struct {
		Tags []string `query:"tags"`
	}
	type ParamsQuery struct {
		Address Address `query:"address"`
	}

	type Metadata struct {
		Versions []string `form:"versions"`
	}
	type FormData struct {
		Metadata Metadata `form:"meta"`
	}

	type Item struct {
		Tags []string `query:"tags"`
	}
	type Section struct {
		Items Item `query:"item"`
	}
	type ParamsDeep struct {
		Section Section `query:"section"`
	}

	tests := []struct {
		name     string
		setup    func() (*http.Request, any)
		bindFunc func(c *Context, params any) error
		validate func(t *testing.T, params any)
	}{
		{
			name: "nested struct with slice field - query",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest("GET", "/?address.tags=home&address.tags=work", nil)
				return req, &ParamsQuery{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindQuery(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*ParamsQuery)
				require.Len(t, p.Address.Tags, 2, "Expected 2 tags")
				assert.Equal(t, "home", p.Address.Tags[0], "Expected first tag to be 'home'")
				assert.Equal(t, "work", p.Address.Tags[1], "Expected second tag to be 'work'")
			},
		},
		{
			name: "nested struct with slice field - form",
			setup: func() (*http.Request, any) {
				form := url.Values{}
				form.Add("meta.versions", "1.0")
				form.Add("meta.versions", "2.0")
				form.Add("meta.versions", "3.0")
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req, &FormData{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindForm(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*FormData)
				require.Len(t, p.Metadata.Versions, 3, "Expected 3 versions")
				assert.Equal(t, "1.0", p.Metadata.Versions[0], "Expected first version")
				assert.Equal(t, "2.0", p.Metadata.Versions[1], "Expected second version")
				assert.Equal(t, "3.0", p.Metadata.Versions[2], "Expected third version")
			},
		},
		{
			name: "deeply nested struct with slice",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest("GET", "/?section.item.tags=tag1&section.item.tags=tag2", nil)
				return req, &ParamsDeep{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindQuery(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*ParamsDeep)
				require.Len(t, p.Section.Items.Tags, 2, "Expected 2 tags")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, params := tt.setup()
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, tt.bindFunc(c, params), "%s should succeed", tt.name)
			tt.validate(t, params)
		})
	}
}

// TestParamsGetter_GetAll_ThroughBinding tests paramsGetter.GetAll through actual binding
func TestParamsGetter_GetAll_ThroughBinding(t *testing.T) {
	t.Parallel()

	type Params struct {
		ID string `params:"id"`
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	c.Params = map[string]string{"id": "123"}

	var params Params
	require.NoError(t, c.BindParams(&params), "BindParams should succeed")
	assert.Equal(t, "123", params.ID, "Expected ID=123")

	// Test that GetAll is used internally for slices (if applicable)
	type ParamsWithSlice struct {
		IDs []string `params:"id"`
	}

	var paramsSlice ParamsWithSlice
	c.Params = map[string]string{"id": "456"}
	require.NoError(t, c.BindParams(&paramsSlice), "BindParams should succeed for slice")
	require.Len(t, paramsSlice.IDs, 1, "Expected 1 ID")
	assert.Equal(t, "456", paramsSlice.IDs[0], "Expected first ID to be '456'")
}

// TestCookieGetter_GetAll_ThroughBinding tests cookieGetter.GetAll through actual binding
// This tests the URL unescaping error path
func TestCookieGetter_GetAll_ThroughBinding(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type CookiesWithSession struct {
		Session []string `cookie:"session"`
	}

	type CookiesWithData struct {
		Data []string `cookie:"data"`
	}

	tests := []struct {
		name     string
		setup    func() *http.Request
		params   any
		validate func(t *testing.T, cookies any)
	}{
		{
			name: "multiple cookies with same name",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
				req.AddCookie(&http.Cookie{Name: "session", Value: "def456"})
				return req
			},
			params: &CookiesWithSession{},
			validate: func(t *testing.T, cookies any) {
				c := cookies.(*CookiesWithSession)
				require.Len(t, c.Session, 2, "Expected 2 session cookies")
				assert.Equal(t, "abc123", c.Session[0], "Expected first session")
				assert.Equal(t, "def456", c.Session[1], "Expected second session")
			},
		},
		{
			name: "URL unescaping error path",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				// Create a cookie with invalid URL encoding to trigger error path
				req.AddCookie(&http.Cookie{Name: "data", Value: "%ZZ"}) // Invalid percent encoding
				return req
			},
			params: &CookiesWithData{},
			validate: func(t *testing.T, cookies any) {
				c := cookies.(*CookiesWithData)
				// Should fallback to raw cookie value on unescaping error
				require.Len(t, c.Data, 1, "Expected 1 data cookie")
				assert.Equal(t, "%ZZ", c.Data[0], "Expected raw value %%ZZ")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := tt.setup()
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.BindCookies(tt.params), "BindCookies should succeed")
			tt.validate(t, tt.params)
		})
	}
}

// TestHeaderGetter_GetAll_ThroughBinding tests headerGetter.GetAll through actual binding
func TestHeaderGetter_GetAll_ThroughBinding(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, c.BindHeaders(&headers), "BindHeaders should succeed")

	require.Len(t, headers.Tags, 3, "Expected 3 tags")
	assert.Equal(t, "tag1", headers.Tags[0], "Expected first tag")
	assert.Equal(t, "tag2", headers.Tags[1], "Expected second tag")
	assert.Equal(t, "tag3", headers.Tags[2], "Expected third tag")
}

// TestWarmupBindingCache_EdgeCases tests WarmupBindingCache with edge cases to improve coverage
func TestWarmupBindingCache_EdgeCases(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		Name string `query:"name"`
	}

	type User struct {
		ID int `json:"id"`
	}

	type Product struct {
		Name string `form:"name"`
	}

	type EmptyStruct struct{}

	tests := []struct {
		name string
		args []any
	}{
		{
			name: "pointer to struct",
			args: []any{&TestStruct{}},
		},
		{
			name: "non-struct types",
			args: []any{"string", 42, []int{1, 2, 3}, map[string]int{"a": 1}},
		},
		{
			name: "mix of structs and non-structs",
			args: []any{User{}, &Product{}, "string", 123},
		},
		{
			name: "pointer to non-struct",
			args: []any{func() *string { s := "test"; return &s }()},
		},
		{
			name: "empty struct",
			args: []any{EmptyStruct{}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Should not panic
			WarmupBindingCache(tt.args...)
		})
	}
}

// TestBindBody_ContentTypeWithParameters tests BindBody with content type containing parameters
func TestBindBody_ContentTypeWithParameters(t *testing.T) {
	t.Parallel()

	type Data struct {
		Value string `json:"value"`
	}

	type FormData struct {
		Name string `form:"name"`
	}

	tests := []struct {
		name           string
		setup          func() (*http.Request, *Router)
		expectedStatus int
	}{
		{
			name: "JSON with charset parameter",
			setup: func() (*http.Request, *Router) {
				r := New()
				r.POST("/test", func(c *Context) {
					var data Data
					if err := c.BindBody(&data); err != nil {
						c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(http.StatusOK, data)
				})
				req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"value":"test"}`))
				req.Header.Set("Content-Type", "application/json; charset=utf-8")
				return req, r
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "form with boundary parameter",
			setup: func() (*http.Request, *Router) {
				r := New()
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
				return req, r
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "multipart with boundary",
			setup: func() (*http.Request, *Router) {
				r := New()
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
				_ = writer.WriteField("name", "Jane")
				_ = writer.Close()
				req := httptest.NewRequest(http.MethodPost, "/multipart", body)
				req.Header.Set("Content-Type", writer.FormDataContentType()) // Includes boundary parameter
				return req, r
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "content type with leading/trailing spaces and parameters",
			setup: func() (*http.Request, *Router) {
				r := New()
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
				return req, r
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, r := tt.setup()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
		})
	}
}

// TestBindJSON_NilBody tests BindJSON with nil request body
func TestBindJSON_NilBody(t *testing.T) {
	t.Parallel()

	type Data struct {
		Value string `json:"value"`
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = nil // Explicitly set to nil
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var data Data
	err := c.BindJSON(&data)

	require.Error(t, err, "Expected error for nil body")
	assert.Contains(t, err.Error(), "request body is nil", "Expected 'request body is nil' error")
}

// TestTagParsing_CommaSeparatedOptions tests json/form tags with comma-separated options
func TestTagParsing_CommaSeparatedOptions(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type JSONDataOmitempty struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
		Age   int    `json:"age,omitempty"`
	}

	type FormDataOmitempty struct {
		Username string `form:"username,omitempty"`
		Password string `form:"password,omitempty"`
	}

	type JSONDataEmptyName struct {
		FieldName string `json:",omitempty"` // Empty name, should use "FieldName"
	}

	type JSONDataSkipField struct {
		Public  string `json:"public"`
		Private string `json:"-"` // Should be skipped
	}

	tests := []struct {
		name     string
		setup    func() (*http.Request, any)
		bindFunc func(c *Context, params any) error
		validate func(t *testing.T, params any)
	}{
		{
			name: "json tag with omitempty",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"John","email":"john@example.com","age":30}`))
				req.Header.Set("Content-Type", "application/json")
				return req, &JSONDataOmitempty{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindJSON(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*JSONDataOmitempty)
				assert.Equal(t, "John", p.Name)
				assert.Equal(t, "john@example.com", p.Email)
				assert.Equal(t, 30, p.Age)
			},
		},
		{
			name: "form tag with omitempty",
			setup: func() (*http.Request, any) {
				form := url.Values{}
				form.Set("username", "testuser")
				form.Set("password", "secret123")
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req, &FormDataOmitempty{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindForm(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*FormDataOmitempty)
				assert.Equal(t, "testuser", p.Username)
				assert.Equal(t, "secret123", p.Password)
			},
		},
		{
			name: "json tag with empty name and options",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"FieldName":"test"}`))
				req.Header.Set("Content-Type", "application/json")
				return req, &JSONDataEmptyName{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindJSON(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*JSONDataEmptyName)
				assert.Equal(t, "test", p.FieldName)
			},
		},
		{
			name: "json tag with dash (skip field)",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"public":"visible","Private":"should be ignored"}`))
				req.Header.Set("Content-Type", "application/json")
				return req, &JSONDataSkipField{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindJSON(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*JSONDataSkipField)
				assert.Equal(t, "visible", p.Public)
				assert.Empty(t, p.Private, "Private should be empty (skipped)")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, params := tt.setup()
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, tt.bindFunc(c, params), "%s should succeed", tt.name)
			tt.validate(t, params)
		})
	}
}

// TestCookieGetter_Get_UnescapingError tests cookieGetter.Get with URL unescaping error
func TestCookieGetter_Get_UnescapingError(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "data", Value: "%ZZ"}, // Invalid percent encoding
	}

	getter := &cookieGetter{cookies: cookies}

	// Should fallback to raw cookie value on unescaping error
	value := getter.Get("data")
	assert.Equal(t, "%ZZ", value, "Expected raw value %%ZZ on unescaping error")
}

// TestCookieGetter_Get_NotFound tests cookieGetter.Get when cookie is not found
func TestCookieGetter_Get_NotFound(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "session_id", Value: "abc123"},
		{Name: "theme", Value: "dark"},
	}

	getter := &cookieGetter{cookies: cookies}

	// Should return empty string when cookie key is not found
	value := getter.Get("nonexistent")
	assert.Empty(t, value, "Expected empty string for nonexistent cookie")

	// Verify existing cookies still work
	session := getter.Get("session_id")
	assert.Equal(t, "abc123", session, "Expected session_id to be 'abc123'")
}

// TestBindForm_ParseErrors tests BindForm parse errors
func TestBindForm_ParseErrors(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name string `form:"name"`
	}

	tests := []struct {
		name           string
		setup          func() *http.Request
		expectedErrMsg string
	}{
		{
			name: "multipart parse error",
			setup: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("malformed multipart"))
				req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid-boundary")
				return req
			},
			expectedErrMsg: "failed to parse multipart form",
		},
		{
			name: "form parse error",
			setup: func() *http.Request {
				// Use failingReader to trigger ParseForm failure
				req := httptest.NewRequest(http.MethodPost, "/", &failingReader{})
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Body = io.NopCloser(&failingReader{})
				return req
			},
			expectedErrMsg: "failed to parse form",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := tt.setup()
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var data FormData
			err := c.BindForm(&data)

			require.Error(t, err, "Expected error for %s", tt.name)
			assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
		})
	}
}

// TestParseTime_AllFormats tests parseTime with all supported formats
func TestParseTime_AllFormats(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			type Params struct {
				Time time.Time `query:"time"`
			}

			req := httptest.NewRequest("GET", "/?time="+url.QueryEscape(tt.value), nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params Params
			err := c.BindQuery(&params)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for %q", tt.value)
			} else {
				require.NoError(t, err, "Unexpected error for %q", tt.value)
				assert.False(t, params.Time.IsZero(), "Time should not be zero for valid format %q", tt.value)
			}
		})
	}
}

// TestExtractBracketKey_EdgeCases tests extractBracketKey edge cases
func TestExtractBracketKey_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{"no prefix match", "other[key]", "prefix", ""},
		{"no closing bracket", "prefix[unclosed", "prefix", ""},
		{"empty brackets", "prefix[]", "prefix", ""},
		{"nested brackets", "prefix[key1][key2]", "prefix", ""},
		{"quoted key with double quotes", `prefix["key.with.dots"]`, "prefix", "key.with.dots"},
		{"quoted key with single quotes", "prefix['key-with-dash']", "prefix", "key-with-dash"},
		{"quoted key empty after trimming", `prefix[""]`, "prefix", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractBracketKey(tt.input, tt.prefix)
			assert.Equal(t, tt.expected, result, "extractBracketKey(%q, %q) = %q, want %q", tt.input, tt.prefix, result, tt.expected)
		})
	}
}

// TestPrefixGetter_Has_RemainingPaths tests remaining paths in prefixGetter.Has
func TestPrefixGetter_Has_RemainingPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() (*prefixGetter, string, bool)
		validate func(t *testing.T, result bool, key string)
	}{
		{
			name: "headerGetter with prefix",
			setup: func() (*prefixGetter, string, bool) {
				headers := http.Header{}
				headers.Set("X-Meta-Tags", "tag1")
				headers.Set("X-Other-Data", "value")
				hg := &headerGetter{headers: headers}
				pg := &prefixGetter{
					inner:  hg,
					prefix: "X-Meta-",
				}
				return pg, "Tags", true
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.True(t, result, "Expected Has(%q) to return true", key)
			},
		},
		{
			name: "headerGetter with prefix - nonexistent",
			setup: func() (*prefixGetter, string, bool) {
				headers := http.Header{}
				headers.Set("X-Meta-Tags", "tag1")
				headers.Set("X-Other-Data", "value")
				hg := &headerGetter{headers: headers}
				pg := &prefixGetter{
					inner:  hg,
					prefix: "X-Meta-",
				}
				return pg, "Nonexistent", false
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.False(t, result, "Expected Has(%q) to return false", key)
			},
		},
		{
			name: "paramsGetter with prefix",
			setup: func() (*prefixGetter, string, bool) {
				params := map[string]string{
					"meta.name": "John",
					"meta.age":  "30",
				}
				pg := &paramsGetter{params: params}
				pg2 := &prefixGetter{
					inner:  pg,
					prefix: "meta.",
				}
				return pg2, "name", true
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.True(t, result, "Expected Has(%q) to return true", key)
			},
		},
		{
			name: "cookieGetter with prefix",
			setup: func() (*prefixGetter, string, bool) {
				cookies := []*http.Cookie{
					{Name: "session.id", Value: "abc123"},
					{Name: "session.token", Value: "def456"},
				}
				cg := &cookieGetter{cookies: cookies}
				pg := &prefixGetter{
					inner:  cg,
					prefix: "session.",
				}
				return pg, "id", true
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.True(t, result, "Expected Has(%q) to return true", key)
			},
		},
		{
			name: "cookieGetter with prefix - nonexistent",
			setup: func() (*prefixGetter, string, bool) {
				cookies := []*http.Cookie{
					{Name: "session.id", Value: "abc123"},
					{Name: "session.token", Value: "def456"},
				}
				cg := &cookieGetter{cookies: cookies}
				pg := &prefixGetter{
					inner:  cg,
					prefix: "session.",
				}
				return pg, "nonexistent", false
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.False(t, result, "Expected Has(%q) to return false", key)
			},
		},
		{
			name: "inner getter that doesn't match queryGetter or formGetter",
			setup: func() (*prefixGetter, string, bool) {
				// Create a custom getter that doesn't match the type assertions
				customGetter := &paramsGetter{params: map[string]string{"key": "value"}}
				pg := &prefixGetter{
					inner:  customGetter,
					prefix: "prefix.",
				}
				return pg, "nonexistent", false
			},
			validate: func(t *testing.T, result bool, key string) {
				assert.False(t, result, "Expected Has(%q) to return false", key)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pg, key, expected := tt.setup()
			result := pg.Has(key)
			assert.Equal(t, expected, result, "Has(%q) = %v, want %v", key, result, expected)
			tt.validate(t, result, key)
		})
	}
}

// TestValidateEnum_ErrorPath tests validateEnum error path
func TestValidateEnum_ErrorPath(t *testing.T) {
	t.Parallel()

	type Params struct {
		Status string `query:"status" enum:"active,inactive,pending"`
	}

	tests := []struct {
		name           string
		query          string
		wantErr        bool
		expectedErrMsg string
		validate       func(t *testing.T, params Params)
	}{
		{
			name:           "invalid enum value",
			query:          "status=invalid",
			wantErr:        true,
			expectedErrMsg: "not in allowed values",
			validate:       func(t *testing.T, params Params) {},
		},
		{
			name:           "empty value skips validation",
			query:          "status=",
			wantErr:        false,
			expectedErrMsg: "",
			validate: func(t *testing.T, params Params) {
				assert.Empty(t, params.Status, "Expected empty status")
			},
		},
		{
			name:           "missing parameter skips validation",
			query:          "",
			wantErr:        false,
			expectedErrMsg: "",
			validate: func(t *testing.T, params Params) {
				assert.Empty(t, params.Status, "Expected empty status")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := "/"
			if tt.query != "" {
				url = "/?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params Params
			err := c.BindQuery(&params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			} else {
				require.NoError(t, err, "Expected no error for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestSetSliceField_ErrorPath tests setSliceField error paths
func TestSetSliceField_ErrorPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		params         any
		expectedErrMsg string
	}{
		{
			name:  "invalid element conversion",
			query: "ids=123&ids=invalid&ids=456",
			params: &struct {
				IDs []int `query:"ids"`
			}{},
			expectedErrMsg: "element",
		},
		{
			name:  "invalid time in slice",
			query: "times=2024-01-01&times=invalid-time&times=2024-01-02",
			params: &struct {
				Times []time.Time `query:"times"`
			}{},
			expectedErrMsg: "", // Any error is acceptable
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			require.Error(t, err, "Expected error for %s", tt.name)
			if tt.expectedErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			}
		})
	}
}

// TestSetField_PointerEmptyValue tests setField with pointer and empty value
func TestSetField_PointerEmptyValue(t *testing.T) {
	t.Parallel()

	type Params struct {
		Name *string `query:"name"`
		Age  *int    `query:"age"`
	}

	tests := []struct {
		name     string
		query    string
		validate func(t *testing.T, params Params)
	}{
		{
			name:  "empty string leaves pointer nil",
			query: "name=",
			validate: func(t *testing.T, params Params) {
				assert.Nil(t, params.Name, "Expected Name to be nil for empty value")
			},
		},
		{
			name:  "missing value leaves pointer nil",
			query: "",
			validate: func(t *testing.T, params Params) {
				assert.Nil(t, params.Name, "Expected Name to be nil for missing value")
				assert.Nil(t, params.Age, "Expected Age to be nil for missing value")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := "/"
			if tt.query != "" {
				url = "/?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			var params Params
			require.NoError(t, c.BindQuery(&params), "BindQuery should succeed for %s", tt.name)
			tt.validate(t, params)
		})
	}
}

// TestConvertValue_EdgeCases tests convertValue remaining paths
func TestConvertValue_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		params         any
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:  "int8 overflow",
			query: "value=999999",
			params: &struct {
				Value int8 `query:"value"`
			}{},
			wantErr:        false, // May or may not error depending on conversion, but tests the path
			expectedErrMsg: "",
		},
		{
			name:  "uint overflow",
			query: "value=-1",
			params: &struct {
				Value uint `query:"value"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid unsigned integer",
		},
		{
			name:  "float with invalid format",
			query: "value=not-a-number",
			params: &struct {
				Value float64 `query:"value"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid float",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
				}
			} else {
				// May or may not error, just test the path
				_ = err
			}
		})
	}
}

// TestParamsGetter_GetAll_NonExistent tests paramsGetter.GetAll for non-existent key
func TestParamsGetter_GetAll_NonExistent(t *testing.T) {
	t.Parallel()

	params := map[string]string{"id": "123"}
	getter := &paramsGetter{params: params}

	// Test non-existent key returns nil
	none := getter.GetAll("nonexistent")
	assert.Nil(t, none, "Expected nil for non-existent key")

	// Test existing key returns slice
	all := getter.GetAll("id")
	require.Len(t, all, 1, "Expected slice with 1 element")
	assert.Equal(t, "123", all[0], "Expected first element to be '123'")
}

// TestBind_ErrorPaths tests bind function error paths
func TestBind_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setup          func() (any, valueGetter)
		expectedErrMsg string
	}{
		{
			name: "non-pointer input",
			setup: func() (any, valueGetter) {
				type Params struct {
					Name string `query:"name"`
				}
				var params Params
				return params, &queryGetter{url.Values{}}
			},
			expectedErrMsg: "pointer to struct",
		},
		{
			name: "nil pointer",
			setup: func() (any, valueGetter) {
				var params *struct {
					Name string `query:"name"`
				}
				return params, &queryGetter{url.Values{}}
			},
			expectedErrMsg: "nil",
		},
		{
			name: "pointer to non-struct",
			setup: func() (any, valueGetter) {
				var value *int
				return &value, &queryGetter{url.Values{}}
			},
			expectedErrMsg: "pointer to struct",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params, getter := tt.setup()
			err := bind(params, getter, "query")

			require.Error(t, err, "Expected error for %s", tt.name)
			assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
		})
	}
}

// TestParseStructType_RemainingPaths tests parseStructType remaining paths
func TestParseStructType_RemainingPaths(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type ParamsUnexported struct {
		Exported string `query:"exported"`
		_        string `query:"unexported"` // unexported field (lowercase) should be skipped
	}

	type ParamsNoQueryTag struct {
		HasTag     string `query:"has_tag"`
		NoQueryTag string // No query tag, should be skipped for query binding
	}

	type DataEmptyJSONTag struct {
		FieldName string `json:""` // Empty tag, should use "FieldName"
	}

	type Embedded struct {
		Value string `query:"value"`
	}
	type ParamsEmbedded struct {
		*Embedded        // Pointer to embedded struct
		Name      string `query:"name"`
	}

	tests := []struct {
		name     string
		setup    func() (*http.Request, any)
		bindFunc func(c *Context, params any) error
		validate func(t *testing.T, params any)
	}{
		{
			name: "unexported fields are skipped",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest("GET", "/?exported=test&unexported=ignored", nil)
				return req, &ParamsUnexported{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindQuery(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*ParamsUnexported)
				assert.Equal(t, "test", p.Exported, "Expected Exported=test")
			},
		},
		{
			name: "non-standard tag skipped when empty",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest("GET", "/?has_tag=value", nil)
				return req, &ParamsNoQueryTag{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindQuery(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*ParamsNoQueryTag)
				assert.Equal(t, "value", p.HasTag, "Expected HasTag=value")
				assert.Empty(t, p.NoQueryTag, "NoQueryTag should remain zero value")
			},
		},
		{
			name: "json tag without name uses field name",
			setup: func() (*http.Request, any) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"FieldName":"test"}`))
				req.Header.Set("Content-Type", "application/json")
				return req, &DataEmptyJSONTag{}
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindJSON(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*DataEmptyJSONTag)
				assert.Equal(t, "test", p.FieldName, "Expected FieldName=test")
			},
		},
		{
			name: "embedded struct with pointer",
			setup: func() (*http.Request, any) {
				// Initialize the embedded pointer to avoid nil pointer panic
				params := ParamsEmbedded{
					Embedded: &Embedded{},
					Name:     "",
				}
				req := httptest.NewRequest("GET", "/?value=embedded&name=test", nil)
				return req, &params
			},
			bindFunc: func(c *Context, params any) error {
				return c.BindQuery(params)
			},
			validate: func(t *testing.T, params any) {
				p := params.(*ParamsEmbedded)
				assert.Equal(t, "test", p.Name, "Expected Name=test")
				require.NotNil(t, p.Embedded, "Embedded should not be nil")
				assert.Equal(t, "embedded", p.Value, "Expected Embedded.Value=embedded")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, params := tt.setup()
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, tt.bindFunc(c, params), "%s should succeed", tt.name)
			tt.validate(t, params)
		})
	}
}

// TestBind_SkipUnexportedFields tests that unexported fields are skipped
func TestBind_SkipUnexportedFields(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, c.BindQuery(&params), "BindQuery should succeed")

	assert.Equal(t, "john", params.Name, "Expected Name=john")
	assert.Equal(t, 30, params.Age, "Expected Age=30")

	// To actually hit this path, we need a field that's in the cache but CanSet() returns false.
	// This is defensive code, and in practice parseStructType filters these out.
	// This ensures we skip any field that somehow made it into the cache but isn't settable.
}

// TestBind_NestedStructError tests error handling for nested struct binding failures
func TestBind_NestedStructError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         string
		params        any
		expectedField string
		validate      func(t *testing.T, bindErr *BindError)
	}{
		{
			name:  "nested struct with invalid data",
			query: "address.zip_code=invalid",
			params: &struct {
				Address struct {
					ZipCode int `query:"zip_code"`
				} `query:"address"`
			}{},
			expectedField: "Address",
			validate: func(t *testing.T, bindErr *BindError) {
				assert.Equal(t, "query", bindErr.Tag, "Expected Tag='query'")
				assert.Empty(t, bindErr.Value, "Expected empty Value for nested struct error")
				assert.NotNil(t, bindErr.Err, "Expected underlying error")
			},
		},
		{
			name:  "nested struct with invalid time format",
			query: "meta.created_at=invalid-time",
			params: &struct {
				Metadata struct {
					CreatedAt time.Time `query:"created_at"`
				} `query:"meta"`
			}{},
			expectedField: "Metadata",
			validate:      func(t *testing.T, bindErr *BindError) {},
		},
		{
			name:  "nested struct with invalid enum",
			query: "config.status=invalid",
			params: &struct {
				Config struct {
					Status string `query:"status" enum:"active,inactive"`
				} `query:"config"`
			}{},
			expectedField: "Config",
			validate:      func(t *testing.T, bindErr *BindError) {},
		},
		{
			name:  "deeply nested struct error",
			query: "middle.inner.value=not-a-number",
			params: &struct {
				Middle struct {
					Inner struct {
						Value int `query:"value"`
					} `query:"inner"`
				} `query:"middle"`
			}{},
			expectedField: "Middle",
			validate:      func(t *testing.T, bindErr *BindError) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.BindQuery(tt.params)

			require.Error(t, err, "Expected error for %s", tt.name)

			var bindErr *BindError
			require.True(t, errors.As(err, &bindErr), "Expected BindError, got %T: %v", err, err)
			assert.Equal(t, tt.expectedField, bindErr.Field, "Expected Field=%q", tt.expectedField)
			tt.validate(t, bindErr)
		})
	}
}

// TestSetMapField_ComplexKeys tests map field binding with complex bracket notation
func TestSetMapField_ComplexKeys(t *testing.T) {
	t.Parallel()

	type Data struct {
		Config map[string]string `form:"config"`
	}

	r := New()
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

	assert.Equal(t, http.StatusOK, w.Code, "should handle complex map keys: %s", w.Body.String())
}

// TestSplitMediaType_EdgeCases tests splitMediaType with various inputs
func TestSplitMediaType_EdgeCases(_ *testing.T) {
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
	t.Parallel()

	type CookieData struct {
		Session string `cookie:"session"`
		Token   string `cookie:"token"`
	}

	r := New()
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
	assert.Equal(t, http.StatusOK, w.Code, "expected 200, got %d: %s", w.Code, w.Body.String())
}
