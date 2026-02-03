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
	"mime/multipart"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultipartGetter_ValueGetter tests ValueGetter interface implementation
func TestMultipartGetter_ValueGetter(t *testing.T) {
	t.Parallel()

	t.Run("Get returns first value", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			Value: url.Values{
				"name":  {"Alice"},
				"tags":  {"go", "rust", "python"},
				"empty": {""},
			},
		}

		getter := NewMultipartGetter(form)

		assert.Equal(t, "Alice", getter.Get("name"))
		assert.Equal(t, "go", getter.Get("tags"))
		assert.Equal(t, "", getter.Get("empty"))
		assert.Equal(t, "", getter.Get("missing"))
	})

	t.Run("GetAll returns all values", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			Value: url.Values{
				"tags":     {"go", "rust", "python"},
				"single":   {"value"},
				"brackets": nil,
			},
		}

		getter := NewMultipartGetter(form)

		assert.Equal(t, []string{"go", "rust", "python"}, getter.GetAll("tags"))
		assert.Equal(t, []string{"value"}, getter.GetAll("single"))
		assert.Nil(t, getter.GetAll("missing"))
	})

	t.Run("GetAll supports bracket notation", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			Value: url.Values{
				"ids[]": {"1", "2", "3"},
			},
		}

		getter := NewMultipartGetter(form)

		// Should find with bracket notation
		assert.Equal(t, []string{"1", "2", "3"}, getter.GetAll("ids"))
	})

	t.Run("Has checks key existence", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			Value: url.Values{
				"present": {"value"},
				"empty":   {""},
			},
		}

		getter := NewMultipartGetter(form)

		assert.True(t, getter.Has("present"))
		assert.True(t, getter.Has("empty"))
		assert.False(t, getter.Has("missing"))
	})

	t.Run("Has supports bracket notation", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			Value: url.Values{
				"items[]": {"a", "b"},
			},
		}

		getter := NewMultipartGetter(form)

		assert.True(t, getter.Has("items"))
	})
}

