package router

import (
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestConstraintValidationDebug(t *testing.T) {
	r := New()

	// Register route with numeric constraint
	r.GET("/users/:id", func(c *Context) {
		t.Logf("Handler executed with id=%s", c.Param("id"))
	}).WhereNumber("id")

	// Test valid request (should match)
	req1 := httptest.NewRequest("GET", "/users/123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	t.Logf("Valid request /users/123: status=%d (expected 200)", w1.Code)
	if w1.Code != 200 {
		t.Errorf("Valid request failed: got %d, want 200", w1.Code)
	}

	// Test invalid request (should NOT match - return 404)
	req2 := httptest.NewRequest("GET", "/users/invalid123", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	t.Logf("Invalid request /users/invalid123: status=%d (expected 404)", w2.Code)
	if w2.Code != 404 {
		t.Errorf("Invalid request succeeded: got %d, want 404", w2.Code)
	}

	// Check template cache
	if r.templateCache != nil {
		r.templateCache.mu.RLock()
		t.Logf("Dynamic templates count: %d", len(r.templateCache.dynamicTemplates))
		if len(r.templateCache.dynamicTemplates) > 0 {
			tmpl := r.templateCache.dynamicTemplates[0]
			t.Logf("Template pattern: %s", tmpl.pattern)
			t.Logf("Template has constraints: %v", tmpl.hasConstraints)
			t.Logf("Template constraints count: %d", len(tmpl.constraints))
			for i, c := range tmpl.constraints {
				if c != nil {
					t.Logf("  Constraint[%d]: pattern=%s", i, c.String())
				} else {
					t.Logf("  Constraint[%d]: nil", i)
				}
			}
		}
		r.templateCache.mu.RUnlock()
	}
}

func TestFastPathConstraintValidation(t *testing.T) {
	// Test the fast path for constraint validation
	constraints := []RouteConstraint{
		{Param: "id", Pattern: mustCompile(`^\d+$`)},
	}

	tmpl := compileRouteTemplate("GET", "/users/:id", []HandlerFunc{func(c *Context) {}}, constraints)

	t.Logf("Template segmentCount: %d", tmpl.segmentCount)
	t.Logf("Template paramPos: %v", tmpl.paramPos)
	t.Logf("Template hasConstraints: %v", tmpl.hasConstraints)
	t.Logf("Template constraints: %d", len(tmpl.constraints))

	// Test valid value
	ctx1 := &Context{}
	matched1 := tmpl.matchAndExtract("/users/123", ctx1)
	t.Logf("Valid /users/123: matched=%v, paramCount=%d", matched1, ctx1.paramCount)
	if !matched1 {
		t.Error("Valid request should match")
	}

	// Test invalid value
	ctx2 := &Context{}
	matched2 := tmpl.matchAndExtract("/users/abc123", ctx2)
	t.Logf("Invalid /users/abc123: matched=%v", matched2)
	if matched2 {
		t.Error("Invalid request should NOT match")
	}
}

func mustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// TestRouteConstraints tests additional constraint validators
func TestRouteConstraints(t *testing.T) {
	r := New()

	r.GET("/alpha/:name", func(c *Context) {
		c.String(200, "name=%s", c.Param("name"))
	}).WhereAlpha("name")

	r.GET("/uuid/:id", func(c *Context) {
		c.String(200, "id=%s", c.Param("id"))
	}).WhereUUID("id")

	tests := []struct {
		name       string
		path       string
		shouldPass bool
		expected   string
	}{
		{"valid alpha", "/alpha/john", true, "name=john"},
		{"invalid alpha with numbers", "/alpha/john123", false, ""},
		{"valid UUID", "/uuid/123e4567-e89b-12d3-a456-426614174000", true, "id=123e4567-e89b-12d3-a456-426614174000"},
		{"invalid UUID", "/uuid/not-a-uuid", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.shouldPass {
				if w.Code != 200 {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
				if w.Body.String() != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, w.Body.String())
				}
			} else {
				if w.Code != 404 {
					t.Errorf("Expected status 404, got %d", w.Code)
				}
			}
		})
	}
}
