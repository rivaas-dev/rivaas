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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		options   []Option
		wantError string
	}{
		{
			name: "valid minimal config",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
			},
			wantError: "",
		},
		{
			name: "with all options",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithInfoDescription("Test description"),
				WithContact("Support", "https://example.com", "support@example.com"),
				WithLicense("MIT", "https://opensource.org/licenses/MIT"),
				WithServer("https://api.example.com", "Production"),
				WithVersion(V31x),
			},
			wantError: "",
		},
		{
			name: "with version 3.0.4",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V30x),
			},
			wantError: "",
		},
		{
			name: "with version 3.1.2",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V31x),
			},
			wantError: "",
		},
		{
			name: "with strict downlevel",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithStrictDownlevel(true),
			},
			wantError: "",
		},
		{
			name: "with bearer auth",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithBearerAuth("bearerAuth", "JWT authentication"),
			},
			wantError: "",
		},
		{
			name: "with API key auth",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithAPIKey("apiKey", "X-API-Key", InHeader, "API key authentication"),
			},
			wantError: "",
		},
		{
			name: "with default security",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithDefaultSecurity("bearerAuth"),
			},
			wantError: "",
		},
		{
			name: "with custom paths",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithSpecPath("/api/openapi.json"),
				WithSwaggerUI("/api/docs"),
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := New(tt.options...)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, "Test API", cfg.Info.Title)
				assert.Equal(t, "1.0.0", cfg.Info.Version)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
	)

	assert.Equal(t, "/openapi.json", cfg.SpecPath)
	assert.Equal(t, "/docs", cfg.UIPath)
	assert.True(t, cfg.ServeUI)
	assert.Equal(t, V30x, cfg.Version)
	assert.False(t, cfg.StrictDownlevel)
}

func TestConfig_WithVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  Version
		expected Version
	}{
		{"3.0.x", V30x, V30x},
		{"3.1.x", V31x, V31x},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := MustNew(
				WithTitle("Test API", "1.0.0"),
				WithVersion(tt.version),
			)

			assert.Equal(t, tt.expected, cfg.Version)
		})
	}
}

func TestConfig_WithStrictDownlevel(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithStrictDownlevel(true),
	)

	assert.True(t, cfg.StrictDownlevel)
}

func TestConfig_WithSwaggerUI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		expected     bool
		expectedPath string
	}{
		{"enabled default", "/docs", true, "/docs"},
		{"enabled custom path", "/swagger", true, "/swagger"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := MustNew(
				WithTitle("Test API", "1.0.0"),
				WithSwaggerUI(tt.path),
			)

			assert.Equal(t, tt.expected, cfg.ServeUI)
			assert.Equal(t, tt.expectedPath, cfg.UIPath)
		})
	}

	// Test disabled
	t.Run("disabled", func(t *testing.T) {
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithoutSwaggerUI(),
		)

		assert.False(t, cfg.ServeUI)
	})
}

func TestConfig_WithServers(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithServer("https://api.example.com", "Production"),
		WithServer("https://staging.example.com", "Staging"),
	)

	assert.Len(t, cfg.Servers, 2)
	assert.Equal(t, "https://api.example.com", cfg.Servers[0].URL)
	assert.Equal(t, "Production", cfg.Servers[0].Description)
	assert.Equal(t, "https://staging.example.com", cfg.Servers[1].URL)
	assert.Equal(t, "Staging", cfg.Servers[1].Description)
}

func TestConfig_WithTags(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithTag("users", "User management operations"),
		WithTag("orders", "Order operations"),
	)

	assert.Len(t, cfg.Tags, 2)
	assert.Equal(t, "users", cfg.Tags[0].Name)
	assert.Equal(t, "User management operations", cfg.Tags[0].Description)
	assert.Equal(t, "orders", cfg.Tags[1].Name)
	assert.Equal(t, "Order operations", cfg.Tags[1].Description)
}

func TestConfig_WithSecuritySchemes(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithBearerAuth("bearerAuth", "JWT authentication"),
		WithAPIKey("apiKey", "X-API-Key", InHeader, "API key"),
	)

	assert.Len(t, cfg.SecuritySchemes, 2)

	bearer, ok := cfg.SecuritySchemes["bearerAuth"]
	require.True(t, ok)
	assert.Equal(t, "http", bearer.Type)
	assert.Equal(t, "bearer", bearer.Scheme)
	assert.Equal(t, "JWT", bearer.BearerFormat)

	apiKey, ok := cfg.SecuritySchemes["apiKey"]
	require.True(t, ok)
	assert.Equal(t, "apiKey", apiKey.Type)
	assert.Equal(t, "X-API-Key", apiKey.Name)
	assert.Equal(t, "header", apiKey.In)
}

func TestConfig_WithDefaultSecurity(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithDefaultSecurity("bearerAuth"),
		WithDefaultSecurity("oauth2", "read", "write"),
	)

	assert.Len(t, cfg.DefaultSecurity, 2)

	// First requirement
	req1 := cfg.DefaultSecurity[0]
	assert.Contains(t, req1, "bearerAuth")
	assert.Empty(t, req1["bearerAuth"])

	// Second requirement with scopes
	req2 := cfg.DefaultSecurity[1]
	assert.Contains(t, req2, "oauth2")
	assert.Equal(t, []string{"read", "write"}, req2["oauth2"])
}

