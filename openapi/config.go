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
	"errors"
	"fmt"
	"strings"
	"sync"

	"rivaas.dev/openapi/internal/model"
)

// config holds construction-time OpenAPI configuration.
// Options mutate config; New() validates config and builds the API from it.
type config struct {
	info             model.Info
	servers          []model.Server
	tags             []model.Tag
	securitySchemes  map[string]*model.SecurityScheme
	defaultSecurity  []model.SecurityRequirement
	externalDocs     *model.ExternalDocs
	extensions       map[string]any
	version          Version
	strictDownlevel  bool
	specPath         string
	uiPath           string
	serveUI          bool
	validateSpec     bool
	ui               uiConfig
	operations       []Operation
	validationErrors []error // Errors from nil options (e.g. WithSwaggerUI)
}

// defaultConfig returns a config with default values.
func defaultConfig() *config {
	return &config{
		info: model.Info{
			Title:   "API",
			Version: "1.0.0",
		},
		securitySchemes: make(map[string]*model.SecurityScheme),
		version:         V30x,
		strictDownlevel: false,
		specPath:        "/openapi.json",
		uiPath:          "/docs",
		serveUI:         true,
		validateSpec:    false,
		ui:              defaultUIConfig(),
	}
}

// API holds OpenAPI configuration and defines an API specification.
// Configuration is read-only after creation; use getters to read values.
// Operations can be set at construction via [WithOperations] or added later via [API.AddOperation].
// Create instances using [New] or [MustNew].
type API struct {
	info            model.Info
	servers         []model.Server
	tags            []model.Tag
	securitySchemes map[string]*model.SecurityScheme
	defaultSecurity []model.SecurityRequirement
	externalDocs    *model.ExternalDocs
	extensions      map[string]any
	version         Version
	strictDownlevel bool
	specPath        string
	uiPath          string
	serveUI         bool
	validateSpec    bool
	ui              uiConfig
	operations      []Operation
	operationsMu    sync.RWMutex
}

// Option configures OpenAPI behavior using the functional options pattern.
// Options apply to an internal config struct; the constructor builds the API from the validated config.
// Options are applied in order, with later options potentially overriding earlier ones.
type Option func(*config)

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
//	    openapi.WithDescription("API description"),
//	    openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(opts ...Option) (*API, error) {
	cfg := defaultConfig()
	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("openapi: option at index %d cannot be nil", i)
		}
		opt(cfg)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return apiFromConfig(cfg), nil
}

// validateConfig checks that the config is valid.
func validateConfig(cfg *config) error {
	if len(cfg.validationErrors) > 0 {
		return errors.Join(cfg.validationErrors...)
	}
	if cfg.info.Title == "" {
		return ErrTitleRequired
	}
	if cfg.info.Version == "" {
		return ErrVersionRequired
	}
	if cfg.info.License != nil {
		if cfg.info.License.Identifier != "" && cfg.info.License.URL != "" {
			return ErrLicenseMutuallyExclusive
		}
	}
	for i, server := range cfg.servers {
		if len(server.Variables) > 0 && server.URL == "" {
			return fmt.Errorf("openapi: server[%d]: %w", i, ErrServerVariablesNeedURL)
		}
	}
	if err := cfg.ui.validate(); err != nil {
		return fmt.Errorf("openapi: %w", err)
	}
	for key := range cfg.extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: extension key must start with 'x-': %s", key)
		}
		if (cfg.version == V31x || cfg.version == Version("")) && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}
	for key := range cfg.info.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: info extension key must start with 'x-': %s", key)
		}
		if (cfg.version == V31x || cfg.version == Version("")) && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: info extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}
	return nil
}

