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

import (
	"fmt"
	"strings"

	"rivaas.dev/openapi/internal/model"
)

// API holds OpenAPI configuration and defines an API specification.
// All fields are public for functional options, but direct modification after creation
// is not recommended. Use functional options to configure.
//
// Create instances using [New] or [MustNew].
type API struct {
	// Info contains API metadata (title, version, description, contact, license).
	Info model.Info

	// Servers lists available server URLs for the API.
	Servers []model.Server

	// Tags provides additional metadata for operations.
	Tags []model.Tag

	// SecuritySchemes defines available authentication/authorization schemes.
	SecuritySchemes map[string]*model.SecurityScheme

	// DefaultSecurity applies security requirements to all operations by default.
	DefaultSecurity []model.SecurityRequirement

	// ExternalDocs provides external documentation links.
	ExternalDocs *model.ExternalDocs

	// Extensions contains specification extensions (fields prefixed with x-).
	// Extensions are added to the root of the OpenAPI specification.
	//
	// Direct mutation of this map after New/MustNew bypasses API.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using WithExtension() option instead of direct mutation.
	Extensions map[string]any

	// Version is the target OpenAPI version.
	// Use V30x or V31x constants.
	// Default: V30x
	Version Version

	// StrictDownlevel causes projection to error (instead of warn) when
	// 3.1-only features are used with a 3.0 target.
	// Default: false
	StrictDownlevel bool

	// SpecPath is the HTTP path where the OpenAPI specification JSON is served.
	// Default: "/openapi.json"
	SpecPath string

	// UIPath is the HTTP path where Swagger UI is served.
	// Default: "/docs"
	UIPath string

	// ServeUI enables or disables Swagger UI serving.
	// Default: true
	ServeUI bool

	// ValidateSpec enables JSON Schema validation of generated specs.
	// When enabled, Generate validates the output against the official
	// OpenAPI meta-schema (3.0.x or 3.1.x based on target version).
	// This catches specification errors early but adds ~1-5ms overhead.
	// Default: false
	ValidateSpec bool

	// ui holds Swagger UI configuration (private to enforce functional options).
	ui UIConfig
}

// Option configures OpenAPI behavior using the functional options pattern.
// Options are applied in order, with later options potentially overriding earlier ones.
type Option func(*API)

// ParameterLocation represents where an API parameter can be located.
type ParameterLocation string

const (
	// InHeader indicates the parameter is passed in the HTTP header.
	InHeader ParameterLocation = "header"

	// InQuery indicates the parameter is passed as a query string parameter.
	InQuery ParameterLocation = "query"

	// InCookie indicates the parameter is passed as a cookie.
	InCookie ParameterLocation = "cookie"
)

// New creates a new OpenAPI [API] with the given options.
//
// It applies default values and validates the configuration. Returns an error if
// validation fails (e.g., missing title or version). Use [API.Validate] to check
// validation rules.
//
// Example:
//
//	api, err := openapi.New(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithInfoDescription("API description"),
//	    openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(opts ...Option) (*API, error) {
	api := &API{
		Info: model.Info{
			Title:   "API",
			Version: "1.0.0",
		},
		SecuritySchemes: make(map[string]*model.SecurityScheme),
		Version:         V30x,
		StrictDownlevel: false,
		SpecPath:        "/openapi.json",
		UIPath:          "/docs",
		ServeUI:         true,
		ValidateSpec:    false,
		ui:              defaultUIConfig(),
	}

	for _, opt := range opts {
		opt(api)
	}

	// Validate config
	if err := api.Validate(); err != nil {
		return nil, err
	}

	return api, nil
}

// MustNew creates a new OpenAPI [API] and panics if validation fails.
//
// This is a convenience wrapper around [New] for use in package initialization or
// when configuration errors should cause immediate failure.
//
// Example:
//
//	api := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithInfoDescription("API description"),
//	)
func MustNew(opts ...Option) *API {
	api, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return api
}

