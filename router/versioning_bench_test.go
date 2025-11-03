package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// BenchmarkFastQueryVersion benchmarks the fast query version function
func BenchmarkFastQueryVersion(b *testing.B) {
	rawQuery := "foo=bar&v=v1&baz=qux"
	param := "v"

	b.ResetTimer()
	for b.Loop() {
		_, _ = fastQueryVersion(rawQuery, param)
	}
}

// BenchmarkStdlibQueryVersion benchmarks the stdlib query version function
func BenchmarkStdlibQueryVersion(b *testing.B) {
	rawQuery := "foo=bar&v=v1&baz=qux"

	b.ResetTimer()
	for b.Loop() {
		values, _ := url.ParseQuery(rawQuery)
		_ = values.Get("v")
	}
}

// BenchmarkFastPathVersion benchmarks the fast path version function
func BenchmarkFastPathVersion(b *testing.B) {
	path := "/v2/users/123"
	prefix := "/v"

	b.ResetTimer()
	for b.Loop() {
		_, _ = fastPathVersion(path, prefix)
	}
}

// BenchmarkFastHeaderVersion benchmarks the fast header version function
func BenchmarkFastHeaderVersion(b *testing.B) {
	headers := http.Header{"API-Version": []string{"v2"}}
	headerName := "API-Version"

	b.ResetTimer()
	for b.Loop() {
		_ = fastHeaderVersion(headers, headerName)
	}
}

// BenchmarkStdlibHeaderVersion benchmarks the stdlib header version function
func BenchmarkStdlibHeaderVersion(b *testing.B) {
	headers := http.Header{"API-Version": []string{"v2"}}

	b.ResetTimer()
	for b.Loop() {
		_ = headers.Get("API-Version")
	}
}

// BenchmarkFastAcceptVersion benchmarks the fast accept version function
func BenchmarkFastAcceptVersion(b *testing.B) {
	accept := "application/vnd.myapi.v2+json, text/html"
	pattern := "application/vnd.myapi.{version}+json"

	b.ResetTimer()
	for b.Loop() {
		_, _ = fastAcceptVersion(accept, pattern)
	}
}

// BenchmarkVersionDetection benchmarks the version detection function with validation
func BenchmarkVersionDetection(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithValidVersions("v1", "v2", "v3"),
		WithDefaultVersion("v1"),
	))

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")

	b.ResetTimer()
	for b.Loop() {
		_ = r.detectVersion(req)
	}
}

// BenchmarkVersionDetectionNoValidation benchmarks the version detection function without validation
func BenchmarkVersionDetectionNoValidation(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v1"),
	))

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")

	b.ResetTimer()
	for b.Loop() {
		_ = r.detectVersion(req)
	}
}

// BenchmarkVersionDetectionWithObserver benchmarks the version detection function with observer
func BenchmarkVersionDetectionWithObserver(b *testing.B) {
	detectedCount := 0
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v1"),
		WithVersionObserver(
			func(version string, method string) { detectedCount++ },
			func() {},
			func(attempted string) {},
		),
	))

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")

	b.ResetTimer()
	for b.Loop() {
		_ = r.detectVersion(req)
	}
}

// BenchmarkVersionDetectionPathPriority benchmarks the version detection function with path priority
func BenchmarkVersionDetectionPathPriority(b *testing.B) {
	r := New(WithVersioning(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("X-API-Version"),
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	))

	// Path should take priority
	req := httptest.NewRequest("GET", "/v2/users?v=v1", nil)
	req.Header.Set("X-API-Version", "v3")

	b.ResetTimer()
	for b.Loop() {
		version := r.detectVersion(req)
		if version != "v2" {
			b.Fatalf("expected v2, got %s", version)
		}
	}
}

// BenchmarkVersionDetectionHeaderPriority benchmarks the version detection function with header priority
func BenchmarkVersionDetectionHeaderPriority(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	))

	// Header should take priority over query
	req := httptest.NewRequest("GET", "/users?v=v1", nil)
	req.Header.Set("X-API-Version", "v2")

	b.ResetTimer()
	for b.Loop() {
		version := r.detectVersion(req)
		if version != "v2" {
			b.Fatalf("expected v2, got %s", version)
		}
	}
}

// BenchmarkAcceptVersioning benchmarks the accept versioning function
func BenchmarkAcceptVersioning(b *testing.B) {
	r := New(WithVersioning(
		WithAcceptVersioning("application/vnd.myapi.{version}+json"),
		WithDefaultVersion("v1"),
	))

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Accept", "application/vnd.myapi.v2+json, text/html")

	b.ResetTimer()
	for b.Loop() {
		version := r.detectVersion(req)
		if version != "v2" {
			b.Fatalf("expected v2, got %s", version)
		}
	}
}

