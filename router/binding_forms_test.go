package router

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindForm_Integration tests basic router.Context integration
func TestBindForm_Integration(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name   string  `form:"name"`
		Age    int     `form:"age"`
		Active bool    `form:"active"`
		Score  float64 `form:"score"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("name", "John Doe")
	form.Set("age", "30")
	form.Set("active", "true")
	form.Set("score", "95.5")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "John Doe", response.Name)
	assert.Equal(t, 30, response.Age)
	assert.True(t, response.Active)
	assert.Equal(t, 95.5, response.Score)
}

// TestBindForm_Multipart tests binding multipart form data
func TestBindForm_Multipart(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}

	r := New()
	r.POST("/upload", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Jane")
	_ = writer.WriteField("email", "jane@example.com")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Jane", response.Name)
	assert.Equal(t, "jane@example.com", response.Email)
}

// TestBindForm_MalformedMultipart tests handling of malformed multipart data
func TestBindForm_MalformedMultipart(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name string `form:"name"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "parse failed"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("malformed"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