// UI returns the Swagger UI configuration.
//
// This provides access to UI settings for rendering the Swagger UI.
func (a *API) UI() UIConfig {
	return a.ui
}

// Validate checks if the [API] is valid.
//
// It ensures that required fields (title, version) are set and validates
// nested configurations like UI settings. Returns an error describing all
// validation failures.
//
// Validation is automatically called by [New] and [MustNew].
func (a *API) Validate() error {
	if a.Info.Title == "" {
		return ErrTitleRequired
	}
	if a.Info.Version == "" {
		return ErrVersionRequired
	}

	// Validate license: identifier and URL are mutually exclusive
	if a.Info.License != nil {
		if a.Info.License.Identifier != "" && a.Info.License.URL != "" {
			return ErrLicenseMutuallyExclusive
		}
	}

	// Validate servers: variables require a server URL
	for i, server := range a.Servers {
		if len(server.Variables) > 0 && server.URL == "" {
			return fmt.Errorf("openapi: server[%d]: %w", i, ErrServerVariablesNeedURL)
		}
	}

	// Validate UI config
	if err := a.ui.Validate(); err != nil {
		return fmt.Errorf("openapi: %w", err)
	}

	// Validate root-level extensions
	for key := range a.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: extension key must start with 'x-': %s", key)
		}
		// Check reserved prefixes for 3.1.x
		if (a.Version == V31x || a.Version == "") && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}

	// Validate Info extensions
	for key := range a.Info.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: info extension key must start with 'x-': %s", key)
		}
		// Check reserved prefixes for 3.1.x
		if (a.Version == V31x || a.Version == "") && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: info extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}

	return nil
}

// WithTitle sets the API title and version.
//
// Both title and version are required. If not set, defaults to "API" and "1.0.0".
//
// Example:
//
//	openapi.WithTitle("User Management API", "2.1.0")
func WithTitle(title, version string) Option {
	return func(a *API) {
		a.Info.Title = title
		a.Info.Version = version
	}
}

// WithInfoDescription sets the API description in the Info object.
//
// The description supports Markdown formatting and appears in the OpenAPI spec
// and Swagger UI.
//
// Example:
//
//	openapi.WithInfoDescription("A RESTful API for managing users and their profiles.")
func WithInfoDescription(desc string) Option {
	return func(a *API) {
		a.Info.Description = desc
	}
}

// WithInfoSummary sets the API summary in the Info object (OpenAPI 3.1+ only).
// In 3.0 targets, this will be dropped with a warning.
//
// Example:
//
//	openapi.WithInfoSummary("User Management API")
func WithInfoSummary(summary string) Option {
	return func(a *API) {
		a.Info.Summary = summary
	}
}

// WithTermsOfService sets the Terms of Service URL/URI.
func WithTermsOfService(url string) Option {
	return func(a *API) {
		a.Info.TermsOfService = url
	}
}

// WithInfoExtension adds a specification extension to the Info object.
//
// Extension keys must start with "x-". In OpenAPI 3.1.x, keys starting with
// "x-oai-" or "x-oas-" are reserved and cannot be used.
//
// Example:
//
//	openapi.WithInfoExtension("x-api-category", "public")
func WithInfoExtension(key string, value any) Option {
	return func(a *API) {
		if a.Info.Extensions == nil {
			a.Info.Extensions = make(map[string]any)
		}
		a.Info.Extensions[key] = value
	}
}

// WithExternalDocs sets external documentation URL and optional description.
func WithExternalDocs(url, description string) Option {
	return func(a *API) {
		a.ExternalDocs = &model.ExternalDocs{
			URL:         url,
			Description: description,
		}
	}
}

// WithContact sets contact information for the API.
//
// All parameters are optional. Empty strings are omitted from the specification.
//
// Example:
//
//	openapi.WithContact("API Support", "https://example.com/support", "support@example.com")
func WithContact(name, url, email string) Option {
	return func(a *API) {
		a.Info.Contact = &model.Contact{
			Name:  name,
			URL:   url,
			Email: email,
		}
	}
}

