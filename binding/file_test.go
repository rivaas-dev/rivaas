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

//go:build !integration

package binding

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFile tests the File constructor
func TestNewFile(t *testing.T) {
	t.Parallel()

	t.Run("creates file with basic properties", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("upload", "test.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		// Parse form
		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		require.NotNil(t, form.File["upload"])

		file := NewFile(form.File["upload"][0])
		assert.Equal(t, "test.txt", file.Name)
		assert.Greater(t, file.Size, int64(0))
		assert.NotEmpty(t, file.ContentType)
	})

	t.Run("sanitizes filename with path traversal", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("malicious", "../../../etc/passwd")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["malicious"][0])

		assert.Equal(t, "passwd", file.Name)
		assert.NotContains(t, file.Name, "..")
		assert.NotContains(t, file.Name, "/")
	})

	t.Run("sanitizes filename with backslashes", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("windows", "..\\..\\Windows\\System32\\config")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["windows"][0])

		// Backslashes should be replaced with underscores
		assert.NotContains(t, file.Name, "\\", "Backslashes should be sanitized")
		// The filename may still contain underscore-replaced dots, which is safe
		// What matters is no actual path traversal characters remain
	})

	t.Run("defaults content type when missing", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Create form file without Content-Type
		part, err := writer.CreatePart(map[string][]string{
			"Content-Disposition": {`form-data; name="upload"; filename="test.bin"`},
		})
		require.NoError(t, err)
		_, err = part.Write([]byte("binary data"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["upload"][0])

		assert.Equal(t, "application/octet-stream", file.ContentType)
	})
}

// TestFile_Bytes tests reading file contents into memory
func TestFile_Bytes(t *testing.T) {
	t.Parallel()

	t.Run("reads file content", func(t *testing.T) {
		t.Parallel()

		content := []byte("hello world from file")
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("data", "hello.txt")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["data"][0])

		data, err := file.Bytes()
		require.NoError(t, err)
		assert.Equal(t, content, data)
	})

	t.Run("handles empty file", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_, err := writer.CreateFormFile("empty", "empty.txt")
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["empty"][0])

		data, err := file.Bytes()
		require.NoError(t, err)
		assert.Empty(t, data)
	})
}

// TestFile_Open tests streaming file content
func TestFile_Open(t *testing.T) {
	t.Parallel()

	t.Run("opens file for reading", func(t *testing.T) {
		t.Parallel()

		content := []byte("streaming content")
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("stream", "stream.bin")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["stream"][0])

		reader, err := file.Open()
		require.NoError(t, err)
		defer func() {
			if closeErr := reader.Close(); closeErr != nil {
				t.Errorf("failed to close reader: %v", closeErr)
			}
		}()

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, content, data)
	})
}

// TestFile_Save tests saving file to disk
func TestFile_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves file successfully", func(t *testing.T) {
		t.Parallel()

		content := []byte("test file content for saving")
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("upload", "testfile.txt")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["upload"][0])

		// Save to temp directory
		tmpDir := t.TempDir()
		dstPath := filepath.Join(tmpDir, "saved-file.txt")

		err = file.Save(dstPath)
		require.NoError(t, err)

		// Verify file was saved
		//nolint:gosec // G304: This is a test using controlled temp directory
		savedContent, err := os.ReadFile(dstPath)
		require.NoError(t, err)
		assert.Equal(t, content, savedContent)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("upload", "test.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["upload"][0])

		// Save to nested path
		tmpDir := t.TempDir()
		dstPath := filepath.Join(tmpDir, "nested", "dir", "file.txt")

		err = file.Save(dstPath)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(dstPath)
		assert.NoError(t, err)
	})

	t.Run("cleans destination path", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("upload", "test.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		file := NewFile(form.File["upload"][0])

		tmpDir := t.TempDir()
		// Path with redundant elements
		dstPath := filepath.Join(tmpDir, "dir", "..", ".", "file.txt")

		err = file.Save(dstPath)
		require.NoError(t, err)

		// Verify file exists at cleaned path
		cleanPath := filepath.Join(tmpDir, "file.txt")
		_, err = os.Stat(cleanPath)
		assert.NoError(t, err)
	})
}

// TestFile_Ext tests file extension extraction
func TestFile_Ext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "simple extension",
			filename: "photo.jpg",
			expected: ".jpg",
		},
		{
			name:     "multiple extensions",
			filename: "archive.tar.gz",
			expected: ".gz",
		},
		{
			name:     "no extension",
			filename: "README",
			expected: "",
		},
		{
			name:     "dot file",
			filename: ".gitignore",
			expected: ".gitignore", // filepath.Ext treats the whole name as extension
		},
		{
			name:     "uppercase extension",
			filename: "document.PDF",
			expected: ".PDF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			fw, err := writer.CreateFormFile("file", tt.filename)
			require.NoError(t, err)
			_, err = fw.Write([]byte("content"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			form := createMultipartForm(t, body.Bytes(), writer.FormDataContentType())
			file := NewFile(form.File["file"][0])

			assert.Equal(t, tt.expected, file.Ext())
		})
	}
}

// createMultipartForm is a helper to create a parsed multipart form
func createMultipartForm(t *testing.T, body []byte, contentType string) *multipart.Form {
	t.Helper()

	// Create a multipart form directly for testing
	reader := bytes.NewReader(body)
	boundary := contentType[len("multipart/form-data; boundary="):]

	mr := multipart.NewReader(reader, boundary)
	form, err := mr.ReadForm(32 << 20)
	require.NoError(t, err)

	return form
}
