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
	"crypto/sha256"
	"fmt"
	"maps"
	"reflect"
	"sync"

	"rivaas.dev/openapi/export"
	"rivaas.dev/openapi/internal/build"
	"rivaas.dev/openapi/internal/metaschema"
	"rivaas.dev/openapi/model"
)

// Manager manages OpenAPI specification generation and caching.
//
// It coordinates between route metadata and schema generation to provide
// a complete OpenAPI documentation solution.
//
// Concurrency: Manager is safe for concurrent use. The Register method
// and GenerateSpec can be called from multiple goroutines safely. The
// manager uses internal locking to protect its state and cache.
//
// Manager instances are created via NewManager() and should not be constructed
// directly.
type Manager struct {
	builder      *build.Builder
	cfg          *Config
	mu           sync.RWMutex
	routes       []*RouteWrapper
	cacheInvalid bool
	specJSON     []byte
	etag         string
	lastWarnings []export.Warning // Warnings from last successful spec generation
}

// NewManager creates a new OpenAPI manager from configuration.
//
// The manager is initialized with the provided configuration and is ready to
// register routes and generate OpenAPI specifications. Returns nil if cfg is nil.
//
// Example:
//
//	cfg := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithSwaggerUI(true, "/docs"),
//	)
//	manager := openapi.NewManager(cfg)
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		return nil
	}

	// Convert Config types to model types
	info := model.Info{
		Title:          cfg.Info.Title,
		Summary:        cfg.Info.Summary,
		Description:    cfg.Info.Description,
		TermsOfService: cfg.Info.TermsOfService,
		Version:        cfg.Info.Version,
	}
	if len(cfg.Info.Extensions) > 0 {
		info.Extensions = make(map[string]any, len(cfg.Info.Extensions))
		maps.Copy(info.Extensions, cfg.Info.Extensions)
	}
	if cfg.Info.Contact != nil {
		info.Contact = &model.Contact{
			Name:  cfg.Info.Contact.Name,
			URL:   cfg.Info.Contact.URL,
			Email: cfg.Info.Contact.Email,
		}
		if len(cfg.Info.Contact.Extensions) > 0 {
			info.Contact.Extensions = make(map[string]any, len(cfg.Info.Contact.Extensions))
			maps.Copy(info.Contact.Extensions, cfg.Info.Contact.Extensions)
		}
	}
	if cfg.Info.License != nil {
		info.License = &model.License{
			Name:       cfg.Info.License.Name,
			Identifier: cfg.Info.License.Identifier,
			URL:        cfg.Info.License.URL,
		}
		if len(cfg.Info.License.Extensions) > 0 {
			info.License.Extensions = make(map[string]any, len(cfg.Info.License.Extensions))
			maps.Copy(info.License.Extensions, cfg.Info.License.Extensions)
		}
	}

	b := build.NewBuilder(info)

	// Set external documentation if provided
	if cfg.ExternalDocs != nil {
		extDocs := &model.ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
		if len(cfg.ExternalDocs.Extensions) > 0 {
			extDocs.Extensions = make(map[string]any, len(cfg.ExternalDocs.Extensions))
			maps.Copy(extDocs.Extensions, cfg.ExternalDocs.Extensions)
		}
		b.SetExternalDocs(extDocs)
	}

	for _, s := range cfg.Servers {
		b.AddServerWithExtensions(s.URL, s.Description, s.Extensions)
		// Add variables if present
		if len(s.Variables) > 0 {
			for name, v := range s.Variables {
				sv := &model.ServerVariable{
					Enum:        v.Enum,
					Default:     v.Default,
					Description: v.Description,
				}
				if len(v.Extensions) > 0 {
					sv.Extensions = make(map[string]any, len(v.Extensions))
					maps.Copy(sv.Extensions, v.Extensions)
				}
				b.AddServerVariable(name, sv)
			}
		}
	}

	for _, t := range cfg.Tags {
		if t.ExternalDocs != nil {
			extDocs := &model.ExternalDocs{
				Description: t.ExternalDocs.Description,
				URL:         t.ExternalDocs.URL,
			}
			if len(t.ExternalDocs.Extensions) > 0 {
				extDocs.Extensions = make(map[string]any, len(t.ExternalDocs.Extensions))
				maps.Copy(extDocs.Extensions, t.ExternalDocs.Extensions)
			}
			b.AddTagWithExternalDocs(t.Name, t.Description, extDocs, t.Extensions)
		} else {
			b.AddTagWithExtensions(t.Name, t.Description, t.Extensions)
		}
	}

	// Convert security schemes
	if len(cfg.SecuritySchemes) > 0 {
		for name, s := range cfg.SecuritySchemes {
			ss := &model.SecurityScheme{
				Type:             s.Type,
				Description:      s.Description,
				Name:             s.Name,
				In:               s.In,
				Scheme:           s.Scheme,
				BearerFormat:     s.BearerFormat,
				OpenIdConnectUrl: s.OpenIdConnectUrl,
			}
			if s.Flows != nil {
				ss.Flows = &model.OAuthFlows{}
				if s.Flows.Implicit != nil {
					ss.Flows.Implicit = &model.OAuthFlow{
						AuthorizationUrl: s.Flows.Implicit.AuthorizationUrl,
						TokenUrl:         s.Flows.Implicit.TokenUrl,
						RefreshUrl:       s.Flows.Implicit.RefreshUrl,
						Scopes:           s.Flows.Implicit.Scopes,
					}
				}
				if s.Flows.Password != nil {
					ss.Flows.Password = &model.OAuthFlow{
						AuthorizationUrl: s.Flows.Password.AuthorizationUrl,
						TokenUrl:         s.Flows.Password.TokenUrl,
						RefreshUrl:       s.Flows.Password.RefreshUrl,
						Scopes:           s.Flows.Password.Scopes,
					}
				}
				if s.Flows.ClientCredentials != nil {
					ss.Flows.ClientCredentials = &model.OAuthFlow{
						AuthorizationUrl: s.Flows.ClientCredentials.AuthorizationUrl,
						TokenUrl:         s.Flows.ClientCredentials.TokenUrl,
						RefreshUrl:       s.Flows.ClientCredentials.RefreshUrl,
						Scopes:           s.Flows.ClientCredentials.Scopes,
					}
				}
				if s.Flows.AuthorizationCode != nil {
					ss.Flows.AuthorizationCode = &model.OAuthFlow{
						AuthorizationUrl: s.Flows.AuthorizationCode.AuthorizationUrl,
						TokenUrl:         s.Flows.AuthorizationCode.TokenUrl,
						RefreshUrl:       s.Flows.AuthorizationCode.RefreshUrl,
						Scopes:           s.Flows.AuthorizationCode.Scopes,
					}
				}
			}
			b.AddSecurityScheme(name, ss)
		}
	}

	if len(cfg.DefaultSecurity) > 0 {
		sec := make([]model.SecurityRequirement, len(cfg.DefaultSecurity))
		for i, r := range cfg.DefaultSecurity {
			sec[i] = model.SecurityRequirement(r)
		}
		b.SetGlobalSecurity(sec)
	}

	return &Manager{
		builder:      b,
		cfg:          cfg,
		routes:       []*RouteWrapper{},
		cacheInvalid: true,
	}
}

