package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContextHelpers tests context helper methods
func TestContextHelpers(t *testing.T) {
	r := New()

	t.Run("PostForm", func(t *testing.T) {
		r.POST("/form", func(c *Context) {
			username := c.FormValue("username")
			password := c.FormValue("password")
			c.String(http.StatusOK, "user=%s,pass=%s", username, password)
		})

		req := httptest.NewRequest("POST", "/form", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.PostForm = map[string][]string{
			"username": {"john"},
			"password": {"secret"},
		}
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "user=john,pass=secret", w.Body.String())
	})

	t.Run("PostFormDefault", func(t *testing.T) {
		r.POST("/form-default", func(c *Context) {
			role := c.FormValueDefault("role", "guest")
			c.String(http.StatusOK, "role=%s", role)
		})

		req := httptest.NewRequest("POST", "/form-default", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "role=guest", w.Body.String())
	})

	t.Run("IsSecure", func(t *testing.T) {
		r.GET("/secure", func(c *Context) {
			if c.IsHTTPS() {
				c.String(http.StatusOK, "secure")
			} else {
				c.String(http.StatusOK, "insecure")
			}
		})

		// Test HTTP
		req := httptest.NewRequest("GET", "/secure", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "insecure", w.Body.String())

		// Test with X-Forwarded-Proto header
		req = httptest.NewRequest("GET", "/secure", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "secure", w.Body.String())
	})

	t.Run("NoContent", func(t *testing.T) {
		r.DELETE("/item", func(c *Context) {
			c.NoContent()
		})

		req := httptest.NewRequest("DELETE", "/item", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("SetCookie and GetCookie", func(t *testing.T) {
		r.GET("/set-cookie", func(c *Context) {
			c.SetCookie("session", "abc123", 3600, "/", "", false, true)
			c.String(http.StatusOK, "cookie set")
		})

		r.GET("/get-cookie", func(c *Context) {
			session, err := c.GetCookie("session")
			if err != nil {
				c.String(http.StatusNotFound, "no cookie")
			} else {
				c.String(http.StatusOK, "session=%s", session)
			}
		})

		// Test setting cookie
		req := httptest.NewRequest("GET", "/set-cookie", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		cookies := w.Result().Cookies()
		assert.NotEmpty(t, cookies)

		// Test getting cookie
		req = httptest.NewRequest("GET", "/get-cookie", nil)
		req.AddCookie(cookies[0])
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "session=abc123")

		// Test missing cookie
		req = httptest.NewRequest("GET", "/get-cookie", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "no cookie", w.Body.String())
	})
}

// TestNewContext tests the NewContext function
func TestNewContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(w, req)

	assert.NotNil(t, ctx)
	assert.Equal(t, req, ctx.Request)
	assert.Equal(t, w, ctx.Response)
	assert.Equal(t, int32(-1), ctx.index)
}

// TestStatusMethod tests the Status method edge cases
func TestStatusMethod(t *testing.T) {
	r := New()

	t.Run("Status with wrapped responseWriter", func(t *testing.T) {
		r.GET("/status-wrapped", func(c *Context) {
			c.Status(http.StatusAccepted)
			c.String(http.StatusOK, "ok") // Should use Accepted status
		})

		req := httptest.NewRequest("GET", "/status-wrapped", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("Status with plain responseWriter", func(t *testing.T) {
		// Create context with plain http.ResponseWriter
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)

		ctx.Status(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}
