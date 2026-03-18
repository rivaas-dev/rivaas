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

import "fmt"

// WithOptions composes multiple OperationOptions into a single option.
//
// This enables creating reusable option sets for common patterns across operations.
// Options are applied in the order they are provided, with later options potentially
// overriding values set by earlier options.
//
// WithOptions returns an error if any element of opts is nil (validation at compose time).
//
// Example:
//
//	// Define reusable option sets (check error at init)
//	CommonErrors, err := openapi.WithOptions(
//	    openapi.WithResponse(400, Error{}),
//	    openapi.WithResponse(401, Error{}),
//	    openapi.WithResponse(500, Error{}),
//	)
//	if err != nil {
//	    // handle err
//	}
//	UserEndpoint, err := openapi.WithOptions(
//	    openapi.WithTags("users"),
//	    AuthRequired,
//	    CommonErrors,
//	)
//	if err != nil {
//	    // handle err
//	}
//
//	// Apply composed options to operations
//	openapi.WithGET("/users/:id",
//	    UserEndpoint,
//	    openapi.WithSummary("Get user"),
//	    openapi.WithResponse(200, User{}),
//	)
//	openapi.WithPOST("/users",
//	    UserEndpoint,
//	    openapi.WithSummary("Create user"),
//	    openapi.WithRequest(CreateUser{}),
//	    openapi.WithResponse(201, User{}),
//	)
func WithOptions(opts ...OperationOption) (OperationOption, error) {
	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("openapi: operation option at index %d cannot be nil", i)
		}
	}
	return func(d *operationDoc) {
		for _, opt := range opts {
			opt(d)
		}
	}, nil
}
