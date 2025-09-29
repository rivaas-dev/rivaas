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