// BenchmarkAcceptVersioningMultipleValues benchmarks the accept versioning function with multiple values
func BenchmarkAcceptVersioningMultipleValues(b *testing.B) {
	r := New(WithVersioning(
		WithAcceptVersioning("application/vnd.myapi.{version}+json"),
		WithDefaultVersion("v1"),
	))

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Accept", "text/html, application/json, application/vnd.myapi.v3+json, */*")

	b.ResetTimer()
	for b.Loop() {
		version := r.detectVersion(req)
		if version != "v3" {
			b.Fatalf("expected v3, got %s", version)
		}
	}
}

// BenchmarkVersionedRequest benchmarks the versioned request handling function
func BenchmarkVersionedRequest(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v1"),
	))

	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.String(200, "v1 users")
	})

	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.String(200, "v2 users")
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkVersionedRequestWithDeprecation benchmarks the versioned request handling function with deprecation
func BenchmarkVersionedRequestWithDeprecation(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v1"),
		WithDeprecatedVersion("v1", time.Now().Add(30*24*time.Hour)),
	))

	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.String(200, "v1 users")
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v1")

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkPathVersioning benchmarks the path versioning function
func BenchmarkPathVersioning(b *testing.B) {
	r := New(WithVersioning(
		WithPathVersioning("/v{version}/"),
		WithDefaultVersion("v1"),
	))

	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.String(200, "v1 users")
	})

	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.String(200, "v2 users")
	})

	req := httptest.NewRequest("GET", "/v2/users", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkPathVersioningWithApiPrefix benchmarks the path versioning function with API prefix
func BenchmarkPathVersioningWithApiPrefix(b *testing.B) {
	r := New(WithVersioning(
		WithPathVersioning("/api/v{version}/"),
		WithDefaultVersion("v1"),
	))

	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.String(200, "v2 users")
	})

	req := httptest.NewRequest("GET", "/api/v2/users", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkValidateVersionNoValidation benchmarks the validate version function without validation
func BenchmarkValidateVersionNoValidation(b *testing.B) {
	cfg := &VersioningConfig{}

	b.ResetTimer()
	for b.Loop() {
		_ = cfg.validateVersion("v2")
	}
}

// BenchmarkValidateVersionWith3Valid benchmarks the validate version function with 3 valid versions
func BenchmarkValidateVersionWith3Valid(b *testing.B) {
	cfg := &VersioningConfig{
		ValidVersions: []string{"v1", "v2", "v3"},
	}

	b.ResetTimer()
	for b.Loop() {
		_ = cfg.validateVersion("v2")
	}
}

// BenchmarkValidateVersionWith10Valid benchmarks the validate version function with 10 valid versions
func BenchmarkValidateVersionWith10Valid(b *testing.B) {
	cfg := &VersioningConfig{
		ValidVersions: []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10"},
	}

	b.ResetTimer()
	for b.Loop() {
		_ = cfg.validateVersion("v5")
	}
}

// BenchmarkSetDeprecationHeadersNoDeprecation benchmarks the set deprecation headers function without deprecation
func BenchmarkSetDeprecationHeadersNoDeprecation(b *testing.B) {
	cfg := &VersioningConfig{}
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		cfg.setDeprecationHeaders(w, "v2")
	}
}

// BenchmarkSetDeprecationHeadersWithDeprecation benchmarks the set deprecation headers function with deprecation
func BenchmarkSetDeprecationHeadersWithDeprecation(b *testing.B) {
	cfg := &VersioningConfig{
		DeprecatedVersions: map[string]time.Time{
			"v1": time.Now().Add(30 * 24 * time.Hour),
		},
	}
	w := httptest.NewRecorder()

	b.ResetTimer()
	for b.Loop() {
		cfg.setDeprecationHeaders(w, "v1")
	}
}

// BenchmarkNonVersionedRouting benchmarks the non-versioned routing function
func BenchmarkNonVersionedRouting(b *testing.B) {
	r := New()
	r.GET("/users", func(c *Context) {
		c.String(200, "users")
	})

	req := httptest.NewRequest("GET", "/users", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkVersionedRoutingOverhead benchmarks the versioned routing overhead
func BenchmarkVersionedRoutingOverhead(b *testing.B) {
	r := New(WithVersioning(
		WithHeaderVersioning("X-API-Version"),
		WithDefaultVersion("v1"),
	))

	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.String(200, "users")
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v1")

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkComplexVersioningScenario benchmarks the complex versioning scenario
func BenchmarkComplexVersioningScenario(b *testing.B) {
	r := New(WithVersioning(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("X-API-Version"),
		WithAcceptVersioning("application/vnd.myapi.{version}+json"),
		WithQueryVersioning("v"),
		WithValidVersions("v1", "v2", "v3"),
		WithDefaultVersion("v1"),
		WithDeprecatedVersion("v1", time.Now().Add(30*24*time.Hour)),
		WithVersionObserver(
			func(version string, method string) {},
			func() {},
			func(attempted string) {},
		),
	))

	for _, ver := range []string{"v1", "v2", "v3"} {
		version := ver
		vr := r.Version(version)
		vr.GET("/users", func(c *Context) {
			c.String(200, "%s users", version)
		})
	}

	req := httptest.NewRequest("GET", "/v2/users", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
