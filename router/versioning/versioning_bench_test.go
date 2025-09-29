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

package versioning

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkEngine_DetectVersion_Header(b *testing.B) {
	engine, err := New(
		WithHeaderVersioning("API-Version"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("API-Version", "v2")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_Query(b *testing.B) {
	engine, err := New(
		WithQueryVersioning("v"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test?v=v2", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_Path(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/v2/users", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_Accept(b *testing.B) {
	engine, err := New(
		WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/vnd.myapi.v2+json")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_MultipleStrategies(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("API-Version"),
		WithQueryVersioning("v"),
		WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/v2/users?v=v3", nil)
	req.Header.Set("API-Version", "v4")
	req.Header.Set("Accept", "application/vnd.myapi.v5+json")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_WithValidation(b *testing.B) {
	engine, err := New(
		WithQueryVersioning("v"),
		WithValidVersions("v1", "v2", "v3"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test?v=v2", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_StripPathVersion(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
	)
	if err != nil {
		b.Fatal(err)
	}

	path := "/v1/users/123/posts"
	version := "v1"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.StripPathVersion(path, version)
	}
}

func BenchmarkEngine_ShouldApplyVersioning(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	path := "/v1/users"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.ShouldApplyVersioning(path)
	}
}

func BenchmarkEngine_ExtractPathSegment(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
	)
	if err != nil {
		b.Fatal(err)
	}

	path := "/v1/users"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = engine.ExtractPathSegment(path)
	}
}

func BenchmarkExtractQueryVersion(b *testing.B) {
	rawQuery := "foo=bar&v=v2&baz=qux"
	param := "v"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractQueryVersion(rawQuery, param)
	}
}

func BenchmarkExtractHeaderVersion(b *testing.B) {
	headers := http.Header{
		"Api-Version": []string{"v2"},
	}
	headerName := "Api-Version"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = extractHeaderVersion(headers, headerName)
	}
}

func BenchmarkExtractPathVersion(b *testing.B) {
	path := "/v2/users"
	prefix := "/v"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractPathVersion(path, prefix)
	}
}

func BenchmarkExtractAcceptVersion(b *testing.B) {
	accept := "application/vnd.myapi.v2+json"
	pattern := "application/vnd.myapi.v{version}+json"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractAcceptVersion(accept, pattern)
	}
}

// Worst-case benchmarks

func BenchmarkEngine_DetectVersion_WorstCase(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithHeaderVersioning("API-Version"),
		WithQueryVersioning("v"),
		WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
		WithValidVersions("v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	// Request with no version information - must check all strategies
	req := httptest.NewRequest("GET", "/users", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_DetectVersion_InvalidVersion(b *testing.B) {
	engine, err := New(
		WithQueryVersioning("v"),
		WithValidVersions("v1", "v2", "v3"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	// Request with invalid version - must validate and fallback
	req := httptest.NewRequest("GET", "/test?v=v99", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = engine.DetectVersion(req)
	}
}

func BenchmarkEngine_SetLifecycleHeaders_WithDeprecation(b *testing.B) {
	sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithDeprecatedVersion("v1", sunsetDate),
		WithDeprecationLink("v1", "https://docs.example.com/migration"),
		WithVersionHeader(),
		WithWarning299(),
	)
	if err != nil {
		b.Fatal(err)
	}

	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Header().Del("Deprecation")
		w.Header().Del("Sunset")
		w.Header().Del("Link")
		w.Header().Del("Warning")
		w.Header().Del("X-API-Version")
		_ = engine.SetLifecycleHeaders(w, "v1", "/api/users")
	}
}

func BenchmarkQueryVersion_LongQuery(b *testing.B) {
	// Long query string with version at the end
	longQuery := "param1=value1&param2=value2&param3=value3&param4=value4&param5=value5&param6=value6&param7=value7&param8=value8&param9=value9&param10=value10&v=v2"
	param := "v"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractQueryVersion(longQuery, param)
	}
}

func BenchmarkAcceptVersion_MultipleValues(b *testing.B) {
	// Accept header with multiple values
	accept := "application/vnd.myapi.v2+json, text/html, application/xml, application/json, image/png"
	pattern := "application/vnd.myapi.v{version}+json"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractAcceptVersion(accept, pattern)
	}
}

func BenchmarkPathVersion_LongPath(b *testing.B) {
	// Long path with version at start
	longPath := "/v1/users/123/posts/456/comments/789/replies/101112/likes/131415"
	prefix := "/v"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractPathVersion(longPath, prefix)
	}
}

// Parallel benchmarks

func BenchmarkEngine_DetectVersion_Header_Parallel(b *testing.B) {
	engine, err := New(
		WithHeaderVersioning("API-Version"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("API-Version", "v2")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = engine.DetectVersion(req)
		}
	})
}

func BenchmarkEngine_DetectVersion_Path_Parallel(b *testing.B) {
	engine, err := New(
		WithPathVersioning("/v{version}/"),
		WithDefaultVersion("v1"),
	)
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/v2/users", nil)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = engine.DetectVersion(req)
		}
	})
}
