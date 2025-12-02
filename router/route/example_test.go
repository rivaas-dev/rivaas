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

package route_test

import (
	"fmt"
	"net/url"

	"rivaas.dev/router/route"
)

// ExampleParseReversePattern demonstrates parsing a route path into segments
// for URL building (reverse routing).
func ExampleParseReversePattern() {
	pattern := route.ParseReversePattern("/users/:id/posts/:postId")

	for _, seg := range pattern.Segments {
		if seg.Static {
			fmt.Printf("Static: %s\n", seg.Value)
		} else {
			fmt.Printf("Param: %s\n", seg.Value)
		}
	}
	// Output:
	// Static: users
	// Param: id
	// Static: posts
	// Param: postId
}

// ExampleParseReversePattern_simple demonstrates parsing a simple static path.
func ExampleParseReversePattern_simple() {
	pattern := route.ParseReversePattern("/health")

	fmt.Printf("Segments: %d\n", len(pattern.Segments))
	fmt.Printf("First segment static: %v, value: %s\n", pattern.Segments[0].Static, pattern.Segments[0].Value)
	// Output:
	// Segments: 1
	// First segment static: true, value: health
}

// ExampleReversePattern_BuildURL demonstrates building URLs from a parsed pattern.
func ExampleReversePattern_BuildURL() {
	pattern := route.ParseReversePattern("/users/:id")

	url, err := pattern.BuildURL(map[string]string{"id": "123"}, nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(url)
	// Output:
	// /users/123
}

// ExampleReversePattern_BuildURL_withQuery demonstrates building URLs with query parameters.
func ExampleReversePattern_BuildURL_withQuery() {
	pattern := route.ParseReversePattern("/users/:id/posts")

	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "10")

	result, err := pattern.BuildURL(map[string]string{"id": "42"}, query)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(result)
	// Output:
	// /users/42/posts?limit=10&page=1
}

// ExampleReversePattern_BuildURL_missingParam demonstrates the error when a required parameter is missing.
func ExampleReversePattern_BuildURL_missingParam() {
	pattern := route.ParseReversePattern("/users/:id")

	_, err := pattern.BuildURL(map[string]string{}, nil)
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// missing required parameter: id
}

// ExampleParamConstraint_int demonstrates an integer constraint.
func ExampleParamConstraint_int() {
	pc := route.ParamConstraint{Kind: route.ConstraintInt}
	constraint := pc.ToRegexConstraint("id")

	fmt.Println("Matches '123':", constraint.Pattern.MatchString("123"))
	fmt.Println("Matches 'abc':", constraint.Pattern.MatchString("abc"))
	// Output:
	// Matches '123': true
	// Matches 'abc': false
}

// ExampleParamConstraint_uuid demonstrates a UUID constraint.
func ExampleParamConstraint_uuid() {
	pc := route.ParamConstraint{Kind: route.ConstraintUUID}
	constraint := pc.ToRegexConstraint("uuid")

	fmt.Println("Matches valid UUID:", constraint.Pattern.MatchString("550e8400-e29b-41d4-a716-446655440000"))
	fmt.Println("Matches invalid:", constraint.Pattern.MatchString("not-a-uuid"))
	// Output:
	// Matches valid UUID: true
	// Matches invalid: false
}

// ExampleParamConstraint_enum demonstrates an enum constraint.
func ExampleParamConstraint_enum() {
	pc := route.ParamConstraint{
		Kind: route.ConstraintEnum,
		Enum: []string{"draft", "published", "archived"},
	}
	constraint := pc.ToRegexConstraint("status")

	fmt.Println("Matches 'draft':", constraint.Pattern.MatchString("draft"))
	fmt.Println("Matches 'published':", constraint.Pattern.MatchString("published"))
	fmt.Println("Matches 'deleted':", constraint.Pattern.MatchString("deleted"))
	// Output:
	// Matches 'draft': true
	// Matches 'published': true
	// Matches 'deleted': false
}

// ExampleParamConstraint_regex demonstrates a custom regex constraint.
func ExampleParamConstraint_regex() {
	pc := route.ParamConstraint{
		Kind:    route.ConstraintRegex,
		Pattern: `[a-z][a-z0-9-]*`,
	}
	pc.Compile() // Compile the regex pattern

	constraint := pc.ToRegexConstraint("slug")

	fmt.Println("Matches 'hello-world':", constraint.Pattern.MatchString("hello-world"))
	fmt.Println("Matches '123abc':", constraint.Pattern.MatchString("123abc"))
	// Output:
	// Matches 'hello-world': true
	// Matches '123abc': false
}

// ExampleParamConstraint_date demonstrates a date constraint.
func ExampleParamConstraint_date() {
	pc := route.ParamConstraint{Kind: route.ConstraintDate}
	constraint := pc.ToRegexConstraint("date")

	fmt.Println("Matches '2025-12-01':", constraint.Pattern.MatchString("2025-12-01"))
	fmt.Println("Matches '12-01-2025':", constraint.Pattern.MatchString("12-01-2025"))
	// Output:
	// Matches '2025-12-01': true
	// Matches '12-01-2025': false
}

// ExampleInheritMiddleware demonstrates the InheritMiddleware mount option.
func ExampleInheritMiddleware() {
	opt := route.InheritMiddleware()
	cfg := &route.MountConfig{}
	opt(cfg)

	fmt.Println("InheritMiddleware:", cfg.InheritMiddleware)
	// Output:
	// InheritMiddleware: true
}

// ExampleWithMiddleware demonstrates the WithMiddleware mount option.
func ExampleWithMiddleware() {
	// Middleware functions would be added here
	// For this example, we show the configuration
	cfg := route.BuildMountConfig(
		route.InheritMiddleware(),
	)

	fmt.Println("InheritMiddleware:", cfg.InheritMiddleware)
	// Output:
	// InheritMiddleware: true
}

// ExampleNamePrefix demonstrates the NamePrefix mount option.
func ExampleNamePrefix() {
	cfg := route.BuildMountConfig(
		route.NamePrefix("api.v1."),
	)

	fmt.Println("NamePrefix:", cfg.NamePrefix)
	// Output:
	// NamePrefix: api.v1.
}

// ExampleBuildMountConfig demonstrates building mount configuration with multiple options.
func ExampleBuildMountConfig() {
	cfg := route.BuildMountConfig(
		route.InheritMiddleware(),
		route.NamePrefix("admin."),
	)

	fmt.Println("InheritMiddleware:", cfg.InheritMiddleware)
	fmt.Println("NamePrefix:", cfg.NamePrefix)
	// Output:
	// InheritMiddleware: true
	// NamePrefix: admin.
}

// ExampleConstraintFromPattern demonstrates creating a constraint from a regex pattern.
func ExampleConstraintFromPattern() {
	constraint := route.ConstraintFromPattern("id", `\d+`)

	fmt.Println("Param:", constraint.Param)
	fmt.Println("Matches '123':", constraint.Pattern.MatchString("123"))
	fmt.Println("Matches 'abc':", constraint.Pattern.MatchString("abc"))
	// Output:
	// Param: id
	// Matches '123': true
	// Matches 'abc': false
}
