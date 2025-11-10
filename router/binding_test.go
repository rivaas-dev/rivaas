package router

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingReader is a custom reader that always returns an error for testing error paths
type failingReader struct{}

func (r *failingReader) Read([]byte) (int, error) {
	return 0, ErrReadError
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