// Register registers a route with OpenAPI metadata.
//
// This method creates and returns a RouteWrapper that allows adding OpenAPI
// documentation through a fluent API. The route is tracked internally for
// spec generation.
//
// Panics if m is nil.
//
// Example:
//
//	manager.Register("GET", "/users/:id").
//	    Doc("Get user", "Retrieves a user by ID").
//	    Request(GetUserRequest{}).
//	    Response(200, UserResponse{})
func (m *Manager) Register(method, path string) *RouteWrapper {
	if m == nil {
		panic("openapi.Manager.Register called on nil manager")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	w := NewRoute(method, path)
	m.routes = append(m.routes, w)
	m.cacheInvalid = true
	m.lastWarnings = nil // Clear warnings when cache is invalidated

	return w
}

// OnRouteAdded is called when a route is registered with the router.
// It creates a RouteWrapper for OpenAPI documentation and applies constraint mapping.
// This enables automatic OpenAPI schema generation from typed route constraints.
//
// The route parameter should be a *router.Route. We use an interface to avoid
// circular dependencies between openapi and router packages.
func (m *Manager) OnRouteAdded(route any) *RouteWrapper {
	if m == nil {
		return nil
	}

	// Use reflection to extract method and path from *router.Route
	// This allows us to work with router.Route without importing router package
	rv := reflect.ValueOf(route)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	var method, path string

	// Try Method() method first (preferred)
	if mth := rv.MethodByName("Method"); mth.IsValid() {
		if res := mth.Call(nil); len(res) > 0 && res[0].Kind() == reflect.String {
			method = res[0].String()
		}
	}
	// Fallback to method field
	if method == "" {
		if mf := rv.FieldByName("method"); mf.IsValid() && mf.Kind() == reflect.String {
			method = mf.String()
		}
	}

	// Try Path() method first (preferred)
	if mth := rv.MethodByName("Path"); mth.IsValid() {
		if res := mth.Call(nil); len(res) > 0 && res[0].Kind() == reflect.String {
			path = res[0].String()
		}
	}
	// Fallback to path field
	if path == "" {
		if pf := rv.FieldByName("path"); pf.IsValid() && pf.Kind() == reflect.String {
			path = pf.String()
		}
	}

	if method == "" || path == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	w := NewRoute(method, path)
	m.routes = append(m.routes, w)
	m.cacheInvalid = true
	m.lastWarnings = nil

	// TODO: Apply constraint mapping to OpenAPI parameters
	// This will map router.ParamConstraint to OpenAPI parameter schemas
	// For now, just register the route - constraint mapping can be added later

	return w
}

// GenerateSpec generates the OpenAPI specification JSON.
//
// This method:
//   - Freezes all route wrappers to make metadata immutable
//   - Builds the IR specification
//   - Projects to the target OpenAPI version
//   - Caches the result for subsequent calls
//
// Returns the JSON bytes and ETag. The ETag can be used for HTTP caching.
//
// Example:
//
//	specJSON, etag, err := manager.GenerateSpec()
//	if err != nil {
//	    log.Fatal(err)
//	}
func (m *Manager) GenerateSpec() ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.cacheInvalid && m.specJSON != nil {
		// Return cached spec with cached warnings
		return m.specJSON, m.etag, nil
	}

	// Freeze all routes
	for _, w := range m.routes {
		w.Freeze()
	}

	// Build enriched routes
	enriched := make([]build.EnrichedRoute, len(m.routes))
	for i, w := range m.routes {
		ri := w.Info()
		doc := w.GetFrozenDoc()
		var buildDoc *build.RouteDoc
		if doc != nil {
			buildDoc = &build.RouteDoc{
				Summary:          doc.Summary,
				Description:      doc.Description,
				OperationID:      doc.OperationID,
				Tags:             doc.Tags,
				Deprecated:       doc.Deprecated,
				Consumes:         doc.Consumes,
				Produces:         doc.Produces,
				RequestType:      doc.RequestType,
				RequestMetadata:  doc.RequestMetadata,
				ResponseTypes:    doc.ResponseTypes,
				ResponseExamples: doc.ResponseExamples,
				Security:         convertSecurityReqs(doc.Security),
			}
		}
		enriched[i] = build.EnrichedRoute{
			RouteInfo: build.RouteInfo{
				Method: ri.Method,
				Path:   ri.Path,
			},
			Doc: buildDoc,
		}
	}

	// Build spec
	spec, err := m.builder.Build(enriched)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build OpenAPI spec: %w", err)
	}

	// Copy extensions from Config to model Spec
	if len(m.cfg.Extensions) > 0 {
		spec.Extensions = make(map[string]any, len(m.cfg.Extensions))
		maps.Copy(spec.Extensions, m.cfg.Extensions)
	}

	// Project to target version
	var version export.Version
	switch m.cfg.Version {
	case Version31:
		version = export.V31
	case Version30:
		version = export.V30
	default:
		version = export.V30 // Default to 3.0.4
	}
	exportCfg := export.Config{
		Version:         version,
		StrictDownlevel: m.cfg.StrictDownlevel,
	}

	specJSON, warns, err := export.Project(spec, exportCfg, metaschema.OAS30, metaschema.OAS31)
	if err != nil {
		return nil, "", fmt.Errorf("failed to project OpenAPI spec: %w", err)
	}

	// Generate ETag from JSON bytes
	m.specJSON = specJSON
	m.etag = fmt.Sprintf(`"%x"`, sha256.Sum256(specJSON))
	m.lastWarnings = warns
	m.cacheInvalid = false

	return specJSON, m.etag, nil
}

