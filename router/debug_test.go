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
	// Test the TIER 3 fast path directly
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