// apiFromConfig builds an API from a validated config.
func apiFromConfig(cfg *config) *API {
	ops := cfg.operations
	if ops == nil {
		ops = []Operation{}
	}
	return &API{
		info:            cfg.info,
		servers:         cfg.servers,
		tags:            cfg.tags,
		securitySchemes: cfg.securitySchemes,
		defaultSecurity: cfg.defaultSecurity,
		externalDocs:    cfg.externalDocs,
		extensions:      cfg.extensions,
		version:         cfg.version,
		strictDownlevel: cfg.strictDownlevel,
		specPath:        cfg.specPath,
		uiPath:          cfg.uiPath,
		serveUI:         cfg.serveUI,
		validateSpec:    cfg.validateSpec,
		ui:              cfg.ui,
		operations:      ops,
	}
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
//	    openapi.WithDescription("API description"),
//	)
func MustNew(opts ...Option) *API {
	api, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return api
}

// UI returns a read-only snapshot of the Swagger UI configuration.
//
// Use the returned [UISnapshot] for rendering (e.g. ToJSON); do not use it for construction.
func (a *API) UI() UISnapshot {
	return &uiSnapshot{c: a.ui}
}

// Info returns the API metadata (title, version, description, contact, license).
// Do not modify the returned value.
func (a *API) Info() model.Info {
	return a.info
}

// Servers returns the list of server URLs. Do not modify the returned slice or its elements.
func (a *API) Servers() []model.Server {
	return a.servers
}

// Tags returns the tags. Do not modify the returned slice or its elements.
func (a *API) Tags() []model.Tag {
	return a.tags
}

// SecuritySchemes returns the security schemes map. Do not modify the returned map.
func (a *API) SecuritySchemes() map[string]*model.SecurityScheme {
	return a.securitySchemes
}

// DefaultSecurity returns the default security requirements. Do not modify the returned slice.
func (a *API) DefaultSecurity() []model.SecurityRequirement {
	return a.defaultSecurity
}

// ExternalDocs returns the external documentation link, or nil.
func (a *API) ExternalDocs() *model.ExternalDocs {
	return a.externalDocs
}

// Extensions returns the root-level specification extensions. Do not modify the returned map.
func (a *API) Extensions() map[string]any {
	return a.extensions
}

// Version returns the target OpenAPI version (V30x or V31x).
func (a *API) Version() Version {
	return a.version
}

// StrictDownlevel returns whether projection errors (instead of warns) for 3.1-only features when targeting 3.0.
func (a *API) StrictDownlevel() bool {
	return a.strictDownlevel
}

// SpecPath returns the HTTP path where the OpenAPI specification JSON is served.
func (a *API) SpecPath() string {
	return a.specPath
}

// UIPath returns the HTTP path where Swagger UI is served.
func (a *API) UIPath() string {
	return a.uiPath
}

// ServeUI returns whether Swagger UI is enabled.
func (a *API) ServeUI() bool {
	return a.serveUI
}

// ValidateSpec returns whether JSON Schema validation of generated specs is enabled.
func (a *API) ValidateSpec() bool {
	return a.validateSpec
}

// Validate checks if the [API] is valid.
//
// It ensures that required fields (title, version) are set and validates
// nested configurations like UI settings. Returns an error describing all
// validation failures.
//
// Validation is automatically called by [New] and [MustNew].
func (a *API) Validate() error {
	if a.info.Title == "" {
		return ErrTitleRequired
	}
	if a.info.Version == "" {
		return ErrVersionRequired
	}
	if a.info.License != nil {
		if a.info.License.Identifier != "" && a.info.License.URL != "" {
			return ErrLicenseMutuallyExclusive
		}
	}
	for i, server := range a.servers {
		if len(server.Variables) > 0 && server.URL == "" {
			return fmt.Errorf("openapi: server[%d]: %w", i, ErrServerVariablesNeedURL)
		}
	}
	if err := a.ui.validate(); err != nil {
		return fmt.Errorf("openapi: %w", err)
	}
	for key := range a.extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: extension key must start with 'x-': %s", key)
		}
		if (a.version == V31x || a.version == Version("")) && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}
	for key := range a.info.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: info extension key must start with 'x-': %s", key)
		}
		if (a.version == V31x || a.version == Version("")) && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
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
	return func(c *config) {
		c.info.Title = title
		c.info.Version = version
	}
}

// WithTitleIfDefault sets the API title and version only if they are still the defaults
// ("API" and "1.0.0"). Used by the app package to inject service name/version when
// the user has not set a custom title. Option order does not matter.
//
// Example (typically used by app, not by users directly):
//
//	openapi.New(append(userOpts, openapi.WithTitleIfDefault(serviceName, serviceVersion))...)
func WithTitleIfDefault(title, version string) Option {
	return func(c *config) {
		if c.info.Title == "API" && c.info.Version == "1.0.0" {
			if title != "" {
				c.info.Title = title
			}
			if version != "" {
				c.info.Version = version
			}
		}
	}
}

// WithDescription sets the API description in the Info object.
//
// The description supports Markdown formatting and appears in the OpenAPI spec
// and Swagger UI.
//
// Example:
//
//	openapi.WithDescription("A RESTful API for managing users and their profiles.")
func WithDescription(desc string) Option {
	return func(c *config) {
		c.info.Description = desc
	}
}

// WithInfoSummary sets the API summary in the Info object (OpenAPI 3.1+ only).
// In 3.0 targets, this will be dropped with a warning.
//
// Example:
//
//	openapi.WithInfoSummary("User Management API")
func WithInfoSummary(summary string) Option {
	return func(c *config) {
		c.info.Summary = summary
	}
}

