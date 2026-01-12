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

package validation

import (
	"testing"
)

// benchUser implements Validator for benchmarks
type benchUser struct {
	Name  string
	Email string
}

func (u *benchUser) Validate() error {
	if u.Name == "" || u.Email == "" {
		return ErrValidationFailed
	}

	return nil
}

// BenchmarkValidate_Tags benchmarks tag-based validation
func BenchmarkValidate_Tags(b *testing.B) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18,max=120"`
	}

	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   25,
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, user, WithStrategy(StrategyTags))
	}
}

// BenchmarkValidate_Interface benchmarks interface-based validation
func BenchmarkValidate_Interface(b *testing.B) {
	user := &benchUser{
		Name:  "John",
		Email: "john@example.com",
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, user, WithStrategy(StrategyInterface))
	}
}

// BenchmarkValidate_JSONSchema benchmarks JSON Schema validation
func BenchmarkValidate_JSONSchema(b *testing.B) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		},
		"required": ["name", "email"]
	}`

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	user := &User{
		Name:  "John",
		Email: "john@example.com",
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, user, WithStrategy(StrategyJSONSchema), WithCustomSchema("bench-user", schema))
	}
}

// BenchmarkValidate_Complex benchmarks validation of a complex nested structure
func BenchmarkValidate_Complex(b *testing.B) {
	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
		Zip    string `json:"zip" validate:"required"`
	}

	type Item struct {
		Name  string  `json:"name" validate:"required"`
		Price float64 `json:"price" validate:"required,min=0"`
	}

	type Order struct {
		ID      string  `json:"id" validate:"required"`
		Address Address `json:"address" validate:"required"`
		Items   []Item  `json:"items" validate:"required,min=1"`
		Total   float64 `json:"total" validate:"required,min=0"`
	}

	order := &Order{
		ID: "order-123",
		Address: Address{
			Street: "123 Main St",
			City:   "NYC",
			Zip:    "10001",
		},
		Items: []Item{
			{Name: "item1", Price: 10.0},
			{Name: "item2", Price: 20.0},
		},
		Total: 30.0,
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, order, WithStrategy(StrategyTags))
	}
}

// BenchmarkValidate_Auto benchmarks auto strategy selection
func BenchmarkValidate_Auto(b *testing.B) {
	user := &benchUser{
		Name:  "John",
		Email: "john@example.com",
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, user) // Auto strategy
	}
}

// BenchmarkValidate_WithMaxErrors benchmarks validation with error truncation
func BenchmarkValidate_WithMaxErrors(b *testing.B) {
	type User struct {
		Field1  string `json:"field1" validate:"required"`
		Field2  string `json:"field2" validate:"required"`
		Field3  string `json:"field3" validate:"required"`
		Field4  string `json:"field4" validate:"required"`
		Field5  string `json:"field5" validate:"required"`
		Field6  string `json:"field6" validate:"required"`
		Field7  string `json:"field7" validate:"required"`
		Field8  string `json:"field8" validate:"required"`
		Field9  string `json:"field9" validate:"required"`
		Field10 string `json:"field10" validate:"required"`
	}

	user := &User{} // All fields missing

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Validate(ctx, user, WithStrategy(StrategyTags), WithMaxErrors(5))
	}
}

// BenchmarkValidate_Concurrent benchmarks concurrent validation
func BenchmarkValidate_Concurrent(b *testing.B) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	user := &User{
		Name:  "John",
		Email: "john@example.com",
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			Validate(ctx, user, WithStrategy(StrategyTags))
		}
	})
}

// BenchmarkValidate_Partial benchmarks partial validation
func BenchmarkValidate_Partial(b *testing.B) {
	type User struct {
		Name    string `json:"name" validate:"required"`
		Email   string `json:"email" validate:"required,email"`
		Address string `json:"address" validate:"required"`
	}

	user := &User{Name: "John"}
	pm := PresenceMap{
		"name": true,
	}

	ctx := b.Context()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		ValidatePartial(ctx, user, pm, WithStrategy(StrategyTags))
	}
}
