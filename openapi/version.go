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

// Version represents an OpenAPI specification version.
type Version string

// OpenAPI specification versions.
const (
	// V30x targets OpenAPI 3.0.x (widely supported).
	V30x Version = "3.0.x"

	// V31x targets OpenAPI 3.1.x (latest, with JSON Schema 2020-12).
	V31x Version = "3.1.x"
)

// String returns the version as a string.
func (v Version) String() string {
	return string(v)
}