// WithLicense sets license information for the API using a URL (OpenAPI 3.0 style).
//
// The name is required. URL is optional.
// This is mutually exclusive with identifier - use WithLicenseIdentifier for SPDX identifiers.
// Validation occurs when New() is called.
//
// Example:
//
//	openapi.WithLicense("MIT", "https://opensource.org/licenses/MIT")
func WithLicense(name, url string) Option {
	return func(a *API) {
		a.Info.License = &model.License{
			Name: name,
			URL:  url,
		}
	}
}

// WithLicenseIdentifier sets license information for the API using an SPDX identifier (OpenAPI 3.1+).
//
// The name is required. Identifier is an SPDX license expression (e.g., "Apache-2.0").
// This is mutually exclusive with URL - use WithLicense for URL-based licenses.
// Validation occurs when New() is called.
//
// Example:
//
//	openapi.WithLicenseIdentifier("Apache 2.0", "Apache-2.0")
func WithLicenseIdentifier(name, identifier string) Option {
	return func(a *API) {
		a.Info.License = &model.License{
			Name:       name,
			Identifier: identifier,
		}
	}
}

// WithServer adds a server URL to the specification.
//
// Multiple servers can be added by calling this option multiple times.
// The description is optional and helps distinguish between environments.
//
// Example:
//
//	openapi.WithServer("https://api.example.com", "Production"),
//	openapi.WithServer("https://staging-api.example.com", "Staging"),
func WithServer(url, desc string) Option {
	return func(a *API) {
		a.Servers = append(a.Servers, model.Server{
			URL:         url,
			Description: desc,
		})
	}
}

// WithServerVariable adds a variable to the last added server for URL template substitution.
//
// The variable name should match a placeholder in the server URL (e.g., {username}).
// Default is required. Enum and description are optional.
//
// IMPORTANT: WithServerVariable must be called AFTER WithServer. It applies to the most
// recently added server. Validation occurs when New() is called.
//
// Example:
//
//	openapi.WithServer("https://{username}.example.com:{port}/v1", "Multi-tenant API"),
//	openapi.WithServerVariable("username", "demo", []string{"demo", "prod"}, "User subdomain"),
//	openapi.WithServerVariable("port", "8443", []string{"8443", "443"}, "Server port"),
func WithServerVariable(name, defaultValue string, enum []string, description string) Option {
	return func(a *API) {
		if len(a.Servers) == 0 {
			// Create a placeholder server - validation will catch this in Validate()
			a.Servers = append(a.Servers, model.Server{})
		}
		server := &a.Servers[len(a.Servers)-1]
		if server.Variables == nil {
			server.Variables = make(map[string]*model.ServerVariable)
		}
		server.Variables[name] = &model.ServerVariable{
			Enum:        enum,
			Default:     defaultValue,
			Description: description,
		}
	}
}

// WithTag adds a tag to the specification.
//
// Tags are used to group operations in Swagger UI. Operations can be assigned
// tags using RouteWrapper.Tags(). Multiple tags can be added by calling this
// option multiple times.
//
// Example:
//
//	openapi.WithTag("users", "User management operations"),
//	openapi.WithTag("orders", "Order processing operations"),
func WithTag(name, desc string) Option {
	return func(a *API) {
		a.Tags = append(a.Tags, model.Tag{
			Name:        name,
			Description: desc,
		})
	}
}

// WithBearerAuth adds Bearer (JWT) authentication scheme.
//
// The name is used to reference this scheme in security requirements.
// The description appears in Swagger UI to help users understand the authentication.
//
// Example:
//
//	openapi.WithBearerAuth("bearerAuth", "JWT token authentication. Format: Bearer <token>")
//
// Then use in routes:
//
//	app.GET("/protected", handler).Bearer()
func WithBearerAuth(name, desc string) Option {
	return func(a *API) {
		if a.SecuritySchemes == nil {
			a.SecuritySchemes = make(map[string]*model.SecurityScheme)
		}
		a.SecuritySchemes[name] = &model.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  desc,
		}
	}
}