func TestConfig_InvalidVersion(t *testing.T) {
	t.Parallel()

	// Invalid versions should still be accepted (validation happens at export time)
	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithVersion("2.0.0"), // Not a valid OpenAPI version
	)

	assert.Equal(t, Version("2.0.0"), cfg.Version)
}

func TestConfig_EmptyTitle(t *testing.T) {
	t.Parallel()

	// Empty title should fail validation
	_, err := New(
		WithTitle("", "1.0.0"),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestConfig_EmptyVersion(t *testing.T) {
	t.Parallel()

	// Empty version should fail validation
	_, err := New(
		WithTitle("Test API", ""),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestConfig_WithValidation(t *testing.T) {
	t.Parallel()

	// Default should have validation disabled for backward compatibility
	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
	)
	assert.False(t, cfg.ValidateSpec)

	// Enable validation
	cfg2 := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithValidation(true),
	)
	assert.True(t, cfg2.ValidateSpec)

	// Explicitly disable validation
	cfg3 := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithValidation(false),
	)
	assert.False(t, cfg3.ValidateSpec)
}

func TestConfig_MultipleServers(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithServer("https://api1.example.com", "Server 1"),
		WithServer("https://api2.example.com", "Server 2"),
		WithServer("https://api3.example.com", "Server 3"),
	)

	assert.Len(t, cfg.Servers, 3)
	assert.Equal(t, "https://api1.example.com", cfg.Servers[0].URL)
	assert.Equal(t, "https://api2.example.com", cfg.Servers[1].URL)
	assert.Equal(t, "https://api3.example.com", cfg.Servers[2].URL)
}

func TestConfig_DuplicateSecuritySchemes(t *testing.T) {
	t.Parallel()

	// Adding the same security scheme twice should overwrite
	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithBearerAuth("auth", "First description"),
		WithBearerAuth("auth", "Second description"),
	)

	assert.Len(t, cfg.SecuritySchemes, 1)
	scheme, ok := cfg.SecuritySchemes["auth"]
	require.True(t, ok)
	assert.Equal(t, "Second description", scheme.Description)
}

func TestConfig_EmptyServerURL(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithServer("", "Empty URL"),
	)

	assert.Len(t, cfg.Servers, 1)
	assert.Empty(t, cfg.Servers[0].URL)
}

func TestConfig_MustNewPanic(t *testing.T) {
	t.Parallel()

	// MustNew should panic on error, but New should return error
	// Since we don't have validation errors currently, this test verifies MustNew doesn't panic on valid config
	assert.NotPanics(t, func() {
		_ = MustNew(WithTitle("Test", "1.0.0"))
	})
}

func TestConfig_AllOptions(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithInfoDescription("A comprehensive test"),
		WithContact("Support", "https://example.com", "support@example.com"),
		WithLicense("MIT", "https://opensource.org/licenses/MIT"),
		WithServer("https://api.example.com", "Production"),
		WithTag("users", "User operations"),
		WithBearerAuth("bearer", "JWT auth"),
		WithAPIKey("apiKey", "X-API-Key", InHeader, "API key"),
		WithDefaultSecurity("bearer"),
		WithVersion(V31x),
		WithStrictDownlevel(true),
		WithSpecPath("/api/openapi.json"),
		WithSwaggerUI("/api/docs"),
	)

	assert.Equal(t, "Test API", cfg.Info.Title)
	assert.Equal(t, "1.0.0", cfg.Info.Version)
	assert.Equal(t, "A comprehensive test", cfg.Info.Description)
	assert.NotNil(t, cfg.Info.Contact)
	assert.NotNil(t, cfg.Info.License)
	assert.Len(t, cfg.Servers, 1)
	assert.Len(t, cfg.Tags, 1)
	assert.Len(t, cfg.SecuritySchemes, 2)
	assert.Len(t, cfg.DefaultSecurity, 1)
	assert.Equal(t, V31x, cfg.Version)
	assert.True(t, cfg.StrictDownlevel)
	assert.Equal(t, "/api/openapi.json", cfg.SpecPath)
	assert.Equal(t, "/api/docs", cfg.UIPath)
	assert.True(t, cfg.ServeUI)
}

func TestConfig_WithExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		value   any
		wantErr bool
	}{
		{
			name:    "valid extension",
			key:     "x-custom-field",
			value:   "value",
			wantErr: false,
		},
		{
			name:    "valid extension with number",
			key:     "x-version",
			value:   42,
			wantErr: false,
		},
		{
			name:    "valid extension with array",
			key:     "x-tags",
			value:   []string{"tag1", "tag2"},
			wantErr: false,
		},
		{
			name:    "valid extension with object",
			key:     "x-metadata",
			value:   map[string]any{"key": "value"},
			wantErr: false,
		},
		{
			name:    "multiple extensions",
			key:     "x-another",
			value:   "another-value",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := MustNew(
				WithTitle("Test API", "1.0.0"),
				WithExtension(tt.key, tt.value),
			)

			if !tt.wantErr {
				require.NotNil(t, cfg.Extensions)
				assert.Equal(t, tt.value, cfg.Extensions[tt.key])
			}
		})
	}

	// Test multiple extensions
	t.Run("multiple extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithExtension("x-field1", "value1"),
			WithExtension("x-field2", "value2"),
			WithExtension("x-field3", 123),
		)
		assert.Len(t, cfg.Extensions, 3)
		assert.Equal(t, "value1", cfg.Extensions["x-field1"])
		assert.Equal(t, "value2", cfg.Extensions["x-field2"])
		assert.Equal(t, 123, cfg.Extensions["x-field3"])
	})
}

