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

package timeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestTimeout_Behavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		timeout        time.Duration
		handlerDelay   time.Duration
		expectedStatus int
	}{
		{
			name:           "completes within timeout",
			timeout:        100 * time.Millisecond,
			handlerDelay:   0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "exceeds timeout",
			timeout:        50 * time.Millisecond,
			handlerDelay:   200 * time.Millisecond,
			expectedStatus: http.StatusRequestTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := router.MustNew()
			r.Use(New(tt.timeout))
			r.GET("/test", func(c *router.Context) {
				if tt.handlerDelay > 0 {
					// Properly respect context cancellation
					select {
					case <-time.After(tt.handlerDelay):
						c.JSON(http.StatusOK, map[string]string{"message": "ok"})
					case <-c.Request.Context().Done():
						// Context canceled due to timeout - don't write response
						return
					}
				} else {
					c.JSON(http.StatusOK, map[string]string{"message": "ok"})
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestTimeout_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(50 * time.Millisecond))

	contextCancelled := make(chan bool, 1)
	r.GET("/test", func(c *router.Context) {
		// Capture context at start
		ctx := c.Request.Context()

		// Check context multiple times during a long operation
		for range 10 {
			select {
			case <-ctx.Done():
				contextCancelled <- true
				return
			default:
				time.Sleep(20 * time.Millisecond)
			}
		}
		// Only write response if not canceled
		select {
		case <-ctx.Done():
			contextCancelled <- true
			return
		default:
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Wait for handler to signal cancellation or timeout
	select {
	case <-contextCancelled:
		// Good - context was canceled
	case <-time.After(200 * time.Millisecond):
		assert.Fail(t, "Handler should detect context cancellation")
	}

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
}

func TestTimeout_SkipPaths(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(50*time.Millisecond, WithSkipPaths("/long-running")))

	r.GET("/long-running", func(c *router.Context) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	r.GET("/fast", func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(100 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		case <-c.Request.Context().Done():
			return
		}
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"skipped path completes", "/long-running", http.StatusOK},
		{"non-skipped path timeouts", "/fast", http.StatusRequestTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestTimeout_CustomHandler(t *testing.T) {
	t.Parallel()
	customHandlerCalled := false

	r := router.MustNew()
	r.Use(New(30*time.Millisecond,
		WithHandler(func(c *router.Context) {
			customHandlerCalled = true
			c.JSON(http.StatusRequestTimeout, map[string]any{
				"error":   "Custom timeout message",
				"timeout": "30ms",
			})
		}),
	))

	r.GET("/slow", func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(150 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		case <-c.Request.Context().Done():
			return
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Give enough time for timeout to trigger
	time.Sleep(50 * time.Millisecond)

	assert.True(t, customHandlerCalled, "Custom timeout handler should be called")
	assert.Equal(t, http.StatusRequestTimeout, w.Code)
}

func TestTimeout_ContextPropagation(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(100 * time.Millisecond))

	var ctxWithTimeout context.Context
	r.GET("/test", func(c *router.Context) {
		ctxWithTimeout = c.Request.Context()
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.NotNil(t, ctxWithTimeout, "Context should be set")

	// Check that context has deadline
	_, ok := ctxWithTimeout.Deadline()
	assert.True(t, ok, "Context should have deadline set")
}

func TestTimeout_MultipleRequests(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(100 * time.Millisecond))

	fastPath := func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "fast"})
	}

	slowPath := func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(200 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "slow"})
		case <-c.Request.Context().Done():
			return
		}
	}

	r.GET("/fast", fastPath)
	r.GET("/slow", slowPath)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"fast request", "/fast", http.StatusOK},
		{"slow request", "/slow", http.StatusRequestTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