// WithAPIKey adds API key authentication scheme.
//
// Parameters:
//   - name: Scheme name used in security requirements
//   - paramName: Name of the header/query parameter (e.g., "X-API-Key")
//   - in: Location of the API key - use InHeader, InQuery, or InCookie
//   - desc: Description shown in Swagger UI
//
// Example:
//
//	openapi.WithAPIKey("apiKey", "X-API-Key", openapi.InHeader, "API key in X-API-Key header")
func WithAPIKey(name, paramName string, in ParameterLocation, desc string) Option {
	return func(a *API) {
		if a.SecuritySchemes == nil {
			a.SecuritySchemes = make(map[string]*model.SecurityScheme)
		}
		a.SecuritySchemes[name] = &model.SecurityScheme{
			Type:        "apiKey",
			Name:        paramName,
			In:          string(in),
			Description: desc,
		}
	}
}

// OAuthFlowType represents the type of OAuth2 flow.
type OAuthFlowType string

const (
	// FlowImplicit represents the OAuth2 implicit flow.
	FlowImplicit OAuthFlowType = "implicit"
	// FlowPassword represents the OAuth2 resource owner password flow.
	FlowPassword OAuthFlowType = "password"
	// FlowClientCredentials represents the OAuth2 client credentials flow.
	FlowClientCredentials OAuthFlowType = "clientCredentials"
	// FlowAuthorizationCode represents the OAuth2 authorization code flow.
	FlowAuthorizationCode OAuthFlowType = "authorizationCode"
)

// OAuth2Flow configures a single OAuth2 flow with explicit type.
type OAuth2Flow struct {
	// Type specifies the OAuth2 flow type (implicit, password, clientCredentials, authorizationCode).
	Type OAuthFlowType

	// AuthorizationURL is required for implicit and authorizationCode flows.
	AuthorizationURL string

	// TokenURL is required for password, clientCredentials, and authorizationCode flows.
	TokenURL string

	// RefreshURL is optional for all flows.
	RefreshURL string

	// Scopes maps scope names to descriptions (required, can be empty).
	Scopes map[string]string
}

// WithOAuth2 adds OAuth2 authentication scheme.
//
// At least one flow must be configured. Use OAuth2Flow to configure each flow type.
// Multiple flows can be provided to support different OAuth2 flow types.
//
// Example:
//
//	openapi.WithOAuth2("oauth2", "OAuth2 authentication",
//		openapi.OAuth2Flow{
//			Type:             openapi.FlowAuthorizationCode,
//			AuthorizationURL: "https://example.com/oauth/authorize",
//			TokenURL:         "https://example.com/oauth/token",
//			Scopes: map[string]string{
//				"read":  "Read access",
//				"write": "Write access",
//			},
//		},
//		openapi.OAuth2Flow{
//			Type:     openapi.FlowClientCredentials,
//			TokenUrl: "https://example.com/oauth/token",
//			Scopes:   map[string]string{"read": "Read access"},
//		},
//	)
func WithOAuth2(name, desc string, flows ...OAuth2Flow) Option {
	return func(a *API) {
		if a.SecuritySchemes == nil {
			a.SecuritySchemes = make(map[string]*model.SecurityScheme)
		}
		oauthFlows := &model.OAuthFlows{}
		for _, flow := range flows {
			flowConfig := &model.OAuthFlow{
				AuthorizationURL: flow.AuthorizationURL,
				TokenURL:         flow.TokenURL,
				RefreshURL:       flow.RefreshURL,
				Scopes:           flow.Scopes,
			}
			switch flow.Type {
			case FlowImplicit:
				oauthFlows.Implicit = flowConfig
			case FlowPassword:
				oauthFlows.Password = flowConfig
			case FlowClientCredentials:
				oauthFlows.ClientCredentials = flowConfig
			case FlowAuthorizationCode:
				oauthFlows.AuthorizationCode = flowConfig
			}
		}
		a.SecuritySchemes[name] = &model.SecurityScheme{
			Type:        "oauth2",
			Description: desc,
			Flows:       oauthFlows,
		}
	}
}

