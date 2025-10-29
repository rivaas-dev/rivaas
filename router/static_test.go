package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaticFileServing tests static file serving
func TestStaticFileServing(t *testing.T) {
	r := New()

	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("Hello, World!"), 0644)
	require.NoError(t, err)

	t.Run("Static directory serving", func(t *testing.T) {
		r.Static("/static", tmpDir)

		req := httptest.NewRequest("GET", "/static/test.txt", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello, World!", w.Body.String())
	})

	t.Run("StaticFile serving", func(t *testing.T) {
		r.StaticFile("/file", testFile)

		req := httptest.NewRequest("GET", "/file", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello, World!", w.Body.String())
	})

	t.Run("File method", func(t *testing.T) {
		r.GET("/download", func(c *Context) {
			c.File(testFile)
		})

		req := httptest.NewRequest("GET", "/download", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello, World!", w.Body.String())
	})
}

// TestStaticFSWithCustomFileSystem tests StaticFS with custom file system
func TestStaticFSWithCustomFileSystem(t *testing.T) {
	r := New()

	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	err := os.WriteFile(testFile, []byte("<h1>Hello</h1>"), 0644)
	require.NoError(t, err)

	r.StaticFS("/files", http.Dir(tmpDir))

	req := httptest.NewRequest("GET", "/files/test.html", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body, _ := io.ReadAll(w.Body)
	assert.Equal(t, "<h1>Hello</h1>", string(body))
}