// TestMultipartGetter_FileGetter tests FileGetter interface implementation
func TestMultipartGetter_FileGetter(t *testing.T) {
	t.Parallel()

	t.Run("File returns first file", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("avatar", "photo.jpg")
		require.NoError(t, err)
		_, err = fw.Write([]byte("image data"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		getter := NewMultipartGetter(form)

		file, err := getter.File("avatar")
		require.NoError(t, err)
		assert.Equal(t, "photo.jpg", file.Name)
	})

	t.Run("File returns error when not found", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			File: make(map[string][]*multipart.FileHeader),
		}

		getter := NewMultipartGetter(form)

		_, err := getter.File("missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("Files returns all files", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for _, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
			fw, err := writer.CreateFormFile("documents", name)
			require.NoError(t, err)
			_, err = fw.Write([]byte("content"))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		getter := NewMultipartGetter(form)

		files, err := getter.Files("documents")
		require.NoError(t, err)
		require.Len(t, files, 3)
		assert.Equal(t, "file1.txt", files[0].Name)
		assert.Equal(t, "file2.txt", files[1].Name)
		assert.Equal(t, "file3.txt", files[2].Name)
	})

	t.Run("Files returns error when not found", func(t *testing.T) {
		t.Parallel()

		form := &multipart.Form{
			File: make(map[string][]*multipart.FileHeader),
		}

		getter := NewMultipartGetter(form)

		_, err := getter.Files("missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoFilesFound)
	})

	t.Run("HasFile checks file existence", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("upload", "test.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())
		getter := NewMultipartGetter(form)

		assert.True(t, getter.HasFile("upload"))
		assert.False(t, getter.HasFile("missing"))
	})
}

// TestMultipartGetter_ApproxLen tests map capacity estimation
func TestMultipartGetter_ApproxLen(t *testing.T) {
	t.Parallel()

	form := &multipart.Form{
		Value: url.Values{
			"user.name":    {"Alice"},
			"user.email":   {"alice@example.com"},
			"user.age":     {"30"},
			"settings.foo": {"bar"},
			"other":        {"value"},
		},
	}

	getter := NewMultipartGetter(form)

	// Should count keys with "user." prefix
	count := getter.ApproxLen("user")
	assert.Equal(t, 3, count)

	// Should count keys with "settings." prefix
	count = getter.ApproxLen("settings")
	assert.Equal(t, 1, count)

	// No keys with "missing." prefix
	count = getter.ApproxLen("missing")
	assert.Equal(t, 0, count)
}

// TestMultipart_StructBinding tests binding multipart forms to structs
func TestMultipart_StructBinding(t *testing.T) {
	t.Parallel()

	t.Run("binds form values", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		require.NoError(t, writer.WriteField("name", "Alice"))
		require.NoError(t, writer.WriteField("age", "30"))
		require.NoError(t, writer.WriteField("active", "true"))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type User struct {
			Name   string `form:"name"`
			Age    int    `form:"age"`
			Active bool   `form:"active"`
		}

		var user User
		err := MultipartTo(form, &user)
		require.NoError(t, err)

		assert.Equal(t, "Alice", user.Name)
		assert.Equal(t, 30, user.Age)
		assert.True(t, user.Active)
	})

	t.Run("binds single file", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("avatar", "photo.jpg")
		require.NoError(t, err)
		_, err = fw.Write([]byte("image data"))
		require.NoError(t, err)
		require.NoError(t, writer.WriteField("username", "alice"))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Request struct {
			Avatar   *File  `form:"avatar"`
			Username string `form:"username"`
		}

		var req Request
		err = MultipartTo(form, &req)
		require.NoError(t, err)

		assert.NotNil(t, req.Avatar)
		assert.Equal(t, "photo.jpg", req.Avatar.Name)
		assert.Equal(t, "alice", req.Username)
	})

	t.Run("binds multiple files", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for _, name := range []string{"file1.txt", "file2.txt"} {
			fw, err := writer.CreateFormFile("attachments", name)
			require.NoError(t, err)
			_, err = fw.Write([]byte("content"))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Request struct {
			Attachments []*File `form:"attachments"`
		}

		var req Request
		err := MultipartTo(form, &req)
		require.NoError(t, err)

		require.Len(t, req.Attachments, 2)
		assert.Equal(t, "file1.txt", req.Attachments[0].Name)
		assert.Equal(t, "file2.txt", req.Attachments[1].Name)
	})

	t.Run("binds JSON in form field", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		require.NoError(t, writer.WriteField("settings", `{"theme":"dark","lang":"en"}`))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Settings struct {
			Theme string `json:"theme"`
			Lang  string `json:"lang"`
		}

		type Request struct {
			Settings Settings `form:"settings"`
		}

		var req Request
		err := MultipartTo(form, &req)
		require.NoError(t, err)

		assert.Equal(t, "dark", req.Settings.Theme)
		assert.Equal(t, "en", req.Settings.Lang)
	})

	t.Run("binds nested JSON struct", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		require.NoError(t, writer.WriteField("config", `{"database":{"host":"localhost","port":5432}}`))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Database struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		}

		type Config struct {
			Database Database `json:"database"`
		}

		type Request struct {
			Config Config `form:"config"`
		}

		var req Request
		err := MultipartTo(form, &req)
		require.NoError(t, err)

		assert.Equal(t, "localhost", req.Config.Database.Host)
		assert.Equal(t, 5432, req.Config.Database.Port)
	})

	t.Run("combines files and JSON", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fw, err := writer.CreateFormFile("document", "report.pdf")
		require.NoError(t, err)
		_, err = fw.Write([]byte("pdf data"))
		require.NoError(t, err)

		require.NoError(t, writer.WriteField("metadata", `{"title":"Report","version":1}`))
		require.NoError(t, writer.WriteField("title", "My Report"))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Metadata struct {
			Title   string `json:"title"`
			Version int    `json:"version"`
		}

		type Request struct {
			Document *File    `form:"document"`
			Metadata Metadata `form:"metadata"`
			Title    string   `form:"title"`
		}

		var req Request
		err = MultipartTo(form, &req)
		require.NoError(t, err)

		assert.NotNil(t, req.Document)
		assert.Equal(t, "report.pdf", req.Document.Name)
		assert.Equal(t, "Report", req.Metadata.Title)
		assert.Equal(t, 1, req.Metadata.Version)
		assert.Equal(t, "My Report", req.Title)
	})
}

// TestMultipart_GenericBinding tests type-safe Multipart function
func TestMultipart_GenericBinding(t *testing.T) {
	t.Parallel()

	t.Run("binds with generic function", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		require.NoError(t, writer.WriteField("name", "Bob"))
		require.NoError(t, writer.WriteField("email", "bob@example.com"))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type User struct {
			Name  string `form:"name"`
			Email string `form:"email"`
		}

		user, err := Multipart[User](form)
		require.NoError(t, err)

		assert.Equal(t, "Bob", user.Name)
		assert.Equal(t, "bob@example.com", user.Email)
	})
}

// TestFromMultipart tests multipart as binding source option
func TestFromMultipart(t *testing.T) {
	t.Parallel()

	t.Run("binds with FromMultipart option", func(t *testing.T) {
		t.Parallel()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fw, err := writer.CreateFormFile("file", "test.txt")
		require.NoError(t, err)
		_, err = fw.Write([]byte("content"))
		require.NoError(t, err)
		require.NoError(t, writer.WriteField("title", "Test"))
		require.NoError(t, writer.Close())

		form := parseMultipartForm(t, body.Bytes(), writer.FormDataContentType())

		type Request struct {
			File  *File  `form:"file"`
			Title string `form:"title"`
		}

		var req Request
		err = BindTo(&req, FromMultipart(form))
		require.NoError(t, err)

		assert.NotNil(t, req.File)
		assert.Equal(t, "test.txt", req.File.Name)
		assert.Equal(t, "Test", req.Title)
	})
}

// parseMultipartForm is a helper to parse multipart form data
func parseMultipartForm(t *testing.T, body []byte, contentType string) *multipart.Form {
	t.Helper()

	reader := bytes.NewReader(body)
	boundary := contentType[len("multipart/form-data; boundary="):]

	mr := multipart.NewReader(reader, boundary)
	form, err := mr.ReadForm(32 << 20)
	require.NoError(t, err)

	return form
}
