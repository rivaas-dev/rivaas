package trailingslash

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rivaas.dev/router"
)

func TestTrailingSlashRemove(t *testing.T) {
	r := router.New()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	// Wrap router with trailing slash handler
	handler := Wrap(r, WithPolicy(PolicyRemove))

	// Test redirect from /users/ to /users
	req := httptest.NewRequest("GET", "/users/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/users" {
		t.Errorf("expected Location /users, got %s", location)
	}

	// Test that /users works correctly
	req2 := httptest.NewRequest("GET", "/users", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestTrailingSlashQueryPreserved(t *testing.T) {
	r := router.New()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	req := httptest.NewRequest("GET", "/users/?page=2&sort=name", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	expected := "/users?page=2&sort=name"
	if location != expected {
		t.Errorf("expected Location %s, got %s", expected, location)
	}
}

func TestTrailingSlashNoLoop(t *testing.T) {
	r := router.New()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	// First request: /users/ → redirects to /users
	req1 := httptest.NewRequest("GET", "/users/", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w1.Code)
	}

	// Second request: /users → no redirect, should work
	req2 := httptest.NewRequest("GET", "/users", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
	if w2.Body.String() != "users" {
		t.Errorf("expected body 'users', got %s", w2.Body.String())
	}
}

func TestTrailingSlashRootPath(t *testing.T) {
	r := router.New()
	r.Use(New())

	r.GET("/", func(c *router.Context) {
		c.String(http.StatusOK, "root")
	})

	// Root path should never be redirected
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "root" {
		t.Errorf("expected body 'root', got %s", w.Body.String())
	}
}

func TestTrailingSlashAdd(t *testing.T) {
	r := router.New()
	r.GET("/users/", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyAdd))

	// /users should redirect to /users/
	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/users/" {
		t.Errorf("expected Location /users/, got %s", location)
	}

	// /users/ should work
	req2 := httptest.NewRequest("GET", "/users/", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestTrailingSlashStrict(t *testing.T) {
	r := router.New()
	r.Use(New(WithPolicy(PolicyStrict)))

	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	// /users/ should return 404 in strict mode
	req := httptest.NewRequest("GET", "/users/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	// /users should work
	req2 := httptest.NewRequest("GET", "/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestTrailingSlashTrimSuffixNotTrimRight(t *testing.T) {
	r := router.New()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	// Test that multiple slashes don't collapse incorrectly
	// /users// should redirect to /users/ (then to /users on second request)
	// But we only remove one slash, so /users// → /users/ → 404 (since route is /users)
	req := httptest.NewRequest("GET", "/users//", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should redirect to /users/ (one slash removed)
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	// TrimSuffix removes exactly one slash, so /users// → /users/
	if location != "/users/" {
		t.Errorf("expected Location /users/, got %s", location)
	}
}

func TestTrailingSlashPreservesMethod(t *testing.T) {
	r := router.New()
	r.POST("/users", func(c *router.Context) {
		c.String(http.StatusOK, "created")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	// POST to /users/ should redirect with 308 (preserves method)
	req := httptest.NewRequest("POST", "/users/", strings.NewReader("data"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("expected 308, got %d", w.Code)
	}

	// 308 preserves method, so client should retry POST to /users
	location := w.Header().Get("Location")
	if location != "/users" {
		t.Errorf("expected Location /users, got %s", location)
	}
}
