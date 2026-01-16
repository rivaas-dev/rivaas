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

package requestid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/router"
)

func TestRequestID_GeneratesID(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "Expected X-Request-ID header to be set")

	// Default generator produces 32 character hex string (16 bytes * 2)
	assert.Len(t, requestID, 32)
}

func TestRequestID_ClientIDHandling(t *testing.T) {
	t.Parallel()
	clientID := "client-provided-id-123"

	tests := []struct {
		name         string
		allowClient  bool
		setClientID  bool
		expectClient bool
	}{
		{
			name:         "allow client ID",
			allowClient:  true,
			setClientID:  true,
			expectClient: true,
		},
		{
			name:         "disallow client ID",
			allowClient:  false,
			setClientID:  true,
			expectClient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := router.MustNew()
			r.Use(New(WithAllowClientID(tt.allowClient)))
			r.GET("/test", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.setClientID {
				req.Header.Set("X-Request-ID", clientID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			requestID := w.Header().Get("X-Request-ID")
			assert.NotEmpty(t, requestID, "Request ID should be set")

			if tt.expectClient {
				assert.Equal(t, clientID, requestID)
			} else {
				assert.NotEqual(t, clientID, requestID)
			}
		})
	}
}

func TestRequestID_Configuration(t *testing.T) {
	t.Parallel()

	counter := 0
	customGenerator := func() string {
		counter++
		return "custom-id-" + string(rune('0'+counter))
	}

	tests := []struct {
		name              string
		options           []func() Option
		expectedHeader    string
		headerShouldExist bool
		checkUnique       bool
		checkPrefix       string
		requestCount      int
	}{
		{
			name: "custom header",
			options: []func() Option{
				func() Option { return WithHeader("X-Correlation-ID") },
			},
			expectedHeader:    "X-Correlation-ID",
			headerShouldExist: true,
		},
		{
			name: "custom generator produces unique IDs",
			options: []func() Option{
				func() Option { return WithGenerator(customGenerator) },
			},
			expectedHeader: "X-Request-ID",
			checkPrefix:    "custom-id-",
			checkUnique:    true,
			requestCount:   2,
		},
		{
			name:              "multiple requests generate unique IDs",
			options:           []func() Option{},
			expectedHeader:    "X-Request-ID",
			headerShouldExist: true,
			checkUnique:       true,
			requestCount:      100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := make([]Option, 0, len(tt.options))
			for _, optFunc := range tt.options {
				opts = append(opts, optFunc())
			}

			r := router.MustNew()
			r.Use(New(opts...))
			r.GET("/test", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			if tt.checkUnique {
				ids := make(map[string]bool)
				count := tt.requestCount
				if count == 0 {
					count = 1
				}

				for range count {
					req := httptest.NewRequest(http.MethodGet, "/test", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					requestID := w.Header().Get(tt.expectedHeader)
					assert.NotEmpty(t, requestID, "Request ID should be generated")

					if tt.checkPrefix != "" {
						assert.True(t, strings.HasPrefix(requestID, tt.checkPrefix))
					}

					if count > 1 {
						assert.False(t, ids[requestID], "Duplicate request ID: %s", requestID)
						ids[requestID] = true
					}
				}

				if count > 1 {
					assert.Len(t, ids, count, "Expected %d unique IDs", count)
				}
			} else {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if tt.headerShouldExist {
					assert.NotEmpty(t, w.Header().Get(tt.expectedHeader))
				}

				// Verify default header is not set when custom header is used
				if tt.expectedHeader != "X-Request-ID" {
					assert.Empty(t, w.Header().Get("X-Request-ID"))
				}
			}
		})
	}
}

func TestRequestID_CombinedOptions(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithHeader("X-Trace-ID"),
		WithAllowClientID(false),
		WithGenerator(func() string {
			return "generated-123"
		}),
	))
	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Try to provide client ID (should be ignored)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-Id", "client-id")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Trace-Id")
	assert.Equal(t, "generated-123", requestID)
}
