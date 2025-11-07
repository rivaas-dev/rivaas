package router

import (
	"testing"
)

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

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user, WithStrategy(ValidationTags))
	}
}

// BenchmarkValidate_Interface benchmarks interface-based validation
func BenchmarkValidate_Interface(b *testing.B) {
	user := &userWithValidator{
		Name:  "John",
		Email: "john@example.com",
	}

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user, WithStrategy(ValidationInterface))
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

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user, WithStrategy(ValidationJSONSchema), WithCustomSchema("bench-user", schema))
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

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(order, WithStrategy(ValidationTags))
	}
}

// BenchmarkValidate_Auto benchmarks auto strategy selection
func BenchmarkValidate_Auto(b *testing.B) {
	user := &userWithValidator{
		Name:  "John",
		Email: "john@example.com",
	}

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user) // Auto strategy
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

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user, WithStrategy(ValidationTags), WithMaxErrors(5))
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Validate(user, WithStrategy(ValidationTags))
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

	b.ResetTimer()
	for b.Loop() {
		_ = Validate(user, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	}
}
