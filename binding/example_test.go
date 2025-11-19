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
	"reflect"

	"rivaas.dev/binding"
)

// ExampleBind demonstrates basic binding from query parameters.
func ExampleBind() {
	type Params struct {
		Name  string `query:"name"`
		Age   int    `query:"age"`
		Email string `query:"email"`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("age", "30")
	values.Set("email", "alice@example.com")

	var params Params
	getter := binding.NewQueryGetter(values)
	err := binding.Bind(&params, getter, binding.TagQuery)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Age: %d, Email: %s\n", params.Name, params.Age, params.Email)
	// Output: Name: Alice, Age: 30, Email: alice@example.com
}

// ExampleBindInto demonstrates the generic BindInto helper.
func ExampleBindInto() {
	type Params struct {
		ID   int    `params:"id"`
		Name string `params:"name"`
	}

	paramsMap := map[string]string{
		"id":   "123",
		"name": "Bob",
	}

	params, err := binding.BindInto[Params](
		binding.NewParamsGetter(paramsMap),
		binding.TagParams,
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("ID: %d, Name: %s\n", params.ID, params.Name)
	// Output: ID: 123, Name: Bob
}

// ExampleBindMulti demonstrates binding from multiple sources.
func ExampleBindMulti() {
	type Request struct {
		// From path parameters
		UserID int `params:"user_id"`

		// From query string
		Page int `query:"page"`

		// From headers
		UserAgent string `header:"User-Agent"`
	}

	params := map[string]string{"user_id": "456"}
	query := url.Values{}
	query.Set("page", "2")

	sources := []binding.SourceConfig{
		{Tag: binding.TagParams, Getter: binding.NewParamsGetter(params)},
		{Tag: binding.TagQuery, Getter: binding.NewQueryGetter(query)},
		{Tag: binding.TagHeader, Getter: binding.NewHeaderGetter(map[string][]string{
			"User-Agent": {"MyApp/1.0"},
		})},
	}

	var req Request
	err := binding.BindMulti(&req, sources)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("UserID: %d, Page: %d, UserAgent: %s\n", req.UserID, req.Page, req.UserAgent)
	// Output: UserID: 456, Page: 2, UserAgent: MyApp/1.0
}

// ExampleHasStructTag demonstrates checking if a struct has specific tags.
func ExampleHasStructTag() {
	type UserRequest struct {
		ID   int    `params:"id"`
		Name string `query:"name"`
		Auth string `header:"Authorization"`
	}

	typ := reflect.TypeOf((*UserRequest)(nil)).Elem()

	hasParams := binding.HasStructTag(typ, binding.TagParams)
	hasQuery := binding.HasStructTag(typ, binding.TagQuery)
	hasCookie := binding.HasStructTag(typ, binding.TagCookie)

	fmt.Printf("Has params tag: %v\n", hasParams)
	fmt.Printf("Has query tag: %v\n", hasQuery)
	fmt.Printf("Has cookie tag: %v\n", hasCookie)
	// Output:
	// Has params tag: true
	// Has query tag: true
	// Has cookie tag: false
}

// ExampleBind_withDefaults demonstrates binding with default values.
func ExampleBind_withDefaults() {
	type Config struct {
		Port     int    `query:"port" default:"8080"`
		Host     string `query:"host" default:"localhost"`
		Debug    bool   `query:"debug" default:"false"`
		LogLevel string `query:"log_level" default:"info"`
	}

	// Empty query string - defaults will be applied
	values := url.Values{}
	var config Config
	getter := binding.NewQueryGetter(values)
	err := binding.Bind(&config, getter, binding.TagQuery)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Port: %d, Host: %s, Debug: %v, LogLevel: %s\n",
		config.Port, config.Host, config.Debug, config.LogLevel)
	// Output: Port: 8080, Host: localhost, Debug: false, LogLevel: info
}

// ExampleBind_withOptions demonstrates binding with custom options.
func ExampleBind_withOptions() {
	type Params struct {
		Birthday string `query:"birthday"`
	}

	values := url.Values{}
	values.Set("birthday", "2000-01-15")

	var params Params
	getter := binding.NewQueryGetter(values)

	// Use custom time layout
	err := binding.Bind(&params, getter, binding.TagQuery,
		binding.WithTimeLayouts("2006-01-02", "2006/01/02"),
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Birthday: %s\n", params.Birthday)
	// Output: Birthday: 2000-01-15
}