// WithTermsOfService sets the Terms of Service URL/URI.
func WithTermsOfService(url string) Option {
	return func(c *config) {
		c.info.TermsOfService = url
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
	return func(c *config) {
		if c.info.Extensions == nil {
			c.info.Extensions = make(map[string]any)
		}
		c.info.Extensions[key] = value
	}
}

// WithExternalDocs sets external documentation URL and optional description.
func WithExternalDocs(url, description string) Option {
	return func(c *config) {
		c.externalDocs = &model.ExternalDocs{
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
	return func(c *config) {
		c.info.Contact = &model.Contact{
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
	return func(c *config) {
		c.info.License = &model.License{
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
	return func(c *config) {
		c.info.License = &model.License{
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
	return func(c *config) {
		c.servers = append(c.servers, model.Server{
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
	return func(c *config) {
		if len(c.servers) == 0 {
			c.servers = append(c.servers, model.Server{})
		}
		server := &c.servers[len(c.servers)-1]
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
	return func(c *config) {
		c.tags = append(c.tags, model.Tag{
			Name:        name,
			Description: desc,
		})
	}
}

// WithBearerAuth adds a Bearer (JWT) authentication scheme.
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
	return func(c *config) {
		if c.securitySchemes == nil {
			c.securitySchemes = make(map[string]*model.SecurityScheme)
		}
		c.securitySchemes[name] = &model.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  desc,
		}
	}
}

// WithAPIKey adds an API key authentication scheme.
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
	return func(c *config) {
		if c.securitySchemes == nil {
			c.securitySchemes = make(map[string]*model.SecurityScheme)
		}
		c.securitySchemes[name] = &model.SecurityScheme{
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

// OAuth2Flow configures a single OAuth2 flow with an explicit type.
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
	return func(c *config) {
		if c.securitySchemes == nil {
			c.securitySchemes = make(map[string]*model.SecurityScheme)
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
		c.securitySchemes[name] = &model.SecurityScheme{
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
	return func(c *config) {
		if c.securitySchemes == nil {
			c.securitySchemes = make(map[string]*model.SecurityScheme)
		}
		c.securitySchemes[name] = &model.SecurityScheme{
			Type:             "openIdConnect",
			Description:      desc,
			OpenIDConnectURL: url,
		}
	}
}

// SecurityRequirement builds a [SecurityReq] for use with [WithDefaultSecurity] or [WithSecurity].
//
// Example:
//
//	openapi.WithDefaultSecurity(
//	    openapi.SecurityRequirement("bearerAuth"),
//	    openapi.SecurityRequirement("oauth2", "read", "write"),
//	)
func SecurityRequirement(scheme string, scopes ...string) SecurityReq {
	return SecurityReq{Scheme: scheme, Scopes: scopes}
}

// WithDefaultSecurity sets default security requirements applied to all operations.
//
// Operations can override this via [WithSecurity] on the operation.
//
// Example:
//
//	openapi.WithDefaultSecurity(openapi.SecurityRequirement("bearerAuth"))
//	openapi.WithDefaultSecurity(openapi.SecurityRequirement("oauth2", "read", "write"))
func WithDefaultSecurity(requirements ...SecurityReq) Option {
	return func(c *config) {
		for _, r := range requirements {
			c.defaultSecurity = append(c.defaultSecurity, model.SecurityRequirement{
				r.Scheme: r.Scopes,
			})
		}
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
	return func(c *config) {
		c.version = version
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
	return func(c *config) {
		c.strictDownlevel = strict
	}
}

// WithOperations sets the operations included in the API at construction.
// Can be empty. Operations can also be added after construction via [API.AddOperation].
//
// Example:
//
//	openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithOperations(
//	        openapi.WithGET("/users/:id", openapi.WithSummary("Get user"), openapi.WithResponse(200, User{})),
//	        openapi.WithPOST("/users", openapi.WithSummary("Create user"), openapi.WithRequest(CreateUserRequest{}), openapi.WithResponse(201, User{})),
//	    ),
//	)
func WithOperations(ops ...Operation) Option {
	return func(c *config) {
		c.operations = ops
	}
}

// WithValidateSpec enables or disables JSON Schema validation of the generated OpenAPI spec.
//
// When enabled, Spec() validates the output against the official
// OpenAPI meta-schema and returns an error if the spec is invalid.
//
// This is useful for:
//   - Development: Catch spec generation bugs early
//   - CI/CD: Ensure generated specs are valid before deployment
//   - Testing: Verify spec correctness in tests
//
// Performance: Adds ~1-5ms overhead per generation. The default is false.
// Enable for development and testing to catch errors early.
//
// Default: false
//
// Example:
//
//	openapi.WithValidateSpec(true)
func WithValidateSpec(validate bool) Option {
	return func(c *config) {
		c.validateSpec = validate
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
	return func(c *config) {
		c.specPath = path
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
	return func(c *config) {
		if c.extensions == nil {
			c.extensions = make(map[string]any)
		}
		c.extensions[key] = value
	}
}