// WithOpenIDConnect adds OpenID Connect authentication scheme.
//
// Parameters:
//   - name: Scheme name used in security requirements
//   - url: Well-known URL to discover OpenID Connect provider metadata
//   - desc: Description shown in Swagger UI
//
// Example:
//
//	openapi.WithOpenIDConnect("oidc", "https://example.com/.well-known/openid-configuration", "OpenID Connect authentication")
func WithOpenIDConnect(name, url, desc string) Option {
	return func(a *API) {
		if a.SecuritySchemes == nil {
			a.SecuritySchemes = make(map[string]*model.SecurityScheme)
		}
		a.SecuritySchemes[name] = &model.SecurityScheme{
			Type:             "openIdConnect",
			Description:      desc,
			OpenIDConnectURL: url,
		}
	}
}

// WithDefaultSecurity sets default security requirements applied to all operations.
//
// Operations can override this by specifying their own security requirements
// using RouteWrapper.Security() or RouteWrapper.Bearer().
//
// Example:
//
//	// Apply Bearer auth to all operations by default
//	openapi.WithDefaultSecurity("bearerAuth")
//
//	// Apply OAuth with specific scopes
//	openapi.WithDefaultSecurity("oauth2", "read", "write")
func WithDefaultSecurity(scheme string, scopes ...string) Option {
	return func(a *API) {
		a.DefaultSecurity = append(a.DefaultSecurity, model.SecurityRequirement{
			scheme: scopes,
		})
	}
}

// WithVersion sets the target OpenAPI version.
//
// Use V30x or V31x constants.
// Default: V30x
//
// Example:
//
//	openapi.WithVersion(openapi.V31x)
func WithVersion(version Version) Option {
	return func(a *API) {
		a.Version = version
	}
}

// WithStrictDownlevel causes projection to error (instead of warn) when
// 3.1-only features are used with a 3.0 target.
//
// Default: false (warnings only)
//
// Example:
//
//	openapi.WithStrictDownlevel(true)
func WithStrictDownlevel(strict bool) Option {
	return func(a *API) {
		a.StrictDownlevel = strict
	}
}

// WithValidation enables or disables JSON Schema validation of the generated OpenAPI spec.
//
// When enabled, Generate() validates the output against the official
// OpenAPI meta-schema and returns an error if the spec is invalid.
//
// This is useful for:
//   - Development: Catch spec generation bugs early
//   - CI/CD: Ensure generated specs are valid before deployment
//   - Testing: Verify spec correctness in tests
//
// Performance: Adds ~1-5ms overhead per generation. The default is false
// for backward compatibility. Enable for development and testing to catch
// errors early.
//
// Default: false
//
// Example:
//
//	openapi.WithValidation(false) // Disable for performance
func WithValidation(enabled bool) Option {
	return func(a *API) {
		a.ValidateSpec = enabled
	}
}

// WithSpecPath sets the HTTP path where the OpenAPI specification JSON is served.
//
// Default: "/openapi.json"
//
// Example:
//
//	openapi.WithSpecPath("/api/openapi.json")
func WithSpecPath(path string) Option {
	return func(a *API) {
		a.SpecPath = path
	}
}

// WithExtension adds a specification extension to the root OpenAPI specification.
//
// Extension keys MUST start with "x-". In OpenAPI 3.1.x, keys starting with
// "x-oai-" or "x-oas-" are reserved for the OpenAPI Initiative.
//
// The value can be any valid JSON value (null, primitive, array, or object).
// Validation of extension keys happens during API.Validate().
//
// Example:
//
//	openapi.WithExtension("x-internal-id", "api-v2")
//	openapi.WithExtension("x-code-samples", []map[string]any{
//	    {"lang": "curl", "source": "curl https://api.example.com/users"},
//	})
func WithExtension(key string, value any) Option {
	return func(a *API) {
		if a.Extensions == nil {
			a.Extensions = make(map[string]any)
		}
		a.Extensions[key] = value
	}
}
