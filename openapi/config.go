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
)

// OpenAPI version constants.
const (
	// Version30 represents OpenAPI 3.0.4.
	Version30 = "3.0.4"
	// Version31 represents OpenAPI 3.1.2.
	Version31 = "3.1.2"
)

// Static errors for validation
var (
	errTitleRequired            = errors.New("openapi: title is required")
	errVersionRequired          = errors.New("openapi: version is required")
	errLicenseMutuallyExclusive = errors.New("openapi: license identifier and url are mutually exclusive - provide only one")
	errServerVariablesNeedURL   = errors.New("openapi: server variables require a server URL")
)

// Config holds OpenAPI configuration.
// All fields are public for functional options, but direct modification after creation
// is not recommended. Use functional options to configure.
type Config struct {
	// Info contains API metadata (title, version, description, contact, license).
	Info Info

	// Servers lists available server URLs for the API.
	Servers []Server

	// Tags provides additional metadata for operations.
	Tags []Tag

	// SecuritySchemes defines available authentication/authorization schemes.
	SecuritySchemes map[string]*SecurityScheme

	// DefaultSecurity applies security requirements to all operations by default.
	DefaultSecurity []SecurityRequirement

	// ExternalDocs provides external documentation links.
	ExternalDocs *ExternalDocs

	// Extensions contains specification extensions (fields prefixed with x-).
	// Extensions are added to the root of the OpenAPI specification.
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using WithExtension() option instead of direct mutation.
	Extensions map[string]any

	// Version is the target OpenAPI version.
	// Use Version30 or Version31 constants.
	// Default: Version30
	Version string

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

	// ui holds Swagger UI configuration (private to enforce functional options).
	ui uiConfig
}

// Option configures OpenAPI behavior using the functional options pattern.
// Options are applied in order, with later options potentially overriding earlier ones.
type Option func(*Config)

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

// New creates a new OpenAPI Config with the given options.
//
// It applies default values and validates the configuration. Returns an error if
// validation fails (e.g., missing title or version).
//
// Example:
//
//	cfg, err := openapi.New(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithDescription("API description"),
//	    openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(opts ...Option) (*Config, error) {
	cfg := &Config{
		Info: Info{
			Title:   "API",
			Version: "1.0.0",
		},
		SecuritySchemes: make(map[string]*SecurityScheme),
		Version:         Version30,
		StrictDownlevel: false,
		SpecPath:        "/openapi.json",
		UIPath:          "/docs",
		ServeUI:         true,
		ui:              defaultUIConfig(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// MustNew creates a new OpenAPI Config and panics if validation fails.
//
// This is a convenience wrapper around New for use in package initialization or
// when configuration errors should cause immediate failure.
//
// Example:
//
//	cfg := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithDescription("API description"),
//	)
func MustNew(opts ...Option) *Config {
	cfg, err := New(opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

// Validate checks if the Config is valid.
//
// It ensures that required fields (title, version) are set and validates
// nested configurations like UI settings. Returns an error describing all
// validation failures.
//
// Validation is automatically called by New() and MustNew().
func (c *Config) Validate() error {
	if c.Info.Title == "" {
		return errTitleRequired
	}
	if c.Info.Version == "" {
		return errVersionRequired
	}

	// Validate license: identifier and URL are mutually exclusive
	if c.Info.License != nil {
		if c.Info.License.Identifier != "" && c.Info.License.URL != "" {
			return errLicenseMutuallyExclusive
		}
	}

	// Validate servers: variables require a server URL
	for i, server := range c.Servers {
		if len(server.Variables) > 0 && server.URL == "" {
			return fmt.Errorf("openapi: server[%d]: %w", i, errServerVariablesNeedURL)
		}
	}

	// Validate UI config
	if err := c.ui.Validate(); err != nil {
		return fmt.Errorf("openapi: %w", err)
	}

	// Validate root-level extensions
	for key := range c.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: extension key must start with 'x-': %s", key)
		}
		// Check reserved prefixes for 3.1.x
		if (c.Version == Version31 || c.Version == "") && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
			return fmt.Errorf("openapi: extension key uses reserved prefix (x-oai- or x-oas-): %s", key)
		}
	}

	// Validate Info extensions
	for key := range c.Info.Extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("openapi: info extension key must start with 'x-': %s", key)
		}
		// Check reserved prefixes for 3.1.x
		if (c.Version == Version31 || c.Version == "") && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
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
	return func(c *Config) {
		c.Info.Title = title
		c.Info.Version = version
	}
}

