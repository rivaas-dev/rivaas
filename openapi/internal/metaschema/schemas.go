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

package metaschema

import _ "embed"

// OAS30 contains the OpenAPI 3.0.x meta-schema JSON.
//
// This schema is used to validate OpenAPI 3.0.x specifications.
// The schema is obtained from https://spec.openapis.org/oas/3.0/schema/2024-10-18
//
//go:embed OpenAPI-v3.0.x.json
var OAS30 []byte

// OAS31 contains the OpenAPI 3.1.x meta-schema JSON.
//
// This schema is used to validate OpenAPI 3.1.x specifications.
// The schema is obtained from https://spec.openapis.org/oas/3.1/schema/2025-09-15
//
//go:embed OpenAPI-v3.1.x.json
var OAS31 []byte
