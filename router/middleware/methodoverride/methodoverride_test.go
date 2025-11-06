package methodoverride

import (
	"context"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func TestMethodOverride_Header(t *testing.T) {
	// Test middleware directly
	handler := New()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Verify method was overridden
	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}

	// Verify original method is stored
	originalMethod := GetOriginalMethod(c)
	if originalMethod != "POST" {
		t.Errorf("Expected original method POST, got %s", originalMethod)
	}
}

func TestMethodOverride_QueryParam(t *testing.T) {
	handler := New()
	req := httptest.NewRequest("POST", "/test?_method=PATCH", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Verify method was overridden
	if c.Request.Method != "PATCH" {
		t.Errorf("Expected method PATCH, got %s", c.Request.Method)
	}
}

func TestMethodOverride_HeaderTakesPrecedence(t *testing.T) {
	handler := New()

	// POST request with both header and query param - header should win
	req := httptest.NewRequest("POST", "/test?_method=PATCH", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Verify method was overridden to PUT (header takes precedence)
	if c.Request.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", c.Request.Method)
	}
}

func TestMethodOverride_OnlyOnFiltering(t *testing.T) {
	handler := New(WithOnlyOn("POST"))

	// GET request should not trigger override (not in OnlyOn list)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Method should not be overridden
	if c.Request.Method != "GET" {
		t.Errorf("Expected method GET (not overridden), got %s", c.Request.Method)
	}

	// POST request should trigger override
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Method should be overridden
	if c.Request.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", c.Request.Method)
	}
}

func TestMethodOverride_AllowList(t *testing.T) {
	handler := New(WithAllow("PUT", "DELETE"))

	// POST with PATCH override - should be ignored (not in allow list)
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PATCH")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Method should not be overridden (PATCH not in allow list)
	if c.Request.Method != "POST" {
		t.Errorf("Expected method POST (not overridden), got %s", c.Request.Method)
	}

	// POST with PUT override - should work (in allow list)
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Method should be overridden
	if c.Request.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", c.Request.Method)
	}
}

func TestMethodOverride_CaseInsensitive(t *testing.T) {
	handler := New()

	// Lowercase override method
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "delete")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}

	// Mixed case
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DeLeTe")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}
}

func TestMethodOverride_RespectBody(t *testing.T) {
	handler := New(WithRespectBody(true))

	// POST request without body - should not override
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Method should not be overridden (no body)
	if c.Request.Method != "POST" {
		t.Errorf("Expected method POST (not overridden), got %s", c.Request.Method)
	}

	// POST request with body - should override
	req = httptest.NewRequest("POST", "/test", nil)
	req.ContentLength = 10
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Method should be overridden
	if c.Request.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", c.Request.Method)
	}
}

func TestMethodOverride_CSRFRequired(t *testing.T) {
	handler := New(WithRequireCSRFToken(true))

	// POST request without CSRF verification - should not override
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Method should not be overridden (CSRF not verified)
	if c.Request.Method != "POST" {
		t.Errorf("Expected method POST (not overridden), got %s", c.Request.Method)
	}

	// POST request with CSRF verification - should override
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	req = req.WithContext(context.WithValue(req.Context(), CSRFVerifiedKey, true))
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Method should be overridden
	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}
}

func TestMethodOverride_CustomHeader(t *testing.T) {
	handler := New(WithHeader("X-HTTP-Method"))

	// POST request with custom header
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}
}

func TestMethodOverride_DisabledQueryParam(t *testing.T) {
	handler := New(WithQueryParam(""))

	// POST request with query param - should be ignored
	req := httptest.NewRequest("POST", "/test?_method=DELETE", nil)
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Method should not be overridden (query param disabled)
	if c.Request.Method != "POST" {
		t.Errorf("Expected method POST (not overridden), got %s", c.Request.Method)
	}

	// POST request with header - should work
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Method should be overridden
	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}
}

func TestMethodOverride_EmptyOverride(t *testing.T) {
	handler := New()

	// POST request with empty override header
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Should remain as POST (empty override)
	if c.Request.Method != "POST" {
		t.Errorf("Expected method POST, got %s", c.Request.Method)
	}
}

func TestMethodOverride_WhitespaceTrimmed(t *testing.T) {
	handler := New()

	// POST request with whitespace in override
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "  DELETE  ")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}
}

func TestGetOriginalMethod(t *testing.T) {
	handler := New()

	// Test with original method in context (POST overridden to DELETE)
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	// Verify method was overridden
	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}

	// Verify original method is stored
	originalMethod := GetOriginalMethod(c)
	if originalMethod != "POST" {
		t.Errorf("Expected original method POST, got %s", originalMethod)
	}

	// Test without override (should return current method)
	req = httptest.NewRequest("GET", "/test2", nil)
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	// Since GET is not in OnlyOn list, method won't be overridden
	originalMethod = GetOriginalMethod(c)
	if originalMethod != "GET" {
		t.Errorf("Expected GET, got %s", originalMethod)
	}
}

func TestMethodOverride_DefaultConfig(t *testing.T) {
	handler := New()

	// Test default header
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}

	// Test default query param
	req = httptest.NewRequest("POST", "/test?_method=DELETE", nil)
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)

	if c.Request.Method != "DELETE" {
		t.Errorf("Expected method DELETE, got %s", c.Request.Method)
	}

	// Test default allow list (PUT, PATCH, DELETE)
	// PUT should work
	req = httptest.NewRequest("POST", "/put", nil)
	req.Header.Set("X-HTTP-Method-Override", "PUT")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)
	if c.Request.Method != "PUT" {
		t.Errorf("Expected method PUT, got %s", c.Request.Method)
	}

	// PATCH should work
	req = httptest.NewRequest("POST", "/patch", nil)
	req.Header.Set("X-HTTP-Method-Override", "PATCH")
	w = httptest.NewRecorder()

	c = router.NewContext(w, req)
	handler(c)
	if c.Request.Method != "PATCH" {
		t.Errorf("Expected method PATCH, got %s", c.Request.Method)
	}
}
