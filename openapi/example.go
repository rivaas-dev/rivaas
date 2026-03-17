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

package openapi

// Named examples for request and response bodies are created with the example package.
// Use example.New and example.NewExternal from rivaas.dev/openapi/example and pass
// the result to WithRequest or WithResponse:
//
//	import "rivaas.dev/openapi/example"
//
//	openapi.WithRequest(CreateUserRequest{},
//	    example.New("minimal", CreateUserRequest{Name: "J", Email: "j@example.com"}),
//	)
//	openapi.WithResponse(200, User{},
//	    example.New("success", User{ID: 1, Name: "John"}, example.WithSummary("Success")),
//	)
