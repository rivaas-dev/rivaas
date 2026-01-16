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

//go:build !integration

package binding

import (
	"net/url"
	"reflect"
	"testing"
)

// BenchmarkBind benchmarks the core Bind function with various struct sizes.
func BenchmarkBind(b *testing.B) {
	type SmallStruct struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	type LargeStruct struct {
		ID       int    `query:"id"`
		Name     string `query:"name"`
		Email    string `query:"email"`
		Age      int    `query:"age"`
		Phone    string `query:"phone"`
		Address  string `query:"address"`
		City     string `query:"city"`
		State    string `query:"state"`
		Zip      string `query:"zip"`
		Country  string `query:"country"`
		Website  string `query:"website"`
		Bio      string `query:"bio"`
		Tags     string `query:"tags"`
		Status   string `query:"status"`
		Verified bool   `query:"verified"`
	}

	smallValues := url.Values{}
	smallValues.Set("name", "Alice")
	smallValues.Set("age", "30")
	smallValues.Set("email", "alice@example.com")

	largeValues := url.Values{}
	for range 15 {
		largeValues.Set("field", "value")
	}

	b.Run("SmallStruct", func(b *testing.B) {
		var params SmallStruct
		getter := NewQueryGetter(smallValues)
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			Raw(getter, TagQuery, &params)
		}
	})

	b.Run("LargeStruct", func(b *testing.B) {
		var params LargeStruct
		getter := NewQueryGetter(largeValues)
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			Raw(getter, TagQuery, &params)
		}
	})
}

// BenchmarkBind_Parallel benchmarks Bind with parallel execution.
func BenchmarkBind_Parallel(b *testing.B) {
	type Params struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("age", "30")
	values.Set("email", "alice@example.com")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var params Params
		getter := NewQueryGetter(values)
		for pb.Next() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			Raw(getter, TagQuery, &params)
		}
	})
}

// BenchmarkBindInto benchmarks the generic BindInto helper.
func BenchmarkBindInto(b *testing.B) {
	type Params struct {
		ID   int    `path:"id"`
		Name string `path:"name"`
	}

	paramsMap := map[string]string{
		"id":   "123",
		"name": "Bob",
	}

	getter := NewPathGetter(paramsMap)
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		RawInto[Params](getter, TagPath)
	}
}

// BenchmarkBindTo benchmarks binding from multiple sources.
func BenchmarkBindTo(b *testing.B) {
	type Request struct {
		UserID    int    `path:"user_id"`
		Page      int    `query:"page"`
		UserAgent string `header:"User-Agent"`
	}

	params := map[string]string{"user_id": "456"}
	query := url.Values{}
	query.Set("page", "2")
	headers := map[string][]string{
		"User-Agent": {"MyApp/1.0"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var req Request
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		BindTo(&req,
			FromPath(params),
			FromQuery(query),
			FromHeader(headers),
		)
	}
}

// BenchmarkHasStructTag benchmarks tag checking.
func BenchmarkHasStructTag(b *testing.B) {
	type Request struct {
		ID   int    `path:"id"`
		Name string `query:"name"`
		Auth string `header:"Authorization"`
	}

	typ := reflect.TypeFor[Request]()
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = HasStructTag(typ, TagPath)
		_ = HasStructTag(typ, TagQuery)
		_ = HasStructTag(typ, TagHeader)
	}
}

// BenchmarkBind_Allocations benchmarks memory allocations.
func BenchmarkBind_Allocations(b *testing.B) {
	type Params struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("age", "30")
	values.Set("email", "alice@example.com")

	getter := NewQueryGetter(values)
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var params Params
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Raw(getter, TagQuery, &params)
	}
}

// BenchmarkBind_NestedStruct benchmarks binding nested structs.
func BenchmarkBind_NestedStruct(b *testing.B) {
	type Address struct {
		Street string `query:"street"`
		City   string `query:"city"`
		Zip    string `query:"zip"`
	}

	type User struct {
		Name    string  `query:"name"`
		Age     int     `query:"age"`
		Address Address `query:""`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("age", "30")
	values.Set("street", "123 Main St")
	values.Set("city", "Springfield")
	values.Set("zip", "12345")

	getter := NewQueryGetter(values)
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var user User
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Raw(getter, TagQuery, &user)
	}
}

// BenchmarkBind_Slices benchmarks binding slice fields.
func BenchmarkBind_Slices(b *testing.B) {
	type Params struct {
		Tags []string `query:"tags"`
		IDs  []int    `query:"ids"`
	}

	values := url.Values{}
	values.Add("tags", "go")
	values.Add("tags", "testing")
	values.Add("tags", "benchmarks")
	values.Add("ids", "1")
	values.Add("ids", "2")
	values.Add("ids", "3")

	getter := NewQueryGetter(values)
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var params Params
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Raw(getter, TagQuery, &params)
	}
}

// BenchmarkBind_WithDefaults benchmarks binding with default values.
func BenchmarkBind_WithDefaults(b *testing.B) {
	type Config struct {
		Port     int    `query:"port" default:"8080"`
		Host     string `query:"host" default:"localhost"`
		Debug    bool   `query:"debug" default:"false"`
		LogLevel string `query:"log_level" default:"info"`
	}

	// Empty query string - defaults will be applied
	values := url.Values{}
	getter := NewQueryGetter(values)
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var config Config
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Raw(getter, TagQuery, &config)
	}
}
