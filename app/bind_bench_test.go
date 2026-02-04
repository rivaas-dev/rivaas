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

package app

import (
	"testing"
)

type benchRequest struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func BenchmarkBind(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	c, err := TestContextWithBody("POST", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		var req benchRequest
		if bindErr := c.Bind(&req); bindErr != nil {
			b.Fatal(bindErr)
		}
	}
}

func BenchmarkBind_Generic(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	c, err := TestContextWithBody("POST", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		_, bindErr := Bind[benchRequest](c)
		if bindErr != nil {
			b.Fatal(bindErr)
		}
	}
}

func BenchmarkBindOnly(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	c, err := TestContextWithBody("POST", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		var req benchRequest
		if bindErr := c.BindOnly(&req); bindErr != nil {
			b.Fatal(bindErr)
		}
	}
}

func BenchmarkMustBind(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	c, err := TestContextWithBody("POST", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		var req benchRequest
		if !c.MustBind(&req) {
			b.Fatal("MustBind failed")
		}
	}
}

func BenchmarkBind_WithStrict(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	c, err := TestContextWithBody("POST", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		var req benchRequest
		if bindErr := c.Bind(&req, WithStrict()); bindErr != nil {
			b.Fatal(bindErr)
		}
	}
}

func BenchmarkBind_WithPartial(b *testing.B) {
	body := map[string]any{
		"name": "Alice",
	}

	c, err := TestContextWithBody("PATCH", "/test", body)
	if err != nil {
		b.Fatal(err)
	}

	type updateRequest struct {
		Name *string `json:"name" validate:"omitempty,min=2"`
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c.ResetBinding()
		var req updateRequest
		if bindErr := c.Bind(&req, WithPartial()); bindErr != nil {
			b.Fatal(bindErr)
		}
	}
}

func BenchmarkBind_Parallel(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c, err := TestContextWithBody("POST", "/test", body)
			if err != nil {
				b.Fatal(err)
			}

			var req benchRequest
			if bindErr := c.Bind(&req); bindErr != nil {
				b.Fatal(bindErr)
			}
		}
	})
}

func BenchmarkBindOnly_Parallel(b *testing.B) {
	body := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c, err := TestContextWithBody("POST", "/test", body)
			if err != nil {
				b.Fatal(err)
			}

			var req benchRequest
			if bindErr := c.BindOnly(&req); bindErr != nil {
				b.Fatal(bindErr)
			}
		}
	})
}