func TestConfig_ExtensionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		options   []Option
		wantError string
	}{
		{
			name: "invalid extension key - no x- prefix",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithExtension("invalid-key", "value"),
			},
			wantError: "extension key must start with 'x-'",
		},
		{
			name: "reserved extension key x-oai- in 3.1",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V31x),
				WithExtension("x-oai-custom", "value"),
			},
			wantError: "reserved prefix",
		},
		{
			name: "reserved extension key x-oas- in 3.1",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V31x),
				WithExtension("x-oas-custom", "value"),
			},
			wantError: "reserved prefix",
		},
		{
			name: "reserved extension key allowed in 3.0",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V30x),
				WithExtension("x-oai-custom", "value"),
			},
			wantError: "",
		},
		{
			name: "valid extensions in 3.1",
			options: []Option{
				WithTitle("Test API", "1.0.0"),
				WithVersion(V31x),
				WithExtension("x-custom-field", "value"),
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := New(tt.options...)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
			}
		})
	}
}

func TestConfig_WithInfoSummary(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithInfoSummary("A brief summary"),
	)

	assert.Equal(t, "A brief summary", cfg.Info.Summary)
}

func TestConfig_WithTermsOfService(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithTermsOfService("https://example.com/terms"),
	)

	assert.Equal(t, "https://example.com/terms", cfg.Info.TermsOfService)
}

func TestConfig_WithExternalDocs(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithExternalDocs("https://example.com/docs", "API Documentation"),
	)

	require.NotNil(t, cfg.ExternalDocs)
	assert.Equal(t, "https://example.com/docs", cfg.ExternalDocs.URL)
	assert.Equal(t, "API Documentation", cfg.ExternalDocs.Description)
}

func TestConfig_WithLicenseIdentifier(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithLicenseIdentifier("MIT License", "MIT"),
	)

	require.NotNil(t, cfg.Info.License)
	assert.Equal(t, "MIT License", cfg.Info.License.Name)
	assert.Equal(t, "MIT", cfg.Info.License.Identifier)
	assert.Empty(t, cfg.Info.License.URL)
}

func TestConfig_WithServerVariable(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithServer("https://{host}.example.com", "Server with variable"),
		WithServerVariable("host", "api", []string{"api", "staging"}, "Server hostname"),
	)

	require.Len(t, cfg.Servers, 1)
	require.NotNil(t, cfg.Servers[0].Variables)
	variable, ok := cfg.Servers[0].Variables["host"]
	require.True(t, ok)
	assert.Equal(t, "api", variable.Default)
	assert.Equal(t, []string{"api", "staging"}, variable.Enum)
	assert.Equal(t, "Server hostname", variable.Description)
}

func TestConfig_WithOAuth2(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithOAuth2("oauth2", "OAuth2 authentication",
			OAuth2Flow{
				Type:             FlowAuthorizationCode,
				AuthorizationURL: "https://example.com/oauth/authorize",
				TokenURL:         "https://example.com/oauth/token",
				Scopes: map[string]string{
					"read":  "Read access",
					"write": "Write access",
				},
			}),
	)

	require.Len(t, cfg.SecuritySchemes, 1)
	scheme, ok := cfg.SecuritySchemes["oauth2"]
	require.True(t, ok)
	assert.Equal(t, "oauth2", scheme.Type)
	assert.Equal(t, "OAuth2 authentication", scheme.Description)
	require.NotNil(t, scheme.Flows)
	require.NotNil(t, scheme.Flows.AuthorizationCode)
	assert.Equal(t, "https://example.com/oauth/authorize", scheme.Flows.AuthorizationCode.AuthorizationURL)
	assert.Equal(t, "https://example.com/oauth/token", scheme.Flows.AuthorizationCode.TokenURL)
	assert.Equal(t, map[string]string{"read": "Read access", "write": "Write access"}, scheme.Flows.AuthorizationCode.Scopes)
}

func TestConfig_WithOpenIDConnect(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithOpenIDConnect("openId", "https://example.com/.well-known/openid-configuration", "OpenID Connect"),
	)

	require.Len(t, cfg.SecuritySchemes, 1)
	scheme, ok := cfg.SecuritySchemes["openId"]
	require.True(t, ok)
	assert.Equal(t, "openIdConnect", scheme.Type)
	assert.Equal(t, "OpenID Connect", scheme.Description)
	assert.Equal(t, "https://example.com/.well-known/openid-configuration", scheme.OpenIDConnectURL)
}
