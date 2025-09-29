package build

import (
	"reflect"

	"rivaas.dev/openapi/internal/schema"
)

// RouteInfo contains basic route information needed for OpenAPI generation.
// This avoids importing the openapi package to prevent import cycles.
type RouteInfo struct {
	Method string // HTTP method (GET, POST, etc.)
	Path   string // URL path with parameters (e.g. "/users/:id")
}

// RouteDoc holds all OpenAPI metadata for a route.
// This is a copy of the openapi.RouteDoc structure to avoid import cycles.
type RouteDoc struct {
	Summary          string
	Description      string
	OperationID      string
	Tags             []string
	Deprecated       bool
	Consumes         []string
	Produces         []string
	RequestType      reflect.Type
	RequestMetadata  *schema.RequestMetadata
	ResponseTypes    map[int]reflect.Type
	ResponseExamples map[int]any
	Security         []SecurityReq
}

// SecurityReq represents a security requirement for an operation.
type SecurityReq struct {
	Scheme string
	Scopes []string
}

// EnrichedRoute combines route information with OpenAPI documentation.
//
// This type is used to pass route data to Builder.Build() for spec generation.
// The Doc field may be nil if the route has no OpenAPI documentation.
type EnrichedRoute struct {
	RouteInfo RouteInfo
	Doc       *RouteDoc
}
