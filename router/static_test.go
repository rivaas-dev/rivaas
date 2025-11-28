// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	t.Parallel()

	r := MustNew()

	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("Hello, World!"), 0644)
	require.NoError(t, err)

	t.Run("Static directory serving", func(t *testing.T) {
		t.Parallel()
		r.Static("/static", tmpDir)

		req := httptest.NewRequest("GET", "/static/test.txt", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello, World!", w.Body.String())
	})

	t.Run("StaticFile serving", func(t *testing.T) {
		t.Parallel()
		r.StaticFile("/file", testFile)

		req := httptest.NewRequest("GET", "/file", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Hello, World!", w.Body.String())
	})

	t.Run("ServeFile method", func(t *testing.T) {
		t.Parallel()
		r.GET("/download", func(c *Context) {
			c.ServeFile(testFile)
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
	t.Parallel()

	r := MustNew()

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
