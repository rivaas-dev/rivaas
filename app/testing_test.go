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

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/tracing"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// TestApp_Test tests the core Test() method functionality.
func TestApp_Test(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupApp      func() *App
		setupRoute    func(*App)
		makeRequest   func() *http.Request
		opts          []TestOption
		wantStatus    int
		wantErr       bool
		checkResponse func(*testing.T, *http.Response)
	}{
		{
			name: "successful GET request",
			setupApp: func() *App {
				return MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			},
			setupRoute: func(app *App) {
				app.GET("/users/:id", func(c *Context) {
					if err := c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")}); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			makeRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/users/123", nil)
			},
			wantStatus: 200,
		},
		{
			name: "POST request with body",
			setupApp: func() *App {
				return MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			},
			setupRoute: func(app *App) {
				app.POST("/users", func(c *Context) {
					if err := c.String(http.StatusCreated, "created"); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			makeRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Alice"}`))
			},
			wantStatus: 201,
		},
		{
			name: "404 not found",
			setupApp: func() *App {
				return MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			},
			setupRoute: func(app *App) {
				// No routes registered
			},
			makeRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
			},
			wantStatus: 404,
		},
		{
			name: "route with path parameters",
			setupApp: func() *App {
				return MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			},
			setupRoute: func(app *App) {
				app.GET("/posts/:id/comments/:commentId", func(c *Context) {
					if err := c.JSON(http.StatusOK, map[string]string{
						"postId":    c.Param("id"),
						"commentId": c.Param("commentId"),
					}); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			makeRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/posts/1/comments/2", nil)
			},
			wantStatus: 200,
			checkResponse: func(t *testing.T, resp *http.Response) {
				t.Helper()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				var data map[string]string
				err = json.Unmarshal(body, &data)
				require.NoError(t, err)
				assert.Equal(t, "1", data["postId"])
				assert.Equal(t, "2", data["commentId"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setupApp()
			tt.setupRoute(app)

			resp, err := app.Test(tt.makeRequest(), tt.opts...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			defer resp.Body.Close()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestApp_Test_Timeout tests timeout functionality with context cancellation.
func TestApp_Test_Timeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		timeout      time.Duration
		handlerDelay time.Duration
		wantErr      bool
		errContains  string
	}{
		{
			name:         "request completes within timeout",
			timeout:      100 * time.Millisecond,
			handlerDelay: 10 * time.Millisecond,
			wantErr:      false,
		},
		{
			name:         "request exceeds timeout",
			timeout:      50 * time.Millisecond,
			handlerDelay: 200 * time.Millisecond,
			wantErr:      true,
			errContains:  "timeout",
		},
		{
			name:         "no timeout (-1)",
			timeout:      -1,
			handlerDelay: 100 * time.Millisecond,
			wantErr:      false,
		},
		{
			name:         "default timeout (1s)",
			timeout:      0, // uses default
			handlerDelay: 50 * time.Millisecond,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))

			app.GET("/slow", func(c *Context) {
				time.Sleep(tt.handlerDelay)
				if err := c.String(http.StatusOK, "done"); err != nil {
					c.Logger().Error("failed to write response", "err", err)
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/slow", nil)
			var opts []TestOption
			// Always add timeout option, even for -1 (no timeout) and 0 (default)
			// This tests that WithTimeout properly handles all cases
			opts = append(opts, WithTimeout(tt.timeout))

			resp, err := app.Test(req, opts...)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				if resp != nil {
					resp.Body.Close()
				}
			}
		})
	}
}

// TestApp_Test_Context tests custom context handling.
func TestApp_Test_Context(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupCtx   func(*testing.T) context.Context
		wantErr    bool
		checkValue func(*testing.T, *http.Response)
	}{
		{
			name: "context with custom value",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				return context.WithValue(t.Context(), contextKey("key"), "value")
			},
			wantErr: false,
			checkValue: func(t *testing.T, resp *http.Response) {
				t.Helper()
				assert.Equal(t, 200, resp.StatusCode)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, "value", string(body))
			},
		},
		{
			name: "canceled context",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				cancel() // immediately cancel

				return ctx
			},
			wantErr: true,
		},
		{
			name: "context with timeout",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
				_ = cancel // cancel will be called when context expires

				return ctx
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))

			app.GET("/test", func(c *Context) {
				if val := c.Request.Context().Value(contextKey("key")); val != nil {
					if err := c.String(http.StatusOK, val.(string)); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				} else {
					if err := c.String(http.StatusOK, "no value"); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp, err := app.Test(req, WithContext(tt.setupCtx(t)))

			if tt.wantErr {
				require.Error(t, err)
				if resp != nil {
					resp.Body.Close()
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				defer resp.Body.Close()
				if tt.checkValue != nil {
					tt.checkValue(t, resp)
				}
			}
		})
	}
}

// TestApp_TestJSON tests the JSON convenience method.
func TestApp_TestJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		body        any
		setupRoute  func(*App)
		wantStatus  int
		wantErr     bool
		errContains string
		checkResp   func(*testing.T, *http.Response)
	}{
		{
			name:   "POST JSON body",
			method: http.MethodPost,
			path:   "/users",
			body:   map[string]string{"name": "Alice", "email": "alice@example.com"},
			setupRoute: func(app *App) {
				app.POST("/users", func(c *Context) {
					var user map[string]string
					if err := c.Bind(&user); err != nil {
						if writeErr := c.String(http.StatusBadRequest, err.Error()); writeErr != nil {
							c.Logger().Error("failed to write error response", "err", writeErr)
						}

						return
					}
					if err := c.JSON(http.StatusCreated, user); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, resp *http.Response) {
				t.Helper()
				contentType := resp.Header.Get("Content-Type")
				assert.Contains(t, contentType, "application/json")
			},
		},
		{
			name:   "nil body",
			method: http.MethodGet,
			path:   "/users",
			body:   nil,
			setupRoute: func(app *App) {
				app.GET("/users", func(c *Context) {
					if err := c.JSON(http.StatusOK, []string{"user1", "user2"}); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			wantStatus: 200,
		},
		{
			name:        "invalid JSON encoding",
			method:      http.MethodPost,
			path:        "/test",
			body:        make(chan int), // channels can't be JSON encoded
			wantErr:     true,
			errContains: "failed to encode JSON body",
		},
		{
			name:   "PUT with JSON body",
			method: http.MethodPut,
			path:   "/users/:id",
			body:   map[string]any{"name": "Bob", "age": 30},
			setupRoute: func(app *App) {
				app.PUT("/users/:id", func(c *Context) {
					var data map[string]any
					if err := c.Bind(&data); err != nil {
						if writeErr := c.String(http.StatusBadRequest, err.Error()); writeErr != nil {
							c.Logger().Error("failed to write error response", "err", writeErr)
						}

						return
					}
					if err := c.JSON(http.StatusOK, map[string]any{
						"id":   c.Param("id"),
						"data": data,
					}); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			wantStatus: 200,
		},
		{
			name:   "PATCH with JSON body",
			method: http.MethodPatch,
			path:   "/users/:id",
			body:   map[string]string{"status": "active"},
			setupRoute: func(app *App) {
				app.PATCH("/users/:id", func(c *Context) {
					var data map[string]string
					if err := c.Bind(&data); err != nil {
						if writeErr := c.String(http.StatusBadRequest, err.Error()); writeErr != nil {
							c.Logger().Error("failed to write error response", "err", writeErr)
						}

						return
					}
					if err := c.JSON(http.StatusOK, data); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			},
			wantStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))

			if tt.setupRoute != nil {
				tt.setupRoute(app)
			}

			resp, err := app.TestJSON(tt.method, tt.path, tt.body)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				if resp != nil {
					resp.Body.Close()
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			defer resp.Body.Close()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

// mockTestingT implements testingT interface for testing ExpectJSON.
type mockTestingT struct {
	errorCalled bool
	errorMsg    string
}

func (m *mockTestingT) Errorf(format string, args ...any) {
	m.errorCalled = true
	m.errorMsg = fmt.Sprintf(format, args...)
}

// TestExpectJSON tests the JSON assertion helper.
func TestExpectJSON(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name         string
		makeResponse func() *http.Response
		wantStatus   int
		out          any
		expectError  bool
		errorCheck   func(*testing.T, *mockTestingT)
	}{
		{
			name: "valid JSON response",
			makeResponse: func() *http.Response {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.WriteHeader(http.StatusOK)
				json.NewEncoder(rec).Encode(User{Name: "Alice", Email: "alice@example.com"})

				return rec.Result()
			},
			wantStatus:  200,
			out:         &User{},
			expectError: false,
			errorCheck: func(t *testing.T, mockT *mockTestingT) {
				t.Helper()
				assert.False(t, mockT.errorCalled, "expected no errors")
			},
		},
		{
			name: "wrong status code",
			makeResponse: func() *http.Response {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(rec).Encode(map[string]string{"error": "server error"})

				return rec.Result()
			},
			wantStatus:  200,
			out:         &User{},
			expectError: true,
			errorCheck: func(t *testing.T, mockT *mockTestingT) {
				t.Helper()
				assert.True(t, mockT.errorCalled, "expected Errorf to be called")
				assert.Contains(t, mockT.errorMsg, "expected status 200, got 500")
			},
		},
		{
			name: "wrong content type",
			makeResponse: func() *http.Response {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "text/plain")
				rec.WriteHeader(http.StatusOK)
				rec.WriteString("plain text")

				return rec.Result()
			},
			wantStatus:  200,
			out:         &User{},
			expectError: true,
			errorCheck: func(t *testing.T, mockT *mockTestingT) {
				t.Helper()
				assert.True(t, mockT.errorCalled, "expected Errorf to be called")
				assert.Contains(t, mockT.errorMsg, "Content-Type")
			},
		},
		{
			name: "invalid JSON",
			makeResponse: func() *http.Response {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.WriteHeader(http.StatusOK)
				rec.WriteString("{invalid json}")

				return rec.Result()
			},
			wantStatus:  200,
			out:         &User{},
			expectError: true,
			errorCheck: func(t *testing.T, mockT *mockTestingT) {
				t.Helper()
				assert.True(t, mockT.errorCalled, "expected Errorf to be called")
				assert.Contains(t, mockT.errorMsg, "failed to decode JSON")
			},
		},
		{
			name: "successful decode into struct",
			makeResponse: func() *http.Response {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.WriteHeader(http.StatusOK)
				json.NewEncoder(rec).Encode(User{Name: "Bob", Email: "bob@example.com"})

				return rec.Result()
			},
			wantStatus:  200,
			out:         &User{},
			expectError: false,
			errorCheck: func(t *testing.T, mockT *mockTestingT) {
				t.Helper()
				assert.False(t, mockT.errorCalled, "expected no errors")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockT := &mockTestingT{}
			resp := tt.makeResponse()
			defer resp.Body.Close()

			ExpectJSON(mockT, resp, tt.wantStatus, tt.out)

			if tt.expectError {
				assert.True(t, mockT.errorCalled, "expected Errorf to be called")
			} else {
				assert.False(t, mockT.errorCalled, "expected no errors, got: %s", mockT.errorMsg)
			}

			if tt.errorCheck != nil {
				tt.errorCheck(t, mockT)
			}
		})
	}
}

// TestTestOptions tests combining multiple test options.
func TestTestOptions(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.GET("/test", func(c *Context) {
		if err := c.String(200, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	tests := []struct {
		name       string
		setupOpts  func(*testing.T) []TestOption
		wantStatus int
		wantErr    bool
	}{
		{
			name: "multiple options combined",
			setupOpts: func(t *testing.T) []TestOption {
				t.Helper()
				return []TestOption{
					WithTimeout(5 * time.Second),
					WithContext(context.WithValue(t.Context(), contextKey("test"), "value")),
				}
			},
			wantStatus: 200,
			wantErr:    false,
		},
		{
			name: "timeout and context",
			setupOpts: func(t *testing.T) []TestOption {
				t.Helper()
				return []TestOption{
					WithTimeout(1 * time.Second),
					WithContext(t.Context()),
				}
			},
			wantStatus: 200,
			wantErr:    false,
		},
		{
			name: "no options (defaults)",
			setupOpts: func(*testing.T) []TestOption {
				t.Helper()
				return nil
			},
			wantStatus: 200,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp, err := app.Test(req, tt.setupOpts(t)...)

			if tt.wantErr {
				require.Error(t, err)
				if resp != nil {
					resp.Body.Close()
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				defer resp.Body.Close()
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

// TestApp_Test_WithTracingEnabled tests that Test works correctly when tracing is enabled.
func TestApp_Test_WithTracingEnabled(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithObservability(
			WithTracing(tracing.WithNoop()),
		),
	)

	app.GET("/test", func(c *Context) {
		if err := c.String(200, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}
