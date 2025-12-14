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

// WithOptions composes multiple OperationOptions into a single option.
//
// This enables creating reusable option sets for common patterns across operations.
// Options are applied in the order they are provided, with later options potentially
// overriding values set by earlier options.
//
// Example:
//
//	// Define reusable option sets
//	var (
//	    CommonErrors = openapi.WithOptions(
//	        openapi.WithResponse(400, Error{}),
//	        openapi.WithResponse(401, Error{}),
//	        openapi.WithResponse(500, Error{}),
//	    )
//
//	    AuthRequired = openapi.WithOptions(
//	        openapi.WithSecurity("jwt"),
//	    )
//
//	    UserEndpoint = openapi.WithOptions(
//	        openapi.WithTags("users"),
//	        AuthRequired,
//	        CommonErrors,
//	    )
//	)
//
//	// Apply composed options to operations
//	openapi.GET("/users/:id",
//	    UserEndpoint,
//	    openapi.WithSummary("Get user"),
//	    openapi.WithResponse(200, User{}),
//	)
//
//	openapi.POST("/users",
//	    UserEndpoint,
//	    openapi.WithSummary("Create user"),
//	    openapi.WithRequest(CreateUser{}),
//	    openapi.WithResponse(201, User{}),
//	)
func WithOptions(opts ...OperationOption) OperationOption {
	return func(d *operationDoc) {
		for _, opt := range opts {
			opt(d)
		}
	}
}
