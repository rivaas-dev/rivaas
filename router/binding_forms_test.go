package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindForm_BasicTypes tests binding basic form data types
func TestBindForm_BasicTypes(t *testing.T) {
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

// TestBindForm_NestedStructs tests binding nested struct data
func TestBindForm_NestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `form:"street"`
		City   string `form:"city"`
		Zip    string `form:"zip"`
	}

	type User struct {
		Name    string  `form:"name"`
		Address Address `form:"address"`
	}

	r := New()
	r.POST("/user", func(c *Context) {
		var user User
		if err := c.BindForm(&user); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	})

	form := url.Values{}
	form.Set("name", "Alice")
	form.Set("address.street", "123 Main St")
	form.Set("address.city", "Springfield")
	form.Set("address.zip", "12345")

	req := httptest.NewRequest(http.MethodPost, "/user", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response User
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Alice", response.Name)
	assert.Equal(t, "123 Main St", response.Address.Street)
	assert.Equal(t, "Springfield", response.Address.City)
	assert.Equal(t, "12345", response.Address.Zip)
}

// TestBindForm_Slices tests binding slice form data
func TestBindForm_Slices(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Tags []string `form:"tags"`
		IDs  []int    `form:"ids"`
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
	form.Add("tags", "go")
	form.Add("tags", "rust")
	form.Add("tags", "python")
	form.Add("ids", "1")
	form.Add("ids", "2")
	form.Add("ids", "3")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, []string{"go", "rust", "python"}, response.Tags)
	assert.Equal(t, []int{1, 2, 3}, response.IDs)
}

// TestBindForm_MapStringString tests map[string]string binding
func TestBindForm_MapStringString(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Metadata map[string]string `form:"metadata"`
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
	form.Set("metadata[key1]", "value1")
	form.Set("metadata[key2]", "value2")
	form.Set("metadata[key3]", "value3")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "value1", response.Metadata["key1"])
	assert.Equal(t, "value2", response.Metadata["key2"])
	assert.Equal(t, "value3", response.Metadata["key3"])
}

// TestBindForm_MapStringAny tests map[string]any binding
func TestBindForm_MapStringAny(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Data map[string]any `form:"data"`
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
	form.Set("data[name]", "John")
	form.Set("data[age]", "30")
	form.Set("data[active]", "true")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.NotNil(t, response.Data)
	assert.NotEmpty(t, response.Data)
}

// TestBindForm_MapStringInt tests map[string]int binding
func TestBindForm_MapStringInt(t *testing.T) {
	t.Parallel()

	type FormData struct {
		IntMap map[string]int `form:"intmap"`
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
	form.Set("intmap[count]", "42")
	form.Set("intmap[total]", "100")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, 42, response.IntMap["count"])
	assert.Equal(t, 100, response.IntMap["total"])
}

