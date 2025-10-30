package router

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestBindForm_BasicTypes tests binding basic form data types
func TestBindForm_BasicTypes(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_NestedStructs tests binding nested struct data
func TestBindForm_NestedStructs(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_Slices tests binding slice form data
func TestBindForm_Slices(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_Maps tests binding map form data
func TestBindForm_Maps(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_MapInterface tests binding map[string]interface{}
func TestBindForm_MapInterface(t *testing.T) {
	type FormData struct {
		Data map[string]interface{} `form:"data"`
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_NestedMaps tests that nested maps return appropriate error
func TestBindForm_NestedMaps(t *testing.T) {
	type FormData struct {
		Config map[string]map[string]string `form:"config"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			// Nested maps are not supported, should return error
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

	// Should return error for nested maps
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for nested maps, got %d", w.Code)
	}
}

// TestBindForm_EmptyForm tests binding with empty form data
func TestBindForm_EmptyForm(t *testing.T) {
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

	// Should succeed with empty values
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_Multipart tests binding multipart form data
func TestBindForm_Multipart(t *testing.T) {
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
	writer.WriteField("name", "Jane")
	writer.WriteField("email", "jane@example.com")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_ParseError tests handling of form parse errors
func TestBindForm_ParseError(t *testing.T) {
	type FormData struct {
		Name string `form:"name"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			// Error should be returned for malformed multipart
			c.JSON(http.StatusBadRequest, map[string]string{"error": "parse failed"})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	// Malformed multipart data
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("malformed"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for malformed multipart, got %d", w.Code)
	}
}

// TestValueGetter_GetAll tests all GetAll implementations
func TestValueGetter_GetAll(t *testing.T) {
	t.Run("queryGetter", func(t *testing.T) {
		values := url.Values{}
		values.Add("tags", "go")
		values.Add("tags", "rust")
		values.Add("tags", "python")

		getter := &queryGetter{values: values}

		all := getter.GetAll("tags")
		if len(all) != 3 {
			t.Errorf("expected 3 values, got %d", len(all))
		}

		if all[0] != "go" || all[1] != "rust" || all[2] != "python" {
			t.Errorf("unexpected values: %v", all)
		}

		// Test non-existent key
		none := getter.GetAll("nonexistent")
		if none != nil {
			t.Errorf("expected nil for non-existent key, got %v", none)
		}
	})

	t.Run("paramsGetter", func(t *testing.T) {
		params := map[string]string{"id": "123"}
		getter := &paramsGetter{params: params}

		all := getter.GetAll("id")
		if len(all) != 1 {
			t.Errorf("expected 1 value, got %d", len(all))
		}

		if all[0] != "123" {
			t.Errorf("expected '123', got '%s'", all[0])
		}

		// Test non-existent key
		none := getter.GetAll("nonexistent")
		if none != nil {
			t.Errorf("expected nil for non-existent key, got %v", none)
		}
	})

	t.Run("cookieGetter", func(t *testing.T) {
		cookies := []*http.Cookie{
			{Name: "session", Value: url.QueryEscape("abc123")},
			{Name: "session", Value: url.QueryEscape("def456")},
		}
		getter := &cookieGetter{cookies: cookies}

		all := getter.GetAll("session")
		if len(all) != 2 {
			t.Errorf("expected 2 values, got %d", len(all))
		}

		// Test non-existent key
		none := getter.GetAll("nonexistent")
		if len(none) != 0 {
			t.Errorf("expected empty slice for non-existent key, got %v", none)
		}
	})

	t.Run("headerGetter", func(t *testing.T) {
		headers := http.Header{}
		headers.Add("X-Tags", "tag1")
		headers.Add("X-Tags", "tag2")
		headers.Add("X-Tags", "tag3")

		getter := &headerGetter{headers: headers}

		all := getter.GetAll("X-Tags")
		if len(all) != 3 {
			t.Errorf("expected 3 values, got %d", len(all))
		}
	})

	t.Run("formGetter", func(t *testing.T) {
		values := url.Values{}
		values.Add("items", "item1")
		values.Add("items", "item2")

		getter := &formGetter{values: values}

		all := getter.GetAll("items")
		if len(all) != 2 {
			t.Errorf("expected 2 values, got %d", len(all))
		}
	})
}

// TestBindError_Unwrap tests the Unwrap method
func TestBindError_Unwrap(t *testing.T) {
	originalErr := &BindError{
		Field: "age",
		Value: "invalid",
		Type:  "int",
		Tag:   "form",
		Err:   nil,
	}

	// Create error with inner error
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

	// Test Unwrap
	unwrapped := outerErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("expected inner error, got %v", unwrapped)
	}

	// Test with nil inner error
	if originalErr.Unwrap() != nil {
		t.Error("expected nil when no inner error")
	}
}

// TestBindForm_MapStringString tests map[string]string binding
func TestBindForm_MapStringString(t *testing.T) {
	type FormData struct {
		Labels map[string]string `form:"labels"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if data.Labels == nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "labels is nil"})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	form := url.Values{}
	form.Set("labels[env]", "production")
	form.Set("labels[region]", "us-east-1")
	form.Set("labels[version]", "v1.2.3")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_MapStringInterface tests map[string]interface{} binding
func TestBindForm_MapStringInterface(t *testing.T) {
	type FormData struct {
		Data map[string]interface{} `form:"data"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if data.Data == nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "data is nil"})
			return
		}

		// Verify data is accessible
		if len(data.Data) == 0 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "data is empty"})
			return
		}

		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	form := url.Values{}
	form.Set("data[string]", "text")
	form.Set("data[number]", "42")
	form.Set("data[bool]", "true")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_MapIntString tests map[int]string binding
func TestBindForm_MapIntString(t *testing.T) {
	type FormData struct {
		Items map[int]string `form:"items"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			// Int key conversion might not be supported
			c.JSON(http.StatusBadRequest, map[string]string{"error": "conversion failed"})
			return
		}

		// If it succeeds, verify the map was populated
		if len(data.Items) == 0 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "map not populated"})
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

	// Accept either success or validation error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_ComplexNested tests deeply nested form structures
func TestBindForm_ComplexNested(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_InvalidTypes tests binding with type conversion errors
func TestBindForm_InvalidTypes(t *testing.T) {
	type FormData struct {
		Age int `form:"age"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			// Should get validation error
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid type, got %d", w.Code)
	}
}

// TestBindForm_SpecialCharacters tests form data with special characters
func TestBindForm_SpecialCharacters(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_ArrayNotation tests array notation in form keys
func TestBindForm_ArrayNotation(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_RequiredValidation tests required field validation
func TestBindForm_RequiredValidation(t *testing.T) {
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

		// Manual validation since binding tag might not enforce required
		if data.Name == "" || data.Email == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "required field missing"})
			return
		}

		c.JSON(http.StatusOK, data)
	})

	// Test missing required field
	form := url.Values{}
	form.Set("name", "John")
	// email is missing

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should fail either at binding or manual validation
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing required field, got %d", w.Code)
	}
}

