package basicauth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name           string
		setupAuth      func() router.HandlerFunc
		authHeader     string
		expectedStatus int
		expectedBody   string
		checkHeader    bool
	}{
		{
			name: "valid credentials",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret")),
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name: "invalid password",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:wrong")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "invalid username",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("nobody:secret")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "missing auth header",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "malformed auth header",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Bearer token123",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "invalid base64",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic !!invalid-base64!!",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "missing colon in credentials",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("adminonly")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "custom realm",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{"user": "pass"}),
					WithRealm("Admin Area"),
				)
			},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "multiple users",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret1",
						"user":  "secret2",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("user:secret2")),
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.New()
			r.Use(tt.setupAuth())
			r.GET("/test", func(c *router.Context) {
				c.String(http.StatusOK, "success")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && w.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, w.Body.String())
			}

			if tt.checkHeader {
				wwwAuth := w.Header().Get("WWW-Authenticate")
				if wwwAuth == "" {
					t.Error("expected WWW-Authenticate header to be set")
				}
			}
		})
	}
}

func TestBasicAuthWithValidator(t *testing.T) {
	validUsers := map[string]string{
		"admin": "password123",
		"user":  "pass456",
	}

	r := router.New()
	r.Use(New(
		WithValidator(func(username, password string) bool {
			expectedPassword, exists := validUsers[username]
			return exists && password == expectedPassword
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	// Valid credentials
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:password123")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Invalid credentials
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:wrong")))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestBasicAuthSkipPaths(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{"admin": "secret"}),
		WithSkipPaths("/health", "/public"),
	))
	r.GET("/health", func(c *router.Context) {
		c.String(http.StatusOK, "healthy")
	})
	r.GET("/protected", func(c *router.Context) {
		c.String(http.StatusOK, "protected")
	})

	// Skipped path - no auth required
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for skipped path, got %d", w.Code)
	}

	// Protected path - auth required
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for protected path, got %d", w.Code)
	}
}

func TestBasicAuthCustomUnauthorizedHandler(t *testing.T) {
	customCalled := false
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{"admin": "secret"}),
		WithUnauthorizedHandler(func(c *router.Context) {
			customCalled = true
			c.String(http.StatusUnauthorized, "custom unauthorized")
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !customCalled {
		t.Error("custom unauthorized handler was not called")
	}

	if w.Body.String() != "custom unauthorized" {
		t.Errorf("expected custom response, got %q", w.Body.String())
	}
}

func TestGetAuthUsername(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{"testuser": "testpass"}),
	))
	r.GET("/test", func(c *router.Context) {
		username := GetUsername(c)
		c.String(http.StatusOK, "user:%s", username)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("testuser:testpass")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expectedBody := "user:testuser"
	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestBasicAuthEmptyPassword(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{
			"user": "",
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	// Valid empty password
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty password, got %d", w.Code)
	}
}

func TestBasicAuthSpecialCharacters(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{
			"user@example.com": "p@ss:w0rd!",
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user@example.com:p@ss:w0rd!")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for special characters, got %d", w.Code)
	}
}

// Benchmark BasicAuth middleware
func BenchmarkBasicAuth(b *testing.B) {
	r := router.New()
	r.Use(New(
		WithUsers(map[string]string{
			"admin": "secret123",
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBasicAuthWithValidator(b *testing.B) {
	validUsers := map[string]string{
		"admin": "secret123",
	}

	r := router.New()
	r.Use(New(
		WithValidator(func(username, password string) bool {
			expectedPassword, exists := validUsers[username]
			return exists && password == expectedPassword
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