// WithDescription sets the API description.
//
// The description supports Markdown formatting and appears in the OpenAPI spec
// and Swagger UI.
//
// Example:
//
//	openapi.WithDescription("A RESTful API for managing users and their profiles.")
func WithDescription(desc string) Option {
	return func(c *Config) {
		c.Info.Description = desc
	}
}

// WithSummary sets the API summary (OpenAPI 3.1+ only).
// In 3.0 targets, this will be dropped with a warning.
func WithSummary(summary string) Option {
	return func(c *Config) {
		c.Info.Summary = summary
	}
}

// WithTermsOfService sets the Terms of Service URL/URI.
func WithTermsOfService(url string) Option {
	return func(c *Config) {
		c.Info.TermsOfService = url
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
	return func(c *Config) {
		if c.Info.Extensions == nil {
			c.Info.Extensions = make(map[string]any)
		}
		c.Info.Extensions[key] = value
	}
}

// WithExternalDocs sets external documentation URL and optional description.
func WithExternalDocs(url, description string) Option {
	return func(c *Config) {
		c.ExternalDocs = &ExternalDocs{
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
	return func(c *Config) {
		c.Info.Contact = &Contact{
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
	return func(c *Config) {
		c.Info.License = &License{
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
	return func(c *Config) {
		c.Info.License = &License{
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
	return func(c *Config) {
		c.Servers = append(c.Servers, Server{
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
	return func(c *Config) {
		if len(c.Servers) == 0 {
			// Create a placeholder server - validation will catch this in Validate()
			c.Servers = append(c.Servers, Server{})
		}
		server := &c.Servers[len(c.Servers)-1]
		if server.Variables == nil {
			server.Variables = make(map[string]*ServerVariable)
		}
		server.Variables[name] = &ServerVariable{
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
	return func(c *Config) {
		c.Tags = append(c.Tags, Tag{
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
	return func(c *Config) {
		if c.SecuritySchemes == nil {
			c.SecuritySchemes = make(map[string]*SecurityScheme)
		}
		c.SecuritySchemes[name] = &SecurityScheme{
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
	return func(c *Config) {
		if c.SecuritySchemes == nil {
			c.SecuritySchemes = make(map[string]*SecurityScheme)
		}
		c.SecuritySchemes[name] = &SecurityScheme{
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

	// AuthorizationUrl is required for implicit and authorizationCode flows.
	AuthorizationUrl string

	// TokenUrl is required for password, clientCredentials, and authorizationCode flows.
	TokenUrl string

	// RefreshUrl is optional for all flows.
	RefreshUrl string

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
//			AuthorizationUrl: "https://example.com/oauth/authorize",
//			TokenUrl:         "https://example.com/oauth/token",
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
	return func(c *Config) {
		if c.SecuritySchemes == nil {
			c.SecuritySchemes = make(map[string]*SecurityScheme)
		}
		oauthFlows := &OAuthFlows{}
		for _, flow := range flows {
			flowConfig := &OAuthFlow{
				AuthorizationUrl: flow.AuthorizationUrl,
				TokenUrl:         flow.TokenUrl,
				RefreshUrl:       flow.RefreshUrl,
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
		c.SecuritySchemes[name] = &SecurityScheme{
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
	return func(c *Config) {
		if c.SecuritySchemes == nil {
			c.SecuritySchemes = make(map[string]*SecurityScheme)
		}
		c.SecuritySchemes[name] = &SecurityScheme{
			Type:             "openIdConnect",
			Description:      desc,
			OpenIdConnectUrl: url,
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
	return func(c *Config) {
		c.DefaultSecurity = append(c.DefaultSecurity, SecurityRequirement{
			scheme: scopes,
		})
	}
}

// WithVersion sets the target OpenAPI version.
//
// Use Version30 or Version31 constants.
// Default: Version30
//
// Example:
//
//	openapi.WithVersion(openapi.Version31)
func WithVersion(version string) Option {
	return func(c *Config) {
		c.Version = version
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
	return func(c *Config) {
		c.StrictDownlevel = strict
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
	return func(c *Config) {
		c.SpecPath = path
	}
}

// WithSwaggerUI enables or disables Swagger UI and optionally sets its path.
//
// If enabled (default), Swagger UI is served at the configured UIPath.
// The path parameter is optional - if not provided, uses the default "/docs".
//
// Example:
//
//	// Enable with default path (/docs)
//	openapi.WithSwaggerUI(true)
//
//	// Enable with custom path
//	openapi.WithSwaggerUI(true, "/swagger")
//
//	// Disable Swagger UI
//	openapi.WithSwaggerUI(false)
func WithSwaggerUI(enabled bool, path ...string) Option {
	return func(c *Config) {
		c.ServeUI = enabled
		if len(path) > 0 {
			c.UIPath = path[0]
		}
	}
}

// WithUIDeepLinking enables or disables deep linking in Swagger UI.
//
// When enabled, Swagger UI updates the browser URL when operations are expanded,
// allowing direct linking to specific operations. Default: true.
//
// Example:
//
//	openapi.WithUIDeepLinking(true)
func WithUIDeepLinking(enabled bool) Option {
	return func(c *Config) {
		c.ui.DeepLinking = enabled
	}
}

// WithUIDisplayOperationID shows or hides operation IDs in Swagger UI.
//
// Operation IDs are useful for code generation and API client libraries.
// Default: false.
//
// Example:
//
//	openapi.WithUIDisplayOperationID(true)
func WithUIDisplayOperationID(show bool) Option {
	return func(c *Config) {
		c.ui.DisplayOperationID = show
	}
}

// WithUIDocExpansion sets the default expansion level for operations and tags.
//
// Valid modes:
//   - DocExpansionList: Expand only tags (default)
//   - DocExpansionFull: Expand tags and operations
//   - DocExpansionNone: Collapse everything
//
// Example:
//
//	openapi.WithUIDocExpansion(openapi.DocExpansionFull)
func WithUIDocExpansion(mode DocExpansionMode) Option {
	return func(c *Config) {
		c.ui.DocExpansion = mode
	}
}

// WithUIModelsExpandDepth sets the default expansion depth for model schemas.
//
// Depth controls how many levels of nested properties are expanded by default.
// Use -1 to hide models completely. Default: 1.
//
// Example:
//
//	openapi.WithUIModelsExpandDepth(2) // Expand 2 levels deep
func WithUIModelsExpandDepth(depth int) Option {
	return func(c *Config) {
		c.ui.DefaultModelsExpandDepth = depth
	}
}

// WithUIModelExpandDepth sets the default expansion depth for model example sections.
//
// Controls how many levels of the example value are expanded. Default: 1.
//
// Example:
//
//	openapi.WithUIModelExpandDepth(3)
func WithUIModelExpandDepth(depth int) Option {
	return func(c *Config) {
		c.ui.DefaultModelExpandDepth = depth
	}
}

// WithUIDefaultModelRendering sets the initial model display mode.
//
// Valid modes:
//   - ModelRenderingExample: Show example value (default)
//   - ModelRenderingModel: Show model structure
//
// Example:
//
//	openapi.WithUIDefaultModelRendering(openapi.ModelRenderingModel)
func WithUIDefaultModelRendering(mode ModelRenderingMode) Option {
	return func(c *Config) {
		c.ui.DefaultModelRendering = mode
	}
}

// WithUITryItOut enables or disables "Try it out" functionality by default.
//
// When enabled, the "Try it out" button is automatically expanded for all operations.
// Default: true.
//
// Example:
//
//	openapi.WithUITryItOut(false) // Require users to click "Try it out"
func WithUITryItOut(enabled bool) Option {
	return func(c *Config) {
		c.ui.TryItOutEnabled = enabled
	}
}

// WithUIRequestSnippets enables or disables code snippet generation.
//
// When enabled, Swagger UI generates code snippets showing how to call the API
// in various languages (curl, etc.). The languages parameter specifies which
// snippet generators to include. If not provided, defaults to curl_bash.
//
// Example:
//
//	openapi.WithUIRequestSnippets(true, openapi.SnippetCurlBash, openapi.SnippetCurlPowerShell)
func WithUIRequestSnippets(enabled bool, languages ...RequestSnippetLanguage) Option {
	return func(c *Config) {
		c.ui.RequestSnippetsEnabled = enabled
		if len(languages) > 0 {
			c.ui.RequestSnippets.Languages = languages
		}
	}
}

// WithUIRequestSnippetsExpanded sets whether request snippets are expanded by default.
//
// When true, code snippets are shown immediately without requiring user interaction.
// Default: false.
//
// Example:
//
//	openapi.WithUIRequestSnippetsExpanded(true)
func WithUIRequestSnippetsExpanded(expanded bool) Option {
	return func(c *Config) {
		c.ui.RequestSnippets.DefaultExpanded = expanded
	}
}

// WithUIDisplayRequestDuration shows or hides request duration in Swagger UI.
//
// When enabled, the time taken for "Try it out" requests is displayed.
// Default: true.
//
// Example:
//
//	openapi.WithUIDisplayRequestDuration(true)
func WithUIDisplayRequestDuration(show bool) Option {
	return func(c *Config) {
		c.ui.DisplayRequestDuration = show
	}
}

// WithUIFilter enables or disables the operation filter/search box.
//
// When enabled, users can filter operations by typing in a search box.
// Default: false.
//
// Example:
//
//	openapi.WithUIFilter(true)
func WithUIFilter(enabled bool) Option {
	return func(c *Config) {
		c.ui.Filter = enabled
	}
}

// WithUIMaxDisplayedTags limits the number of tags displayed in Swagger UI.
//
// When set to a positive number, only the first N tags are shown. Remaining tags
// are hidden. Use 0 or negative to show all tags. Default: 0 (show all).
//
// Example:
//
//	openapi.WithUIMaxDisplayedTags(10) // Show only first 10 tags
func WithUIMaxDisplayedTags(max int) Option {
	return func(c *Config) {
		c.ui.MaxDisplayedTags = max
	}
}

// WithUIOperationsSorter sets how operations are sorted within tags.
//
// Valid modes:
//   - OperationsSorterAlpha: Sort alphabetically by path
//   - OperationsSorterMethod: Sort by HTTP method (GET, POST, etc.)
//   - OperationsSorterNone: Use server order (no sorting, default)
//
// Example:
//
//	openapi.WithUIOperationsSorter(openapi.OperationsSorterAlpha)
func WithUIOperationsSorter(mode OperationsSorterMode) Option {
	return func(c *Config) {
		c.ui.OperationsSorter = mode
	}
}

// WithUITagsSorter sets how tags are sorted in Swagger UI.
//
// Valid modes:
//   - TagsSorterAlpha: Sort tags alphabetically
//   - TagsSorterNone: Use server order (no sorting, default)
//
// Example:
//
//	openapi.WithUITagsSorter(openapi.TagsSorterAlpha)
func WithUITagsSorter(mode TagsSorterMode) Option {
	return func(c *Config) {
		c.ui.TagsSorter = mode
	}
}

// WithUISyntaxHighlight enables or disables syntax highlighting in Swagger UI.
//
// When enabled, request/response examples and code snippets are syntax-highlighted
// using the configured theme. Default: true.
//
// Example:
//
//	openapi.WithUISyntaxHighlight(true)
func WithUISyntaxHighlight(enabled bool) Option {
	return func(c *Config) {
		c.ui.SyntaxHighlight.Activated = enabled
	}
}

// WithUISyntaxTheme sets the syntax highlighting theme for code examples.
//
// Available themes: Agate, Arta, Monokai, Nord, Obsidian, TomorrowNight, Idea.
// Default: Agate.
//
// Example:
//
//	openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai)
func WithUISyntaxTheme(theme SyntaxTheme) Option {
	return func(c *Config) {
		c.ui.SyntaxHighlight.Theme = theme
	}
}

// WithUIValidator sets the OpenAPI specification validator URL.
//
// Swagger UI can validate your OpenAPI spec against a validator service.
// Use an empty string or "none" to disable validation. Default: uses Swagger UI's
// default validator.
//
// Example:
//
//	openapi.WithUIValidator("https://validator.swagger.io/validator")
//	openapi.WithUIValidator("") // Disable validation
func WithUIValidator(url string) Option {
	return func(c *Config) {
		c.ui.ValidatorURL = url
	}
}

// WithUIPersistAuth enables or disables authorization persistence.
//
// When enabled, authorization tokens are persisted in browser storage and
// automatically included in subsequent requests. Default: false.
//
// Example:
//
//	openapi.WithUIPersistAuth(true)
func WithUIPersistAuth(enabled bool) Option {
	return func(c *Config) {
		c.ui.PersistAuthorization = enabled
	}
}

// WithUIWithCredentials enables or disables credentials in CORS requests.
//
// When enabled, cookies and authorization headers are included in cross-origin
// requests. Only enable if your API server is configured to accept credentials.
// Default: false.
//
// Example:
//
//	openapi.WithUIWithCredentials(true)
func WithUIWithCredentials(enabled bool) Option {
	return func(c *Config) {
		c.ui.WithCredentials = enabled
	}
}

// WithUISupportedMethods sets which HTTP methods have "Try it out" enabled.
//
// By default, all standard HTTP methods support "Try it out". Use this option
// to restrict which methods can be tested interactively in Swagger UI.
//
// Example:
//
//	openapi.WithUISupportedMethods(openapi.MethodGet, openapi.MethodPost)
func WithUISupportedMethods(methods ...HTTPMethod) Option {
	return func(c *Config) {
		c.ui.SupportedSubmitMethods = methods
	}
}

// WithUIShowExtensions shows or hides vendor extensions (x-* fields) in Swagger UI.
//
// Vendor extensions are custom fields prefixed with "x-" in the OpenAPI spec.
// Default: false.
//
// Example:
//
//	openapi.WithUIShowExtensions(true)
func WithUIShowExtensions(show bool) Option {
	return func(c *Config) {
		c.ui.ShowExtensions = show
	}
}

// WithUIShowCommonExtensions shows or hides common JSON Schema extensions.
//
// When enabled, displays schema constraints like pattern, maxLength, minLength,
// etc. in the UI. Default: false.
//
// Example:
//
//	openapi.WithUIShowCommonExtensions(true)
func WithUIShowCommonExtensions(show bool) Option {
	return func(c *Config) {
		c.ui.ShowCommonExtensions = show
	}
}

// WithExtension adds a specification extension to the root OpenAPI specification.
//
// Extension keys MUST start with "x-". In OpenAPI 3.1.x, keys starting with
// "x-oai-" or "x-oas-" are reserved for the OpenAPI Initiative.
//
// The value can be any valid JSON value (null, primitive, array, or object).
// Validation of extension keys happens during Config.Validate().
//
// Example:
//
//	openapi.WithExtension("x-internal-id", "api-v2")
//	openapi.WithExtension("x-code-samples", []map[string]any{
//	    {"lang": "curl", "source": "curl https://api.example.com/users"},
//	})
func WithExtension(key string, value any) Option {
	return func(c *Config) {
		if c.Extensions == nil {
			c.Extensions = make(map[string]any)
		}
		c.Extensions[key] = value
	}
}