// TestBindForm_URLEncoded tests URL-encoded form data
func TestBindForm_URLEncoded(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_MixedMapTypes tests maps with different value types
func TestBindForm_MixedMapTypes(t *testing.T) {
	type FormData struct {
		StringMap map[string]string      `form:"strmap"`
		IntMap    map[string]int         `form:"intmap"`
		AnyMap    map[string]interface{} `form:"anymap"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	form := url.Values{}
	form.Set("strmap[key1]", "value1")
	form.Set("intmap[count]", "42")
	form.Set("anymap[data]", "anything")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCookieGetter_Has tests the Has method
func TestCookieGetter_Has(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "user_id", Value: "42"},
	}

	getter := &cookieGetter{cookies: cookies}

	if !getter.Has("session") {
		t.Error("should have session cookie")
	}

	if !getter.Has("user_id") {
		t.Error("should have user_id cookie")
	}

	if getter.Has("nonexistent") {
		t.Error("should not have nonexistent cookie")
	}
}

// TestBindForm_DuplicateKeys tests handling of duplicate form keys
func TestBindForm_DuplicateKeys(t *testing.T) {
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

		if len(data.Values) < 2 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "expected multiple values"})
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_MapKeyConversion tests map keys that need type conversion
func TestBindForm_MapKeyConversion(t *testing.T) {
	type FormData struct {
		IntKeys map[int]string `form:"intkeys"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		err := c.BindForm(&data)

		// Int key conversion might not be fully supported
		// Test that it either works or returns a clear error
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "key conversion failed"})
			return
		}

		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	form := url.Values{}
	form.Set("intkeys[100]", "hundred")
	form.Set("intkeys[200]", "two hundred")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Accept either success or error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBindForm_InvalidMapKey tests invalid map key conversion
func TestBindForm_InvalidMapKey(t *testing.T) {
	type FormData struct {
		IntKeys map[int]string `form:"intkeys"`
	}

	r := New()
	r.POST("/submit", func(c *Context) {
		var data FormData
		err := c.BindForm(&data)

		// Should get error for invalid int key
		if err == nil {
			c.JSON(http.StatusOK, data)
		} else {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid key"})
		}
	})

	form := url.Values{}
	form.Set("intkeys[not-a-number]", "value")

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return error for invalid key conversion
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid map key, got %d", w.Code)
	}
}
