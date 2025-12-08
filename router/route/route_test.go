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

package route

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router/compiler"
)

// ParseReversePattern Tests

func TestParseReversePattern_Simple(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users")

	require.Len(t, pattern.Segments, 1)
	assert.True(t, pattern.Segments[0].Static)
	assert.Equal(t, "users", pattern.Segments[0].Value)
}

func TestParseReversePattern_WithParameter(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:id")

	require.Len(t, pattern.Segments, 2)

	assert.True(t, pattern.Segments[0].Static)
	assert.Equal(t, "users", pattern.Segments[0].Value)

	assert.False(t, pattern.Segments[1].Static)
	assert.Equal(t, "id", pattern.Segments[1].Value) // ":" should be stripped
}

func TestParseReversePattern_MultipleParameters(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:userId/posts/:postId/comments/:commentId")

	require.Len(t, pattern.Segments, 6)

	// users
	assert.True(t, pattern.Segments[0].Static)
	assert.Equal(t, "users", pattern.Segments[0].Value)

	// :userId -> userId
	assert.False(t, pattern.Segments[1].Static)
	assert.Equal(t, "userId", pattern.Segments[1].Value)

	// posts
	assert.True(t, pattern.Segments[2].Static)
	assert.Equal(t, "posts", pattern.Segments[2].Value)

	// :postId -> postId
	assert.False(t, pattern.Segments[3].Static)
	assert.Equal(t, "postId", pattern.Segments[3].Value)

	// comments
	assert.True(t, pattern.Segments[4].Static)
	assert.Equal(t, "comments", pattern.Segments[4].Value)

	// :commentId -> commentId
	assert.False(t, pattern.Segments[5].Static)
	assert.Equal(t, "commentId", pattern.Segments[5].Value)
}

func TestParseReversePattern_RootPath(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/")

	assert.Empty(t, pattern.Segments)
}

func TestParseReversePattern_TrailingSlash(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:id/")

	require.Len(t, pattern.Segments, 2)
	assert.Equal(t, "users", pattern.Segments[0].Value)
	assert.Equal(t, "id", pattern.Segments[1].Value)
}

// BuildURL Tests

func TestBuildURL_SimpleStatic(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users")
	url, err := pattern.BuildURL(nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "/users", url)
}

