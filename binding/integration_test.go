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

package binding_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding"
)

// TestIntegration_QueryParameterBinding tests binding from real HTTP requests
func TestIntegration_QueryParameterBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type SearchParams struct {
		Query    string   `query:"q"`
		Page     int      `query:"page" default:"1"`
		PageSize int      `query:"page_size" default:"20"`
		Tags     []string `query:"tags"`
		Sort     string   `query:"sort"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, err := binding.Query[SearchParams](r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "golang", params.Query)
		assert.Equal(t, 2, params.Page)
		assert.Equal(t, 50, params.PageSize)
		assert.Equal(t, []string{"go", "testing"}, params.Tags)
		assert.Equal(t, "desc", params.Sort)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/search?q=golang&page=2&page_size=50&tags=go&tags=testing&sort=desc", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_JSONBodyBinding tests JSON body binding from HTTP requests
func TestIntegration_JSONBodyBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type CreateUserRequest struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Age      int    `json:"age"`
		IsActive bool   `json:"is_active"` //nolint:tagliatelle // snake_case is intentional for API compatibility
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		user, err := binding.JSON[CreateUserRequest](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "John Doe", user.Name)
		assert.Equal(t, "john@example.com", user.Email)
		assert.Equal(t, 30, user.Age)
		assert.True(t, user.IsActive)
		w.WriteHeader(http.StatusCreated)
	})

	body := `{"name":"John Doe","email":"john@example.com","age":30,"is_active":true}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestIntegration_FormDataBinding tests form data binding from HTTP requests
func TestIntegration_FormDataBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type LoginRequest struct {
		Username   string `form:"username"`
		Password   string `form:"password"`
		RememberMe bool   `form:"remember_me"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		assert.NoError(t, err)

		login, err := binding.Form[LoginRequest](r.PostForm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "testuser", login.Username)
		assert.Equal(t, "secret123", login.Password)
		assert.True(t, login.RememberMe)
		w.WriteHeader(http.StatusOK)
	})

	formData := url.Values{}
	formData.Set("username", "testuser")
	formData.Set("password", "secret123")
	formData.Set("remember_me", "true")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MultiSourceBinding tests binding from multiple sources
func TestIntegration_MultiSourceBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type GetUserRequest struct {
		// From path parameters
		UserID int `path:"user_id"`
		// From query string
		IncludeDetails bool   `query:"details"`
		Format         string `query:"format" default:"json"`
		// From headers
		Authorization string `header:"Authorization"`
		RequestID     string `header:"X-Request-ID"` //nolint:tagliatelle // Standard HTTP header format
		// From cookies
		SessionID string `cookie:"session_id"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate path parameter extraction (normally done by router)
		pathParams := map[string]string{"user_id": "123"}

		req, err := binding.Bind[GetUserRequest](
			binding.FromPath(pathParams),
			binding.FromQuery(r.URL.Query()),
			binding.FromHeader(r.Header),
			binding.FromCookie(r.Cookies()),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, 123, req.UserID)
		assert.True(t, req.IncludeDetails)
		assert.Equal(t, "xml", req.Format)
		assert.Equal(t, "Bearer token123", req.Authorization)
		assert.Equal(t, "req-abc-123", req.RequestID)
		assert.Equal(t, "sess-xyz-789", req.SessionID)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123?details=true&format=xml", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Request-ID", "req-abc-123")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "sess-xyz-789"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_HeaderAndCookieBinding tests header and cookie binding
func TestIntegration_HeaderAndCookieBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type RequestMetadata struct {
		UserAgent     string `header:"User-Agent"`
		AcceptLang    string `header:"Accept-Language"`
		Authorization string `header:"Authorization"`
		SessionID     string `cookie:"session_id"`
		Theme         string `cookie:"theme"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta, err := binding.Bind[RequestMetadata](
			binding.FromHeader(r.Header),
			binding.FromCookie(r.Cookies()),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "TestAgent/1.0", meta.UserAgent)
		assert.Equal(t, "en-US,en;q=0.9", meta.AcceptLang)
		assert.Equal(t, "Bearer abc123", meta.Authorization)
		assert.Equal(t, "session-123", meta.SessionID)
		assert.Equal(t, "dark", meta.Theme)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/metadata", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Authorization", "Bearer abc123")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "session-123"})
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ErrorHandling tests error responses for invalid input
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name           string
		url            string
		wantStatusCode int
		wantErrMsg     string
	}{
		{
			name:           "invalid integer",
			url:            "/search?page=invalid",
			wantStatusCode: http.StatusBadRequest,
			wantErrMsg:     "Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			type SearchParams struct {
				Page int    `query:"page"`
				Sort string `query:"sort"`
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := binding.Query[SearchParams](r.URL.Query())
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatusCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantErrMsg)
		})
	}
}

// TestIntegration_BinderReuse tests that Binder instances can be reused across multiple requests
func TestIntegration_BinderReuse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name  string `query:"name"`
		Value int    `query:"value" default:"100"`
	}

	binder := binding.MustNew(
		binding.WithMaxDepth(16),
	)

	// Test sequential reuse of the same binder
	for i := range 10 {
		values := url.Values{}
		values.Set("name", "test")
		values.Set("value", strconv.Itoa(i*10))

		var cfg Config
		err := binder.QueryTo(values, &cfg)
		require.NoError(t, err, "request %d failed", i)
		assert.Equal(t, "test", cfg.Name)
		assert.Equal(t, i*10, cfg.Value)
	}

	// Test with defaults
	values := url.Values{}
	values.Set("name", "only-name")

	var cfg Config
	err := binder.QueryTo(values, &cfg)
	require.NoError(t, err)
	assert.Equal(t, "only-name", cfg.Name)
	assert.Equal(t, 100, cfg.Value) // default value
}

// TestIntegration_NestedStructs tests nested struct binding from HTTP requests
func TestIntegration_NestedStructs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Address struct {
		Street string `query:"street"`
		City   string `query:"city"`
		ZIP    string `query:"zip"`
	}

	type CreateOrderRequest struct {
		CustomerName string  `query:"name"`
		Shipping     Address `query:"shipping"`
		Billing      Address `query:"billing"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order, err := binding.Query[CreateOrderRequest](r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "John", order.CustomerName)
		assert.Equal(t, "123 Main St", order.Shipping.Street)
		assert.Equal(t, "NYC", order.Shipping.City)
		assert.Equal(t, "10001", order.Shipping.ZIP)
		assert.Equal(t, "456 Oak Ave", order.Billing.Street)
		w.WriteHeader(http.StatusOK)
	})

	queryString := "name=John&shipping.street=123+Main+St&shipping.city=NYC&shipping.zip=10001&billing.street=456+Oak+Ave&billing.city=LA&billing.zip=90001"
	req := httptest.NewRequest(http.MethodGet, "/orders?"+queryString, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_DefaultValues tests default value application
func TestIntegration_DefaultValues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type PaginationParams struct {
		Page     int    `query:"page" default:"1"`
		PageSize int    `query:"page_size" default:"20"`
		Sort     string `query:"sort" default:"created_at"`
		Order    string `query:"order" default:"desc"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, err := binding.Query[PaginationParams](r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Only page is provided, others should use defaults
		assert.Equal(t, 5, params.Page)
		assert.Equal(t, 20, params.PageSize)
		assert.Equal(t, "created_at", params.Sort)
		assert.Equal(t, "desc", params.Order)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/items?page=5", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_XMLBodyBinding tests XML body binding from HTTP requests
func TestIntegration_XMLBodyBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Product struct {
		Name  string  `xml:"name"`
		Price float64 `xml:"price"`
		SKU   string  `xml:"sku"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(r.Body)
		assert.NoError(t, err)

		product, err := binding.XML[Product](buf.Bytes())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Widget", product.Name)
		assert.InDelta(t, 29.99, product.Price, 0.001)
		assert.Equal(t, "WGT-001", product.SKU)
		w.WriteHeader(http.StatusCreated)
	})

	body := `<Product><name>Widget</name><price>29.99</price><sku>WGT-001</sku></Product>`
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestIntegration_ComplexTypesBinding tests binding of complex types (time, duration)
func TestIntegration_ComplexTypesBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type EventParams struct {
		StartDate time.Time     `query:"start"`
		EndDate   time.Time     `query:"end"`
		Duration  time.Duration `query:"duration"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, err := binding.Query[EventParams](r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.False(t, params.StartDate.IsZero())
		assert.False(t, params.EndDate.IsZero())
		assert.Equal(t, 2*time.Hour, params.Duration)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/events?start=2024-01-15T10:00:00Z&end=2024-01-15T12:00:00Z&duration=2h", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MapBinding tests map binding from query parameters
func TestIntegration_MapBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type FilterParams struct {
		Filters map[string]string `query:"filter"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, err := binding.Query[FilterParams](r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.NotNil(t, params.Filters)
		assert.Equal(t, "active", params.Filters["status"])
		assert.Equal(t, "electronics", params.Filters["category"])
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/items?filter.status=active&filter.category=electronics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
