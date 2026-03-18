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

// Public DTOs for API getters. These types are independent of internal/model
// so the public API does not leak internal representation.

// Info holds API metadata (title, version, description, contact, license).
// Returned by [API.Info]. Do not modify.
type Info struct {
	Title          string
	Summary        string
	Description    string
	TermsOfService string
	Version        string
	Contact        *Contact
	License        *License
	Extensions     map[string]any
}

// Contact holds contact information for the API.
type Contact struct {
	Name  string
	URL   string
	Email string
}

// License holds license information for the API.
type License struct {
	Name       string // License name
	Identifier string // SPDX identifier (3.1+), mutually exclusive with URL
	URL        string // License URL (3.0), mutually exclusive with Identifier
}

// Server holds a server URL and optional description and variables.
// Returned by [API.Servers]. Do not modify.
type Server struct {
	URL         string
	Description string
	Variables   map[string]*ServerVariable
}

// ServerVariable holds a variable for server URL template substitution.
type ServerVariable struct {
	Enum        []string
	Default     string
	Description string
}

// Tag holds tag metadata. Returned by [API.Tags]. Do not modify.
type Tag struct {
	Name        string
	Description string
}

// SecurityScheme holds a security scheme definition.
// Returned by [API.SecuritySchemes]. Do not modify.
// For oauth2 schemes, Flows may be set. For apiKey: Name and In. For http: Scheme and BearerFormat. For openIdConnect: OpenIDConnectURL.
type SecurityScheme struct {
	Type             string
	Description      string
	Name             string
	In               string
	Scheme           string
	BearerFormat     string
	OpenIDConnectURL string
	Flows            *OAuth2Flows
}

// OAuth2Flows holds OAuth2 flow configuration for display.
type OAuth2Flows struct {
	AuthorizationCode *OAuth2FlowInfo
	Implicit          *OAuth2FlowInfo
	Password          *OAuth2FlowInfo
	ClientCredentials *OAuth2FlowInfo
}

// OAuth2FlowInfo holds a single OAuth2 flow's URLs and scopes.
type OAuth2FlowInfo struct {
	AuthorizationURL string
	TokenURL         string
	RefreshURL       string
	Scopes           map[string]string
}

// SecurityRequirement holds required security schemes and optional scopes (scheme name -> scopes).
// Returned by [API.DefaultSecurity]. Use [RequireSecurity] to build requirements for [WithDefaultSecurity].
type SecurityRequirement map[string][]string

// ExternalDocs holds external documentation link. Returned by [API.ExternalDocs]. Do not modify.
type ExternalDocs struct {
	URL         string
	Description string
}