func TestBuildURL_WithParameter(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:id")
	url, err := pattern.BuildURL(map[string]string{"id": "123"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "/users/123", url)
}

func TestBuildURL_MultipleParameters(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:userId/posts/:postId")
	url, err := pattern.BuildURL(map[string]string{
		"userId": "42",
		"postId": "99",
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, "/users/42/posts/99", url)
}

func TestBuildURL_MissingParameter(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:id")
	_, err := pattern.BuildURL(map[string]string{}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required parameter: id")
}

func TestBuildURL_WithQueryString(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/users/:id")
	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "10")

	resultURL, err := pattern.BuildURL(map[string]string{"id": "123"}, query)

	require.NoError(t, err)
	assert.Contains(t, resultURL, "/users/123?")
	assert.Contains(t, resultURL, "page=1")
	assert.Contains(t, resultURL, "limit=10")
}

func TestBuildURL_EscapesSpecialCharacters(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/search/:query")
	url, err := pattern.BuildURL(map[string]string{"query": "hello world"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "/search/hello%20world", url)
}

func TestBuildURL_RootPath(t *testing.T) {
	t.Parallel()

	pattern := ParseReversePattern("/")
	url, err := pattern.BuildURL(nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "/", url)
}

// ParamConstraint Tests

func TestParamConstraint_ToRegexConstraint_Int(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintInt}
	constraint := pc.ToRegexConstraint("id")

	require.NotNil(t, constraint)
	assert.Equal(t, "id", constraint.Param)
	assert.NotNil(t, constraint.Pattern)

	// Should match integers
	assert.True(t, constraint.Pattern.MatchString("123"))
	assert.True(t, constraint.Pattern.MatchString("0"))
	assert.True(t, constraint.Pattern.MatchString("9999999"))

	// Should NOT match non-integers
	assert.False(t, constraint.Pattern.MatchString("abc"))
	assert.False(t, constraint.Pattern.MatchString("12.3"))
	assert.False(t, constraint.Pattern.MatchString("-123"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_Float(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintFloat}
	constraint := pc.ToRegexConstraint("price")

	require.NotNil(t, constraint)
	assert.Equal(t, "price", constraint.Param)

	// Should match floats
	assert.True(t, constraint.Pattern.MatchString("123"))
	assert.True(t, constraint.Pattern.MatchString("123.45"))
	assert.True(t, constraint.Pattern.MatchString(".5"))
	assert.True(t, constraint.Pattern.MatchString("-123.45"))
	assert.True(t, constraint.Pattern.MatchString("1e10"))
	assert.True(t, constraint.Pattern.MatchString("1.5E-3"))

	// Should NOT match non-floats
	assert.False(t, constraint.Pattern.MatchString("abc"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_UUID(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintUUID}
	constraint := pc.ToRegexConstraint("uuid")

	require.NotNil(t, constraint)
	assert.Equal(t, "uuid", constraint.Param)

	// Should match valid UUIDs
	assert.True(t, constraint.Pattern.MatchString("123e4567-e89b-12d3-a456-426614174000"))
	assert.True(t, constraint.Pattern.MatchString("550e8400-e29b-41d4-a716-446655440000"))
	assert.True(t, constraint.Pattern.MatchString("6BA7B810-9DAD-11D1-80B4-00C04FD430C8"))

	// Should NOT match invalid UUIDs
	assert.False(t, constraint.Pattern.MatchString("not-a-uuid"))
	assert.False(t, constraint.Pattern.MatchString("123"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_Enum(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{
		Kind: ConstraintEnum,
		Enum: []string{"draft", "published", "archived"},
	}
	constraint := pc.ToRegexConstraint("status")

	require.NotNil(t, constraint)
	assert.Equal(t, "status", constraint.Param)

	// Should match enum values
	assert.True(t, constraint.Pattern.MatchString("draft"))
	assert.True(t, constraint.Pattern.MatchString("published"))
	assert.True(t, constraint.Pattern.MatchString("archived"))

	// Should NOT match other values
	assert.False(t, constraint.Pattern.MatchString("deleted"))
	assert.False(t, constraint.Pattern.MatchString("DRAFT")) // case-sensitive
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_Regex(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{
		Kind:    ConstraintRegex,
		Pattern: `[a-z]+`,
	}
	constraint := pc.ToRegexConstraint("slug")

	require.NotNil(t, constraint)
	assert.Equal(t, "slug", constraint.Param)

	// Should match pattern
	assert.True(t, constraint.Pattern.MatchString("hello"))
	assert.True(t, constraint.Pattern.MatchString("world"))

	// Should NOT match non-matching values
	assert.False(t, constraint.Pattern.MatchString("Hello"))
	assert.False(t, constraint.Pattern.MatchString("123"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_Date(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintDate}
	constraint := pc.ToRegexConstraint("date")

	require.NotNil(t, constraint)
	assert.Equal(t, "date", constraint.Param)

	// Should match dates
	assert.True(t, constraint.Pattern.MatchString("2025-12-01"))
	assert.True(t, constraint.Pattern.MatchString("2000-01-01"))

	// Should NOT match non-dates
	assert.False(t, constraint.Pattern.MatchString("2025-1-1"))
	assert.False(t, constraint.Pattern.MatchString("12-01-2025"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_DateTime(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintDateTime}
	constraint := pc.ToRegexConstraint("timestamp")

	require.NotNil(t, constraint)
	assert.Equal(t, "timestamp", constraint.Param)

	// Should match datetimes
	assert.True(t, constraint.Pattern.MatchString("2025-12-01T10:30:00Z"))
	assert.True(t, constraint.Pattern.MatchString("2025-12-01T10:30:00+01:00"))
	assert.True(t, constraint.Pattern.MatchString("2025-12-01T10:30:00.123Z"))

	// Should NOT match non-datetimes
	assert.False(t, constraint.Pattern.MatchString("2025-12-01"))
	assert.False(t, constraint.Pattern.MatchString("10:30:00"))
	assert.False(t, constraint.Pattern.MatchString(""))
}

func TestParamConstraint_ToRegexConstraint_None(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintNone}
	constraint := pc.ToRegexConstraint("param")

	assert.Nil(t, constraint)
}

func TestParamConstraint_Compile(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{
		Kind:    ConstraintRegex,
		Pattern: `[0-9]+`,
	}

	// Before compile, re should be nil
	assert.Nil(t, pc.re)

	pc.Compile()

	// After compile, re should be set
	require.NotNil(t, pc.re)
	assert.True(t, pc.re.MatchString("123"))
	assert.False(t, pc.re.MatchString("abc"))
}

func TestParamConstraint_Compile_InvalidRegex(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{
		Kind:    ConstraintRegex,
		Pattern: `[invalid`, // Invalid regex
	}

	pc.Compile()

	// Should not panic, re remains nil
	assert.Nil(t, pc.re)
}

func TestParamConstraint_Compile_NonRegex(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{Kind: ConstraintInt}

	pc.Compile()

	// Should not set re for non-regex constraints
	assert.Nil(t, pc.re)
}

// =============================================================================
// Enum Special Characters Test
// =============================================================================

func TestParamConstraint_ToRegexConstraint_EnumWithSpecialChars(t *testing.T) {
	t.Parallel()

	pc := ParamConstraint{
		Kind: ConstraintEnum,
		Enum: []string{"a+b", "c.d", "e*f"},
	}
	constraint := pc.ToRegexConstraint("special")

	require.NotNil(t, constraint)

	// Should match exact values (special chars escaped)
	assert.True(t, constraint.Pattern.MatchString("a+b"))
	assert.True(t, constraint.Pattern.MatchString("c.d"))
	assert.True(t, constraint.Pattern.MatchString("e*f"))

	// Should NOT match regex interpretations
	assert.False(t, constraint.Pattern.MatchString("ab"))   // + is not "one or more"
	assert.False(t, constraint.Pattern.MatchString("cxd"))  // . is not "any char"
	assert.False(t, constraint.Pattern.MatchString("ef"))   // * is not "zero or more"
	assert.False(t, constraint.Pattern.MatchString("eeef")) // * is not "zero or more"
}

// ParamConstraint Table-Driven Tests

func TestParamConstraint_ToRegexConstraint_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		constraint     ParamConstraint
		paramName      string
		shouldMatch    []string
		shouldNotMatch []string
		expectNil      bool
	}{
		{
			name:           "int constraint",
			constraint:     ParamConstraint{Kind: ConstraintInt},
			paramName:      "id",
			shouldMatch:    []string{"123", "0", "9999999", "42"},
			shouldNotMatch: []string{"abc", "12.3", "-123", "", "1.0"},
		},
		{
			name:           "float constraint",
			constraint:     ParamConstraint{Kind: ConstraintFloat},
			paramName:      "price",
			shouldMatch:    []string{"123", "123.45", ".5", "-123.45", "1e10", "1.5E-3"},
			shouldNotMatch: []string{"abc", ""},
		},
		{
			name:           "UUID constraint",
			constraint:     ParamConstraint{Kind: ConstraintUUID},
			paramName:      "uuid",
			shouldMatch:    []string{"123e4567-e89b-12d3-a456-426614174000", "550e8400-e29b-41d4-a716-446655440000"},
			shouldNotMatch: []string{"not-a-uuid", "123", "", "550e8400e29b41d4a716446655440000"},
		},
		{
			name:           "date constraint",
			constraint:     ParamConstraint{Kind: ConstraintDate},
			paramName:      "date",
			shouldMatch:    []string{"2025-12-01", "2000-01-01", "1999-12-31"},
			shouldNotMatch: []string{"2025-1-1", "12-01-2025", "", "2025/12/01"},
		},
		{
			name:           "datetime constraint",
			constraint:     ParamConstraint{Kind: ConstraintDateTime},
			paramName:      "timestamp",
			shouldMatch:    []string{"2025-12-01T10:30:00Z", "2025-12-01T10:30:00+01:00", "2025-12-01T10:30:00.123Z"},
			shouldNotMatch: []string{"2025-12-01", "10:30:00", ""},
		},
		{
			name: "enum constraint",
			constraint: ParamConstraint{
				Kind: ConstraintEnum,
				Enum: []string{"draft", "published", "archived"},
			},
			paramName:      "status",
			shouldMatch:    []string{"draft", "published", "archived"},
			shouldNotMatch: []string{"deleted", "DRAFT", "", "pending"},
		},
		{
			name: "regex constraint",
			constraint: ParamConstraint{
				Kind:    ConstraintRegex,
				Pattern: `[a-z]+`,
			},
			paramName:      "slug",
			shouldMatch:    []string{"hello", "world", "abc"},
			shouldNotMatch: []string{"Hello", "123", "", "WORLD"},
		},
		{
			name:        "none constraint returns nil",
			constraint:  ParamConstraint{Kind: ConstraintNone},
			paramName:   "param",
			expectNil:   true,
			shouldMatch: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			constraint := tt.constraint.ToRegexConstraint(tt.paramName)

			if tt.expectNil {
				assert.Nil(t, constraint)
				return
			}

			require.NotNil(t, constraint, "constraint should not be nil")
			assert.Equal(t, tt.paramName, constraint.Param)

			for _, s := range tt.shouldMatch {
				assert.True(t, constraint.Pattern.MatchString(s), "expected %q to match for %s", s, tt.name)
			}
			for _, s := range tt.shouldNotMatch {
				assert.False(t, constraint.Pattern.MatchString(s), "expected %q NOT to match for %s", s, tt.name)
			}
		})
	}
}

// ConstraintFromPattern Tests

func TestConstraintFromPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		param       string
		pattern     string
		shouldMatch []string
		shouldNot   []string
	}{
		{
			name:        "numeric pattern",
			param:       "id",
			pattern:     `\d+`,
			shouldMatch: []string{"123", "0", "999"},
			shouldNot:   []string{"abc", "", "12a"},
		},
		{
			name:        "alphanumeric pattern",
			param:       "slug",
			pattern:     `[a-zA-Z0-9]+`,
			shouldMatch: []string{"hello", "Hello123", "ABC"},
			shouldNot:   []string{"hello-world", "", "hello world"},
		},
		{
			name:        "UUID pattern",
			param:       "uuid",
			pattern:     `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			shouldMatch: []string{"550e8400-e29b-41d4-a716-446655440000"},
			shouldNot:   []string{"not-a-uuid", "123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			constraint := ConstraintFromPattern(tt.param, tt.pattern)

			assert.Equal(t, tt.param, constraint.Param)
			require.NotNil(t, constraint.Pattern)

			for _, s := range tt.shouldMatch {
				assert.True(t, constraint.Pattern.MatchString(s), "expected %q to match", s)
			}
			for _, s := range tt.shouldNot {
				assert.False(t, constraint.Pattern.MatchString(s), "expected %q NOT to match", s)
			}
		})
	}
}

func TestConstraintFromPattern_InvalidRegex(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		ConstraintFromPattern("param", `[invalid`)
	}, "should panic on invalid regex pattern")
}

// Mount Option Tests

func TestInheritMiddleware(t *testing.T) {
	t.Parallel()

	opt := InheritMiddleware()
	cfg := &MountConfig{}

	opt(cfg)

	assert.True(t, cfg.InheritMiddleware)
}

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	handler1 := "handler1"
	handler2 := "handler2"

	opt := WithMiddleware(handler1, handler2)
	cfg := &MountConfig{}

	opt(cfg)

	require.Len(t, cfg.ExtraMiddleware, 2)
	assert.Equal(t, handler1, cfg.ExtraMiddleware[0])
	assert.Equal(t, handler2, cfg.ExtraMiddleware[1])
}

func TestWithMiddleware_Append(t *testing.T) {
	t.Parallel()

	handler1 := "handler1"
	handler2 := "handler2"
	handler3 := "handler3"

	cfg := &MountConfig{}
	WithMiddleware(handler1)(cfg)
	WithMiddleware(handler2, handler3)(cfg)

	require.Len(t, cfg.ExtraMiddleware, 3)
	assert.Equal(t, handler1, cfg.ExtraMiddleware[0])
	assert.Equal(t, handler2, cfg.ExtraMiddleware[1])
	assert.Equal(t, handler3, cfg.ExtraMiddleware[2])
}

func TestNamePrefix(t *testing.T) {
	t.Parallel()

	opt := NamePrefix("api.v1.")
	cfg := &MountConfig{}

	opt(cfg)

	assert.Equal(t, "api.v1.", cfg.NamePrefix)
}

func TestWithNotFound(t *testing.T) {
	t.Parallel()

	handler := "notFoundHandler"
	opt := WithNotFound(handler)
	cfg := &MountConfig{}

	opt(cfg)

	assert.Equal(t, handler, cfg.NotFoundHandler)
}

func TestBuildMountConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		opts             []MountOption
		expectInherit    bool
		expectNamePrefix string
		expectMiddleware int
	}{
		{
			name:             "no options",
			opts:             nil,
			expectInherit:    false,
			expectNamePrefix: "",
			expectMiddleware: 0,
		},
		{
			name:             "inherit middleware",
			opts:             []MountOption{InheritMiddleware()},
			expectInherit:    true,
			expectNamePrefix: "",
			expectMiddleware: 0,
		},
		{
			name:             "name prefix",
			opts:             []MountOption{NamePrefix("api.")},
			expectInherit:    false,
			expectNamePrefix: "api.",
			expectMiddleware: 0,
		},
		{
			name: "all options",
			opts: []MountOption{
				InheritMiddleware(),
				NamePrefix("admin."),
				WithMiddleware("mw1", "mw2"),
			},
			expectInherit:    true,
			expectNamePrefix: "admin.",
			expectMiddleware: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := BuildMountConfig(tt.opts...)

			require.NotNil(t, cfg)
			assert.Equal(t, tt.expectInherit, cfg.InheritMiddleware)
			assert.Equal(t, tt.expectNamePrefix, cfg.NamePrefix)
			assert.Len(t, cfg.ExtraMiddleware, tt.expectMiddleware)
		})
	}
}

// ExtractConstraintPattern Tests

func TestExtractConstraintPattern(t *testing.T) {
	t.Parallel()

	// Note: ConstraintFromPattern wraps the pattern in ^...$ anchors.
	// So input `\d+` becomes `^\d+$` in the compiled regex.
	// ExtractConstraintPattern should strip those anchors back.

	tests := []struct {
		name     string
		input    string // Pattern passed to ConstraintFromPattern (without anchors)
		expected string // Expected result from ExtractConstraintPattern
	}{
		{
			name:     "simple numeric pattern",
			input:    `\d+`,
			expected: `\d+`,
		},
		{
			name:     "complex pattern",
			input:    `[a-zA-Z][a-zA-Z0-9._-]*`,
			expected: `[a-zA-Z][a-zA-Z0-9._-]*`,
		},
		{
			name:     "empty pattern",
			input:    ``,
			expected: ``,
		},
		{
			name:     "alphanumeric pattern",
			input:    `[a-z0-9]+`,
			expected: `[a-z0-9]+`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			constraint := ConstraintFromPattern("param", tt.input)
			result := ExtractConstraintPattern(constraint)

			assert.Equal(t, tt.expected, result)
		})
	}
}

// Group Tests

// mockRegistrar implements Registrar for testing Group without the full router.
type mockRegistrar struct {
	routes           []*Route
	frozen           bool
	warmedUp         bool
	globalMiddleware []Handler
	namedRoutes      map[string]*Route
}

func newMockRegistrar() *mockRegistrar {
	return &mockRegistrar{
		namedRoutes: make(map[string]*Route),
	}
}

func (m *mockRegistrar) IsFrozen() bool                                     { return m.frozen }
func (m *mockRegistrar) IsWarmedUp() bool                                   { return m.warmedUp }
func (m *mockRegistrar) AddPendingRoute(route *Route)                       { m.routes = append(m.routes, route) }
func (m *mockRegistrar) RegisterRouteNow(route *Route)                      { m.routes = append(m.routes, route) }
func (m *mockRegistrar) GetGlobalMiddleware() []Handler                     { return m.globalMiddleware }
func (m *mockRegistrar) RecordRouteRegistration(_, _ string)                {}
func (m *mockRegistrar) Emit(_ DiagnosticKind, _ string, _ map[string]any)  {}
func (m *mockRegistrar) UpdateRouteInfo(_, _, _ string, _ func(info *Info)) {}
func (m *mockRegistrar) RegisterNamedRoute(name string, route *Route) error {
	if _, exists := m.namedRoutes[name]; exists {
		return &duplicateNameError{name: name}
	}
	m.namedRoutes[name] = route

	return nil
}
func (m *mockRegistrar) GetRouteCompiler() *compiler.RouteCompiler                   { return nil }
func (m *mockRegistrar) UseCompiledRoutes() bool                                     { return false }
func (m *mockRegistrar) AddRouteToTree(_, _ string, _ []Handler, _ []Constraint)     {}
func (m *mockRegistrar) AddVersionRoute(_, _, _ string, _ []Handler, _ []Constraint) {}
func (m *mockRegistrar) StoreRouteInfo(_ Info)                                       {}
func (m *mockRegistrar) AddRouteWithConstraints(method, path string, handlers []Handler) *Route {
	route := NewRoute(m, "", method, path, handlers)
	m.AddPendingRoute(route)

	return route
}
func (m *mockRegistrar) CacheRouteHandlers(_ *compiler.CompiledRoute, _ []Handler) {}

type duplicateNameError struct {
	name string
}

func (e *duplicateNameError) Error() string {
	return "route name already registered: " + e.name
}

func TestNewGroup(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	middleware := []Handler{"mw1", "mw2"}

	g := NewGroup(reg, "/api", middleware)

	require.NotNil(t, g)
	assert.Equal(t, "/api", g.prefix)
	assert.Len(t, g.middleware, 2)
}

func TestGroup_Use(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	g := NewGroup(reg, "/api", nil)

	g.Use("mw1", "mw2")

	assert.Len(t, g.middleware, 2)

	g.Use("mw3")

	assert.Len(t, g.middleware, 3)
}

func TestGroup_SetNamePrefix(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	g := NewGroup(reg, "/api", nil)

	result := g.SetNamePrefix("api.")

	assert.Equal(t, g, result, "should return self for chaining")
	assert.Equal(t, "api.", g.NamePrefix())
}

func TestGroup_SetNamePrefix_Chained(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	g := NewGroup(reg, "/api", nil).SetNamePrefix("api.").SetNamePrefix("v1.")

	assert.Equal(t, "api.v1.", g.NamePrefix())
}

func TestGroup_NestedGroup(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	api := NewGroup(reg, "/api", []Handler{"apiMw"})
	v1 := api.Group("/v1", "v1Mw")

	assert.Equal(t, "/api/v1", v1.prefix)
	assert.Len(t, v1.middleware, 2, "should inherit parent middleware")
}

func TestGroup_NestedGroup_InheritsNamePrefix(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	api := NewGroup(reg, "/api", nil)
	api.SetNamePrefix("api.")

	v1 := api.Group("/v1", nil)

	assert.Equal(t, "api.", v1.NamePrefix(), "nested group should inherit name prefix")
}

func TestGroup_NestedGroup_EmptyPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		parentPrefix string
		childPrefix  string
		expectedFull string
	}{
		{
			name:         "empty parent",
			parentPrefix: "",
			childPrefix:  "/v1",
			expectedFull: "/v1",
		},
		{
			name:         "empty child",
			parentPrefix: "/api",
			childPrefix:  "",
			expectedFull: "/api",
		},
		{
			name:         "both empty",
			parentPrefix: "",
			childPrefix:  "",
			expectedFull: "",
		},
		{
			name:         "both present",
			parentPrefix: "/api",
			childPrefix:  "/v1",
			expectedFull: "/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := newMockRegistrar()
			parent := NewGroup(reg, tt.parentPrefix, nil)
			child := parent.Group(tt.childPrefix)

			assert.Equal(t, tt.expectedFull, child.prefix)
		})
	}
}

func TestGroup_HTTPMethods(t *testing.T) {
	t.Parallel()

	methods := []struct {
		name   string
		call   func(g *Group) *Route
		method string
	}{
		{"GET", func(g *Group) *Route { return g.GET("/test", "handler") }, "GET"},
		{"POST", func(g *Group) *Route { return g.POST("/test", "handler") }, "POST"},
		{"PUT", func(g *Group) *Route { return g.PUT("/test", "handler") }, "PUT"},
		{"DELETE", func(g *Group) *Route { return g.DELETE("/test", "handler") }, "DELETE"},
		{"PATCH", func(g *Group) *Route { return g.PATCH("/test", "handler") }, "PATCH"},
		{"OPTIONS", func(g *Group) *Route { return g.OPTIONS("/test", "handler") }, "OPTIONS"},
		{"HEAD", func(g *Group) *Route { return g.HEAD("/test", "handler") }, "HEAD"},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := newMockRegistrar()
			g := NewGroup(reg, "/api", nil)

			route := tt.call(g)

			require.NotNil(t, route)
			assert.Equal(t, tt.method, route.Method())
			assert.Equal(t, "/api/test", route.Path())
		})
	}
}

func TestGroup_RouteWithMiddleware(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	g := NewGroup(reg, "/api", []Handler{"groupMw"})

	route := g.GET("/test", "handler1", "handler2")

	// The route should have group middleware + route handlers
	handlers := route.Handlers()
	require.Len(t, handlers, 3)
	assert.Equal(t, "groupMw", handlers[0])
	assert.Equal(t, "handler1", handlers[1])
	assert.Equal(t, "handler2", handlers[2])
}

func TestGroup_RouteGroupReference(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	g := NewGroup(reg, "/api", nil)

	route := g.GET("/test", "handler")

	// Route should have group reference set (for name prefixing)
	assert.NotNil(t, route.group)
	assert.Equal(t, g, route.group)
}

// Route Tests

func TestNewRoute(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	handlers := []Handler{"handler1", "handler2"}

	route := NewRoute(reg, "v1", "GET", "/users/:id", handlers)

	require.NotNil(t, route)
	assert.Equal(t, "GET", route.Method())
	assert.Equal(t, "/users/:id", route.Path())
	assert.Equal(t, "v1", route.Version())
	assert.Len(t, route.Handlers(), 2)
}

func TestRoute_SetDescription(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users", nil)

	result := route.SetDescription("Get all users")

	assert.Equal(t, route, result, "should return self for chaining")
	assert.Equal(t, "Get all users", route.Description())
}

func TestRoute_SetTags(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users", nil)

	result := route.SetTags("users", "api")

	assert.Equal(t, route, result, "should return self for chaining")
	assert.Equal(t, []string{"users", "api"}, route.Tags())
}

func TestRoute_SetTags_Append(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users", nil)

	route.SetTags("users").SetTags("api", "public")

	assert.Equal(t, []string{"users", "api", "public"}, route.Tags())
}

func TestRoute_TypedConstraints(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", nil)

	// Initially nil
	assert.Nil(t, route.TypedConstraints())

	// After adding constraint
	route.WhereInt("id")

	constraints := route.TypedConstraints()
	require.NotNil(t, constraints)
	require.Len(t, constraints, 1)
	assert.Equal(t, ConstraintInt, constraints["id"].Kind)
}

func TestRoute_TypedConstraints_Copy(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", nil)
	route.WhereInt("id")

	// Get a copy
	constraints := route.TypedConstraints()
	constraints["id"] = ParamConstraint{Kind: ConstraintUUID}

	// Original should not be modified
	original := route.TypedConstraints()
	assert.Equal(t, ConstraintInt, original["id"].Kind)
}

func TestRoute_SetReversePattern(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", nil)

	pattern := ParseReversePattern("/users/:id")
	route.SetReversePattern(pattern)

	assert.Equal(t, pattern, route.ReversePattern())
}

func TestRoute_Getters(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	handlers := []Handler{"h1", "h2"}
	route := NewRoute(reg, "v2", "POST", "/items/:id", handlers)
	route.SetDescription("Create item")
	route.SetTags("items", "create")

	assert.Equal(t, "POST", route.Method())
	assert.Equal(t, "/items/:id", route.Path())
	assert.Equal(t, "v2", route.Version())
	assert.Equal(t, "Create item", route.Description())
	assert.Equal(t, []string{"items", "create"}, route.Tags())
	assert.Equal(t, handlers, route.Handlers())
	assert.Empty(t, route.Constraints())
	assert.Empty(t, route.Name())
}

// PrepareMountRoute Tests

func TestPrepareMountRoute(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", []Handler{"handler"})
	route.SetDescription("Get user by ID")
	route.SetTags("users", "read")

	data := PrepareMountRoute("/api/v1", route, []Handler{"mw1", "mw2"}, "api.")

	assert.Equal(t, "GET", data.Method)
	assert.Equal(t, "/api/v1/users/:id", data.FullPath)
	assert.Len(t, data.Handlers, 3) // mw1, mw2, handler
	assert.Equal(t, "Get user by ID", data.Description)
	assert.Equal(t, []string{"users", "read"}, data.Tags)
}

func TestPrepareMountRoute_RootPath(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/", []Handler{"handler"})

	data := PrepareMountRoute("/api", route, nil, "")

	assert.Equal(t, "/api", data.FullPath)
}

func TestPrepareMountRoute_WithName(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users", []Handler{"handler"})
	route.name = "users.list" // Set name directly (SetName requires non-frozen registrar)

	data := PrepareMountRoute("/api", route, nil, "admin.")

	assert.Equal(t, "admin.users.list", data.Name)
}

func TestPrepareMountRoute_WithConstraints(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", []Handler{"handler"})
	route.constraints = []Constraint{ConstraintFromPattern("id", `\d+`)}

	data := PrepareMountRoute("/api", route, nil, "")

	require.Len(t, data.Constraints, 1)
	assert.Equal(t, "id", data.Constraints[0].Param)
}

func TestPrepareMountRoute_WithTypedConstraints(t *testing.T) {
	t.Parallel()

	reg := newMockRegistrar()
	route := NewRoute(reg, "", "GET", "/users/:id", []Handler{"handler"})
	route.typedConstraints = map[string]ParamConstraint{
		"id": {Kind: ConstraintInt},
	}

	data := PrepareMountRoute("/api", route, nil, "")

	require.Len(t, data.TypedConstraints, 1)
	assert.Equal(t, ConstraintInt, data.TypedConstraints["id"].Kind)
}

// CompilerHandlers and CompilerConstraints Tests

func TestCompilerHandlers(t *testing.T) {
	t.Parallel()

	handlers := []Handler{"h1", "h2", "h3"}

	result := CompilerHandlers(handlers)

	require.Len(t, result, 3)
	// The result should be compiler.HandlerFunc type
	for i, h := range result {
		assert.NotNil(t, h, "handler %d should not be nil", i)
	}
}

func TestCompilerHandlers_Empty(t *testing.T) {
	t.Parallel()

	result := CompilerHandlers(nil)

	assert.Empty(t, result)
}

func TestCompilerConstraints(t *testing.T) {
	t.Parallel()

	constraints := []Constraint{
		ConstraintFromPattern("id", `\d+`),
		ConstraintFromPattern("slug", `[a-z]+`),
	}

	result := CompilerConstraints(constraints)

	require.Len(t, result, 2)
	assert.Equal(t, "id", result[0].Param)
	assert.Equal(t, "slug", result[1].Param)
}

func TestCompilerConstraints_Empty(t *testing.T) {
	t.Parallel()

	result := CompilerConstraints(nil)

	assert.Nil(t, result)
}

// Info Tests

func TestInfo_Struct(t *testing.T) {
	t.Parallel()

	info := Info{
		Method:      "GET",
		Path:        "/users/:id",
		HandlerName: "GetUser",
		Middleware:  []string{"auth", "log"},
		Constraints: map[string]string{"id": `\d+`},
		IsStatic:    false,
		Version:     "v1",
		ParamCount:  1,
	}

	assert.Equal(t, "GET", info.Method)
	assert.Equal(t, "/users/:id", info.Path)
	assert.Equal(t, "GetUser", info.HandlerName)
	assert.Len(t, info.Middleware, 2)
	assert.Equal(t, `\d+`, info.Constraints["id"])
	assert.False(t, info.IsStatic)
	assert.Equal(t, "v1", info.Version)
	assert.Equal(t, 1, info.ParamCount)
}