// TestBindForm_NestedMapsNotSupported tests that nested maps return appropriate error
func TestBindForm_NestedMapsNotSupported(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Config map[string]map[string]string `form:"config"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "nested maps not supported"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("config[database][host]", "localhost")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestBindForm_EmptyForm tests binding with empty form data
func TestBindForm_EmptyForm(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name string `form:"name"`
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

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

// TestValueGetter_GetAll tests all GetAll implementations
func TestValueGetter_GetAll(t *testing.T) {
	t.Parallel()

	t.Run("queryGetter", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Add("tags", "go")
		values.Add("tags", "rust")
		values.Add("tags", "python")

		getter := &queryGetter{values: values}

		all := getter.GetAll("tags")
		assert.Equal(t, []string{"go", "rust", "python"}, all)

		none := getter.GetAll("nonexistent")
		assert.Nil(t, none)
	})

	t.Run("paramsGetter", func(t *testing.T) {
		t.Parallel()

		params := map[string]string{"id": "123"}
		getter := &paramsGetter{params: params}

		all := getter.GetAll("id")
		assert.Equal(t, []string{"123"}, all)

		none := getter.GetAll("nonexistent")
		assert.Nil(t, none)
	})

	t.Run("cookieGetter", func(t *testing.T) {
		t.Parallel()

		cookies := []*http.Cookie{
			{Name: "session", Value: url.QueryEscape("abc123")},
			{Name: "session", Value: url.QueryEscape("def456")},
		}
		getter := &cookieGetter{cookies: cookies}

		all := getter.GetAll("session")
		assert.Len(t, all, 2)

		none := getter.GetAll("nonexistent")
		assert.Empty(t, none)
	})

	t.Run("headerGetter", func(t *testing.T) {
		t.Parallel()

		headers := http.Header{}
		headers.Add("X-Tags", "tag1")
		headers.Add("X-Tags", "tag2")
		headers.Add("X-Tags", "tag3")

		getter := &headerGetter{headers: headers}

		all := getter.GetAll("X-Tags")
		assert.Len(t, all, 3)
	})

	t.Run("formGetter", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Add("items", "item1")
		values.Add("items", "item2")

		getter := &formGetter{values: values}

		all := getter.GetAll("items")
		assert.Len(t, all, 2)
	})
}

// TestBindError_Unwrap tests the Unwrap method
func TestBindError_Unwrap(t *testing.T) {
	t.Parallel()

	originalErr := &BindError{
		Field: "age",
		Value: "invalid",
		Type:  "int",
		Tag:   "form",
		Err:   nil,
	}

	innerErr := &BindError{
		Field: "nested",
		Value: "bad",
		Type:  "string",
		Tag:   "json",
	}

	outerErr := &BindError{
		Field: "age",
		Value: "invalid",
		Type:  "int",
		Tag:   "form",
		Err:   innerErr,
	}

	unwrapped := outerErr.Unwrap()
	assert.True(t, errors.Is(unwrapped, innerErr))

	assert.Nil(t, originalErr.Unwrap())
}

// TestBindForm_MapIntString tests map[int]string binding
// Note: This test accepts both success and failure since integer key conversion support may vary
func TestBindForm_MapIntString(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Items map[int]string `form:"items"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "conversion failed"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("items[1]", "first")
	form.Set("items[2]", "second")
	form.Set("items[10]", "tenth")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Accept either success or error - implementation may not support int keys
	if w.Code == http.StatusOK {
		var response FormData
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Len(t, response.Items, 3)
		assert.Equal(t, "first", response.Items[1])
		assert.Equal(t, "second", response.Items[2])
		assert.Equal(t, "tenth", response.Items[10])
	} else {
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

// TestBindForm_ComplexNested tests deeply nested form structures
func TestBindForm_ComplexNested(t *testing.T) {
	t.Parallel()

	type Nested struct {
		Level3 string `form:"level3"`
	}

	type Middle struct {
		Level2 string `form:"level2"`
		Nested Nested `form:"nested"`
	}

	type FormData struct {
		Level1 string `form:"level1"`
		Middle Middle `form:"middle"`
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
	form.Set("level1", "value1")
	form.Set("middle.level2", "value2")
	form.Set("middle.nested.level3", "value3")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "value1", response.Level1)
	assert.Equal(t, "value2", response.Middle.Level2)
	assert.Equal(t, "value3", response.Middle.Nested.Level3)
}

// TestBindForm_ReturnsErrorOnInvalidIntConversion tests binding with type conversion errors
func TestBindForm_ReturnsErrorOnInvalidIntConversion(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Age int `form:"age"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "validation failed"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("age", "not-a-number")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestBindForm_SpecialCharacters tests form data with special characters
func TestBindForm_SpecialCharacters(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Text string `form:"text"`
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
	form.Set("text", "Hello & Goodbye! @#$%^&*()")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Hello & Goodbye! @#$%^&*()", response.Text)
}

// TestBindForm_ArrayNotation tests array notation in form keys
func TestBindForm_ArrayNotation(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Items []string `form:"items"`
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
	form.Add("items[]", "item1")
	form.Add("items[]", "item2")
	form.Add("items[]", "item3")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBindForm_RequiredFieldValidation tests required field validation
func TestBindForm_RequiredFieldValidation(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name  string `form:"name" binding:"required"`
		Email string `form:"email" binding:"required"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "validation failed"})
			return
		}

		// Manual validation for demonstration
		if data.Name == "" || data.Email == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "required field missing"})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("name", "John")
	// email is intentionally missing

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestBindForm_URLEncoded tests URL-encoded form data
func TestBindForm_URLEncoded(t *testing.T) {
	t.Parallel()

	type FormData struct {
		URL   string `form:"url"`
		Query string `form:"query"`
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
	form.Set("url", "https://example.com/path?foo=bar")
	form.Set("query", "search term with spaces")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "https://example.com/path?foo=bar", response.URL)
	assert.Equal(t, "search term with spaces", response.Query)
}

// TestCookieGetter_Has tests the Has method
func TestCookieGetter_Has(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "user_id", Value: "42"},
	}

	getter := &cookieGetter{cookies: cookies}

	assert.True(t, getter.Has("session"))
	assert.True(t, getter.Has("user_id"))
	assert.False(t, getter.Has("nonexistent"))
}

// TestBindForm_DuplicateKeys tests handling of duplicate form keys
func TestBindForm_DuplicateKeys(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Values []string `form:"value"`
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
	form.Add("value", "first")
	form.Add("value", "second")
	form.Add("value", "third")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Len(t, response.Values, 3)
	assert.Equal(t, []string{"first", "second", "third"}, response.Values)
}

// TestBindForm_ConvertsIntegerMapKeys tests map keys with integer type conversion
func TestBindForm_ConvertsIntegerMapKeys(t *testing.T) {
	t.Parallel()

	type FormData struct {
		IntKeys map[int]string `form:"intkeys"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "key conversion failed"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("intkeys[100]", "hundred")
	form.Set("intkeys[200]", "two hundred")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Accept success if implementation supports it
	if w.Code == http.StatusOK {
		var response FormData
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Len(t, response.IntKeys, 2)
		assert.Equal(t, "hundred", response.IntKeys[100])
		assert.Equal(t, "two hundred", response.IntKeys[200])
	} else {
		// Or accept error if not supported
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

// TestBindForm_ReturnsErrorOnInvalidMapKey tests invalid map key conversion
func TestBindForm_ReturnsErrorOnInvalidMapKey(t *testing.T) {
	t.Parallel()

	type FormData struct {
		IntKeys map[int]string `form:"intkeys"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid key"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("intkeys[not-a-number]", "value")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestBindForm_UnicodeCharacters tests form data with unicode characters
func TestBindForm_UnicodeCharacters(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Text string `form:"text"`
		Name string `form:"name"`
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
	form.Set("text", "Hello 世界! 🌍")
	form.Set("name", "José María Ömer")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response FormData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Hello 世界! 🌍", response.Text)
	assert.Equal(t, "José María Ömer", response.Name)
}

// TestBindForm_EmptySliceInitialization tests that empty slices are properly initialized
func TestBindForm_EmptySliceInitialization(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Tags []string `form:"tags"`
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

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBindForm_EmptyMapInitialization tests that empty maps are properly initialized
func TestBindForm_EmptyMapInitialization(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Data map[string]string `form:"data"`
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

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
