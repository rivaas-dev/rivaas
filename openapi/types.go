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

// RouteInfo contains basic route information needed for OpenAPI generation.
// This is framework-agnostic and replaces router.RouteInfo.
type RouteInfo struct {
	Method string // HTTP method (GET, POST, etc.)
	Path   string // URL path with parameters (e.g. "/users/:id")
}
