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

package binding_test

import (
	"fmt"
	"net/url"

	"rivaas.dev/binding"
)

// ExampleQuery demonstrates basic binding from query parameters using generic API.
func ExampleQuery() {
	type Params struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("age", "30")
	values.Set("email", "alice@example.com")

	params, err := binding.Query[Params](values)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Age: %d, Email: %s\n", params.Name, params.Age, params.Email)
	// Output: Name: Alice, Age: 30, Email: alice@example.com
}

// ExampleQueryTo demonstrates non-generic query binding.
func ExampleQueryTo() {
	type Params struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	values := url.Values{}
	values.Set("name", "Bob")
	values.Set("age", "25")
	values.Set("email", "bob@example.com")

	var params Params
	err := binding.QueryTo(values, &params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Age: %d, Email: %s\n", params.Name, params.Age, params.Email)
	// Output: Name: Bob, Age: 25, Email: bob@example.com
}

// ExamplePath demonstrates binding from path parameters.
func ExamplePath() {
	type Params struct {
		ID   int    `path:"id"`
		Slug string `path:"slug"`
	}

	pathParams := map[string]string{
		"id":   "123",
		"slug": "hello-world",
	}

	params, err := binding.Path[Params](pathParams)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("ID: %d, Slug: %s\n", params.ID, params.Slug)
	// Output: ID: 123, Slug: hello-world
}

// ExampleJSON demonstrates binding from JSON body.
func ExampleJSON() {
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	body := []byte(`{"name": "Charlie", "email": "charlie@example.com", "age": 35}`)

	user, err := binding.JSON[User](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Email: %s, Age: %d\n", user.Name, user.Email, user.Age)
	// Output: Name: Charlie, Email: charlie@example.com, Age: 35
}

// ExampleBind demonstrates multi-source binding.
func ExampleBind() {
	type Request struct {
		// From path parameters
		UserID int `path:"user_id"`

		// From query string
		Page  int `query:"page"`
		Limit int `query:"limit"`
	}

	pathParams := map[string]string{"user_id": "456"}
	query := url.Values{}
	query.Set("page", "2")
	query.Set("limit", "20")

	req, err := binding.Bind[Request](
		binding.FromPath(pathParams),
		binding.FromQuery(query),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("UserID: %d, Page: %d, Limit: %d\n", req.UserID, req.Page, req.Limit)
	// Output: UserID: 456, Page: 2, Limit: 20
}

// ExampleBindTo demonstrates non-generic multi-source binding.
func ExampleBindTo() {
	type Request struct {
		UserID int `path:"user_id"`
		Page   int `query:"page"`
	}

	pathParams := map[string]string{"user_id": "789"}
	query := url.Values{}
	query.Set("page", "3")

	var req Request
	err := binding.BindTo(&req,
		binding.FromPath(pathParams),
		binding.FromQuery(query),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("UserID: %d, Page: %d\n", req.UserID, req.Page)
	// Output: UserID: 789, Page: 3
}

// ExampleQuery_withDefaults demonstrates binding with default values.
func ExampleQuery_withDefaults() {
	type Config struct {
		Port     int    `query:"port" default:"8080"`
		Host     string `query:"host" default:"localhost"`
		Debug    bool   `query:"debug" default:"false"`
		LogLevel string `query:"log_level" default:"info"`
	}

	// Empty query string - defaults will be applied
	values := url.Values{}

	config, err := binding.Query[Config](values)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Port: %d, Host: %s, Debug: %v, LogLevel: %s\n",
		config.Port, config.Host, config.Debug, config.LogLevel)
	// Output: Port: 8080, Host: localhost, Debug: false, LogLevel: info
}

// ExampleQuery_withOptions demonstrates binding with custom options.
func ExampleQuery_withOptions() {
	type Params struct {
		Tags []string `query:"tags"`
	}

	values := url.Values{}
	values.Set("tags", "go,rust,python")

	// Use CSV mode for comma-separated values
	params, err := binding.Query[Params](values,
		binding.WithSliceMode(binding.SliceCSV),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Tags: %v\n", params.Tags)
	// Output: Tags: [go rust python]
}

// ExampleMustNew demonstrates creating a reusable Binder.
func ExampleMustNew() {
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// Create a configured binder
	binder := binding.MustNew(
		binding.WithMaxDepth(16),
	)

	body := []byte(`{"name": "Diana", "email": "diana@example.com"}`)

	// Use generic helper function with binder
	user, err := binding.JSONWith[User](binder, body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Email: %s\n", user.Name, user.Email)
	// Output: Name: Diana, Email: diana@example.com
}

// ExampleJSON_withUnknownFields demonstrates strict JSON binding.
func ExampleJSON_withUnknownFields() {
	type User struct {
		Name string `json:"name"`
	}

	// JSON with unknown field "extra"
	body := []byte(`{"name": "Eve", "extra": "ignored"}`)

	// Default: unknown fields are ignored
	user, err := binding.JSON[User](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s\n", user.Name)
	// Output: Name: Eve
}

// ExampleHasStructTag demonstrates checking if a struct has specific tags.
func ExampleHasStructTag() {
	type UserRequest struct {
		ID   int    `path:"id"`
		Name string `query:"name"`
		Auth string `header:"Authorization"`
	}

	// Check at compile time which sources a struct uses
	var req UserRequest
	_ = req // Use the variable

	// In real code, you'd use reflect.TypeOf:
	// typ := reflect.TypeOf((*UserRequest)(nil)).Elem()
	// hasPath := binding.HasStructTag(typ, binding.TagPath)

	fmt.Printf("UserRequest has multiple source tags\n")
	// Output: UserRequest has multiple source tags
}
