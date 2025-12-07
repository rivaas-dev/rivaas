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

package proto_test

import (
	"bytes"
	"fmt"
	"log"

	goproto "google.golang.org/protobuf/proto"

	"rivaas.dev/binding/proto"
	"rivaas.dev/binding/proto/testdata"
)

// ExampleProto demonstrates basic Protocol Buffers binding.
func ExampleProto() {
	// Create a test user message
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}

	// Marshal to proto bytes (simulating incoming request)
	body, err := goproto.Marshal(user)
	if err != nil {
		log.Fatal(err)
	}

	// Bind using the proto package
	result, err := proto.Proto[*testdata.User](body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s, Email: %s, Age: %d, Active: %v\n",
		result.GetName(), result.GetEmail(), result.GetAge(), result.GetActive())
	// Output: Name: John, Email: john@example.com, Age: 30, Active: true
}

// ExampleProtoTo demonstrates non-generic Protocol Buffers binding.
func ExampleProtoTo() {
	// Create a test config message
	config := &testdata.Config{
		Title: "My App",
		Server: &testdata.Server{
			Host: "localhost",
			Port: 8080,
		},
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	// Bind using the non-generic function
	var result testdata.Config
	err = proto.ProtoTo(body, &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Title: %s, Server: %s:%d\n",
		result.GetTitle(), result.GetServer().GetHost(), result.GetServer().GetPort())
	// Output: Title: My App, Server: localhost:8080
}

// ExampleProtoReader demonstrates Protocol Buffers binding from io.Reader.
func ExampleProtoReader() {
	// Create a test user message
	user := &testdata.User{
		Name:   "Alice",
		Email:  "alice@example.com",
		Age:    25,
		Active: true,
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(user)
	if err != nil {
		log.Fatal(err)
	}

	// Bind from io.Reader
	result, err := proto.ProtoReader[*testdata.User](bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s, Email: %s\n", result.Name, result.Email)
	// Output: Name: Alice, Email: alice@example.com
}

// ExampleProto_withDiscardUnknown demonstrates binding with unknown field handling.
func ExampleProto_withDiscardUnknown() {
	// Create a test user message
	user := &testdata.User{
		Name:   "Bob",
		Email:  "bob@example.com",
		Age:    35,
		Active: false,
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(user)
	if err != nil {
		log.Fatal(err)
	}

	// Bind with DiscardUnknown option
	result, err := proto.Proto[*testdata.User](body, proto.WithDiscardUnknown())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s, Active: %v\n", result.Name, result.Active)
	// Output: Name: Bob, Active: false
}

// ExampleProto_nestedMessages demonstrates binding nested Protocol Buffers messages.
func ExampleProto_nestedMessages() {
	// Create a nested config message
	config := &testdata.Config{
		Title: "Production Config",
		Server: &testdata.Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &testdata.Database{
			Host:     "db.example.com",
			Port:     5432,
			Name:     "mydb",
			User:     "admin",
			Password: "secret",
		},
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	// Bind the nested structure
	result, err := proto.Proto[*testdata.Config](body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Title: %s\n", result.Title)
	fmt.Printf("Server: %s:%d\n", result.Server.Host, result.Server.Port)
	fmt.Printf("Database: %s@%s:%d\n", result.Database.Name, result.Database.Host, result.Database.Port)
	// Output:
	// Title: Production Config
	// Server: 0.0.0.0:8080
	// Database: mydb@db.example.com:5432
}

// ExampleProto_repeatedFields demonstrates binding repeated Protocol Buffers fields.
func ExampleProto_repeatedFields() {
	// Create a product with repeated fields
	product := &testdata.Product{
		Name:   "Widget",
		Tags:   []string{"electronics", "gadget", "sale"},
		Prices: []int32{100, 150, 200},
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(product)
	if err != nil {
		log.Fatal(err)
	}

	// Bind the repeated fields
	result, err := proto.Proto[*testdata.Product](body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Product: %s\n", result.Name)
	fmt.Printf("Tags: %v\n", result.Tags)
	fmt.Printf("Prices: %v\n", result.Prices)
	// Output:
	// Product: Widget
	// Tags: [electronics gadget sale]
	// Prices: [100 150 200]
}

// ExampleProto_mapFields demonstrates binding Protocol Buffers map fields.
func ExampleProto_mapFields() {
	// Create settings with a map field
	settings := &testdata.Settings{
		Name: "AppSettings",
		Metadata: map[string]string{
			"version":     "1.0.0",
			"environment": "production",
		},
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(settings)
	if err != nil {
		log.Fatal(err)
	}

	// Bind the map fields
	result, err := proto.Proto[*testdata.Settings](body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Settings: %s\n", result.Name)
	fmt.Printf("Version: %s\n", result.Metadata["version"])
	fmt.Printf("Environment: %s\n", result.Metadata["environment"])
	// Output:
	// Settings: AppSettings
	// Version: 1.0.0
	// Environment: production
}

// ExampleProto_withValidator demonstrates Protocol Buffers binding with validation.
func ExampleProto_withValidator() {
	// Create a test user message
	user := &testdata.User{
		Name:   "Charlie",
		Email:  "charlie@example.com",
		Age:    28,
		Active: true,
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(user)
	if err != nil {
		log.Fatal(err)
	}

	// Create a simple validator
	validator := &simpleValidator{}

	// Bind with validation
	result, err := proto.Proto[*testdata.User](body, proto.WithValidator(validator))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s, Validated: true\n", result.Name)
	// Output: Name: Charlie, Validated: true
}

// simpleValidator is a test validator for examples.
type simpleValidator struct{}

func (v *simpleValidator) Validate(data any) error {
	return nil
}

// ExampleProto_multipleOptions demonstrates using multiple options together.
func ExampleProto_multipleOptions() {
	// Create a test user message
	user := &testdata.User{
		Name:   "Diana",
		Email:  "diana@example.com",
		Age:    32,
		Active: true,
	}

	// Marshal to proto bytes
	body, err := goproto.Marshal(user)
	if err != nil {
		log.Fatal(err)
	}

	// Bind with multiple options
	result, err := proto.Proto[*testdata.User](body,
		proto.WithAllowPartial(),
		proto.WithDiscardUnknown(),
		proto.WithRecursionLimit(5000),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s, Age: %d\n", result.Name, result.Age)
	// Output: Name: Diana, Age: 32
}