// UIConfig returns the UI configuration for rendering Swagger UI.
//
// This is a convenience method for integration layers that need to
// serve the Swagger UI.
func (m *Manager) UIConfig() uiConfig {
	return m.cfg.ui
}

// SpecPath returns the configured spec path (e.g., "/openapi.json").
func (m *Manager) SpecPath() string {
	return m.cfg.SpecPath
}

// UIPath returns the configured UI path (e.g., "/docs").
func (m *Manager) UIPath() string {
	return m.cfg.UIPath
}

// ServeUI returns whether Swagger UI should be served.
func (m *Manager) ServeUI() bool {
	return m.cfg.ServeUI
}

// Warnings returns warnings from the last successful spec generation.
//
// Warnings are generated when 3.1-only features are used with a 3.0 target,
// or when features need to be down-leveled. Returns an empty slice if no
// warnings were generated or if the spec hasn't been generated yet.
//
// This method is safe for concurrent use.
//
// Example:
//
//	specJSON, _, err := manager.GenerateSpec()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	warnings := manager.Warnings()
//	for _, w := range warnings {
//	    log.Warnf("OpenAPI warning [%s] at %s: %s", w.Code, w.Path, w.Message)
//	}
func (m *Manager) Warnings() []export.Warning {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.lastWarnings == nil {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]export.Warning, len(m.lastWarnings))
	copy(result, m.lastWarnings)
	return result
}

// convertSecurityReqs converts openapi.SecurityReq to build.SecurityReq.
func convertSecurityReqs(reqs []SecurityReq) []build.SecurityReq {
	if len(reqs) == 0 {
		return nil
	}
	result := make([]build.SecurityReq, len(reqs))
	for i, r := range reqs {
		result[i] = build.SecurityReq{
			Scheme: r.Scheme,
			Scopes: r.Scopes,
		}
	}
	return result
}
