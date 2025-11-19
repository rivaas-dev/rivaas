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
	"encoding/json"
	"errors"
	"fmt"
)

// Static errors for UI validation
var (
	errInvalidDocExpansion          = errors.New("invalid docExpansion")
	errInvalidDefaultModelRendering = errors.New("invalid defaultModelRendering")
	errInvalidOperationsSorter      = errors.New("invalid operationsSorter")
	errInvalidTagsSorter            = errors.New("invalid tagsSorter")
	errInvalidSyntaxTheme           = errors.New("invalid syntax theme")
	errInvalidRequestSnippetLang    = errors.New("invalid request snippet language")
	errInvalidHTTPMethod            = errors.New("invalid HTTP method")
	errInvalidModelsExpandDepth     = errors.New("defaultModelsExpandDepth must be >= -1")
	errInvalidModelExpandDepth      = errors.New("defaultModelExpandDepth must be >= -1")
	errInvalidMaxDisplayedTags      = errors.New("maxDisplayedTags must be >= 0")
)

// DocExpansionMode controls the default expansion behavior of operations and tags in Swagger UI.
//
// This setting determines how much of the API documentation is expanded by default
// when the Swagger UI page loads.
type DocExpansionMode string

// ModelRenderingMode controls how schema models are initially displayed in Swagger UI.
//
// Models can be shown as example values or as structured schema definitions.
type ModelRenderingMode string

// OperationsSorterMode controls how operations are sorted within each tag in Swagger UI.
//
// Operations can be sorted alphabetically by path, by HTTP method, or left in server order.
type OperationsSorterMode string

// TagsSorterMode controls how tags are sorted in Swagger UI.
//
// Tags can be sorted alphabetically or left in server order.
type TagsSorterMode string

// SyntaxTheme defines the syntax highlighting theme used for code examples in Swagger UI.
//
// Different themes provide different color schemes for code snippets and examples.
type SyntaxTheme string

// RequestSnippetLanguage defines the language for generated request code snippets.
//
// These snippets help users understand how to make API calls using different tools.
type RequestSnippetLanguage string

// HTTPMethod represents an HTTP method that can be used in "Try it out" functionality.
//
// This is used to configure which HTTP methods are supported for interactive API testing.
type HTTPMethod string

// DocExpansion constants control default expansion of operations and tags.
const (
	// DocExpansionList expands only tags by default, keeping operations collapsed.
	DocExpansionList DocExpansionMode = "list"

	// DocExpansionFull expands both tags and operations by default.
	DocExpansionFull DocExpansionMode = "full"

	// DocExpansionNone collapses everything by default, requiring manual expansion.
	DocExpansionNone DocExpansionMode = "none"
)

// ModelRendering constants control initial model display.
const (
	// ModelRenderingExample shows example values for schema models.
	ModelRenderingExample ModelRenderingMode = "example"

	// ModelRenderingModel shows the structured schema definition.
	ModelRenderingModel ModelRenderingMode = "model"
)

// OperationsSorter constants control operation sorting within tags.
const (
	// OperationsSorterAlpha sorts operations alphabetically by path.
	OperationsSorterAlpha OperationsSorterMode = "alpha"

	// OperationsSorterMethod sorts operations by HTTP method.
	OperationsSorterMethod OperationsSorterMode = "method"

	// OperationsSorterNone uses server order without sorting.
	OperationsSorterNone OperationsSorterMode = ""
)

// TagsSorter constants control tag sorting.
const (
	// TagsSorterAlpha sorts tags alphabetically.
	TagsSorterAlpha TagsSorterMode = "alpha"

	// TagsSorterNone uses server order without sorting.
	TagsSorterNone TagsSorterMode = ""
)

// Syntax highlighting theme constants.
const (
	// SyntaxThemeAgate provides a dark theme with blue accents.
	SyntaxThemeAgate SyntaxTheme = "agate"

	// SyntaxThemeArta provides a dark theme with orange accents.
	SyntaxThemeArta SyntaxTheme = "arta"

	// SyntaxThemeMonokai provides a dark theme with vibrant colors.
	SyntaxThemeMonokai SyntaxTheme = "monokai"

	// SyntaxThemeNord provides a dark theme with cool blue tones.
	SyntaxThemeNord SyntaxTheme = "nord"

	// SyntaxThemeObsidian provides a dark theme with green accents.
	SyntaxThemeObsidian SyntaxTheme = "obsidian"

	// SyntaxThemeTomorrowNight provides a dark theme with muted colors.
	SyntaxThemeTomorrowNight SyntaxTheme = "tomorrow-night"

	// SyntaxThemeIdea provides a light theme similar to IntelliJ IDEA.
	SyntaxThemeIdea SyntaxTheme = "idea"
)

// Request snippet language constants.
const (
	// SnippetCurlBash generates curl commands for bash/sh shells.
	SnippetCurlBash RequestSnippetLanguage = "curl_bash"

	// SnippetCurlPowerShell generates curl commands for PowerShell.
	SnippetCurlPowerShell RequestSnippetLanguage = "curl_powershell"

	// SnippetCurlCmd generates curl commands for Windows CMD.
	SnippetCurlCmd RequestSnippetLanguage = "curl_cmd"
)

// HTTP method constants for "Try it out" configuration.
const (
	// MethodGet represents the HTTP GET method.
	MethodGet HTTPMethod = "get"

	// MethodPost represents the HTTP POST method.
	MethodPost HTTPMethod = "post"

	// MethodPut represents the HTTP PUT method.
	MethodPut HTTPMethod = "put"

	// MethodDelete represents the HTTP DELETE method.
	MethodDelete HTTPMethod = "delete"

	// MethodPatch represents the HTTP PATCH method.
	MethodPatch HTTPMethod = "patch"

	// MethodHead represents the HTTP HEAD method.
	MethodHead HTTPMethod = "head"

	// MethodOptions represents the HTTP OPTIONS method.
	MethodOptions HTTPMethod = "options"

	// MethodTrace represents the HTTP TRACE method.
	MethodTrace HTTPMethod = "trace"
)

// uiConfig configures Swagger UI behavior and appearance.
//
// This type is used internally to build the JavaScript configuration object
// that controls how Swagger UI renders and behaves. It is not exported as
// users configure the UI through the Config type's options.
type uiConfig struct {
	// Navigation & Deep Linking
	DeepLinking        bool
	DisplayOperationID bool

	// Display & Expansion
	DocExpansion             DocExpansionMode
	DefaultModelsExpandDepth int
	DefaultModelExpandDepth  int
	DefaultModelRendering    ModelRenderingMode

	// Interaction
	TryItOutEnabled        bool
	RequestSnippetsEnabled bool
	DisplayRequestDuration bool

	// Filtering & Sorting
	Filter           bool
	MaxDisplayedTags int
	OperationsSorter OperationsSorterMode
	TagsSorter       TagsSorterMode

	// Syntax Highlighting
	SyntaxHighlight SyntaxHighlightConfig

	// Network & Security
	ValidatorURL           string
	PersistAuthorization   bool
	WithCredentials        bool
	SupportedSubmitMethods []HTTPMethod

	// Advanced
	ShowExtensions       bool
	ShowCommonExtensions bool

	// Request Snippets Configuration
	RequestSnippets RequestSnippetsConfig
}

// SyntaxHighlightConfig controls syntax highlighting appearance in Swagger UI.
type SyntaxHighlightConfig struct {
	// Activated enables or disables syntax highlighting.
	Activated bool

	// Theme specifies the color scheme for syntax highlighting.
	Theme SyntaxTheme
}

// RequestSnippetsConfig controls code snippet generation for API requests.
type RequestSnippetsConfig struct {
	// Languages specifies which snippet languages to generate.
	// If empty, all available languages are included.
	Languages []RequestSnippetLanguage

	// DefaultExpanded determines if snippet sections are expanded by default.
	DefaultExpanded bool
}

// defaultUIConfig returns sensible defaults for Swagger UI configuration.
//
// These defaults provide a good balance of functionality and usability
// for most API documentation needs.
func defaultUIConfig() uiConfig {
	return uiConfig{
		DeepLinking:              true,
		DisplayOperationID:       false,
		DocExpansion:             DocExpansionList,
		DefaultModelsExpandDepth: 1,
		DefaultModelExpandDepth:  1,
		DefaultModelRendering:    ModelRenderingExample,
		TryItOutEnabled:          true,
		RequestSnippetsEnabled:   true,
		DisplayRequestDuration:   true,
		Filter:                   true,
		MaxDisplayedTags:         0,
		OperationsSorter:         OperationsSorterNone,
		TagsSorter:               TagsSorterNone,
		SyntaxHighlight: SyntaxHighlightConfig{
			Activated: true,
			Theme:     SyntaxThemeMonokai,
		},
		ValidatorURL:         "",
		PersistAuthorization: true,
		WithCredentials:      false,
		SupportedSubmitMethods: []HTTPMethod{
			MethodGet, MethodPut, MethodPost, MethodDelete,
			MethodOptions, MethodHead, MethodPatch,
		},
		ShowExtensions:       false,
		ShowCommonExtensions: true,
		RequestSnippets: RequestSnippetsConfig{
			Languages:       nil, // All languages
			DefaultExpanded: true,
		},
	}
}

// Validate checks if the uiConfig is valid and returns an error if any
// configuration values are invalid.
//
// It validates:
//   - DocExpansion mode (must be list, full, or none)
//   - ModelRendering mode (must be example or model)
//   - OperationsSorter mode (must be alpha, method, or empty)
//   - TagsSorter mode (must be alpha or empty)
//   - SyntaxTheme (must be a valid theme name)
//   - RequestSnippetLanguage values (must be valid language identifiers)
//   - HTTPMethod values (must be valid HTTP methods)
//   - Numeric ranges (depths must be >= -1, maxDisplayedTags must be >= 0)
func (c *uiConfig) Validate() error {
	// Validate DocExpansion
	if c.DocExpansion != "" {
		switch c.DocExpansion {
		case DocExpansionList, DocExpansionFull, DocExpansionNone:
			// Valid
		default:
			return fmt.Errorf("%w: %q (must be list, full, or none)", errInvalidDocExpansion, c.DocExpansion)
		}
	}

	// Validate DefaultModelRendering
	if c.DefaultModelRendering != "" {
		switch c.DefaultModelRendering {
		case ModelRenderingExample, ModelRenderingModel:
			// Valid
		default:
			return fmt.Errorf("%w: %q (must be example or model)", errInvalidDefaultModelRendering, c.DefaultModelRendering)
		}
	}

	// Validate OperationsSorter
	if c.OperationsSorter != "" && c.OperationsSorter != OperationsSorterAlpha && c.OperationsSorter != OperationsSorterMethod {
		return fmt.Errorf("%w: %q (must be alpha, method, or empty)", errInvalidOperationsSorter, c.OperationsSorter)
	}

	// Validate TagsSorter
	if c.TagsSorter != "" && c.TagsSorter != TagsSorterAlpha {
		return fmt.Errorf("%w: %q (must be alpha or empty)", errInvalidTagsSorter, c.TagsSorter)
	}

	// Validate SyntaxTheme
	if c.SyntaxHighlight.Activated && c.SyntaxHighlight.Theme != "" {
		validThemes := map[SyntaxTheme]bool{
			SyntaxThemeAgate:         true,
			SyntaxThemeArta:          true,
			SyntaxThemeMonokai:       true,
			SyntaxThemeNord:          true,
			SyntaxThemeObsidian:      true,
			SyntaxThemeTomorrowNight: true,
			SyntaxThemeIdea:          true,
		}
		if !validThemes[c.SyntaxHighlight.Theme] {
			return fmt.Errorf("%w: %q", errInvalidSyntaxTheme, c.SyntaxHighlight.Theme)
		}
	}

	// Validate RequestSnippet languages
	if len(c.RequestSnippets.Languages) > 0 {
		validLangs := map[RequestSnippetLanguage]bool{
			SnippetCurlBash:       true,
			SnippetCurlPowerShell: true,
			SnippetCurlCmd:        true,
		}
		for _, lang := range c.RequestSnippets.Languages {
			if !validLangs[lang] {
				return fmt.Errorf("%w: %q", errInvalidRequestSnippetLang, lang)
			}
		}
	}

	// Validate HTTP methods
	if len(c.SupportedSubmitMethods) > 0 {
		validMethods := map[HTTPMethod]bool{
			MethodGet:     true,
			MethodPost:    true,
			MethodPut:     true,
			MethodDelete:  true,
			MethodPatch:   true,
			MethodHead:    true,
			MethodOptions: true,
			MethodTrace:   true,
		}
		for _, method := range c.SupportedSubmitMethods {
			if !validMethods[method] {
				return fmt.Errorf("%w: %q", errInvalidHTTPMethod, method)
			}
		}
	}

	// Validate ranges
	if c.DefaultModelsExpandDepth < -1 {
		return fmt.Errorf("%w, got %d", errInvalidModelsExpandDepth, c.DefaultModelsExpandDepth)
	}
	if c.DefaultModelExpandDepth < -1 {
		return fmt.Errorf("%w, got %d", errInvalidModelExpandDepth, c.DefaultModelExpandDepth)
	}
	if c.MaxDisplayedTags < 0 {
		return fmt.Errorf("%w, got %d", errInvalidMaxDisplayedTags, c.MaxDisplayedTags)
	}

	return nil
}

// ToConfigMap converts uiConfig to a map suitable for JSON serialization.
//
// The returned map contains all configuration options in the format expected
// by Swagger UI's JavaScript initialization. The specURL parameter specifies
// the URL where the OpenAPI specification JSON can be fetched.
//
// This method is used internally to generate the JavaScript configuration
// object that is embedded in the Swagger UI HTML page.
func (c *uiConfig) ToConfigMap(specURL string) map[string]any {
	m := map[string]any{
		"url":                      specURL,
		"dom_id":                   "#swagger-ui",
		"deepLinking":              c.DeepLinking,
		"displayOperationId":       c.DisplayOperationID,
		"docExpansion":             string(c.DocExpansion),
		"defaultModelsExpandDepth": c.DefaultModelsExpandDepth,
		"defaultModelExpandDepth":  c.DefaultModelExpandDepth,
		"defaultModelRendering":    string(c.DefaultModelRendering),
		"tryItOutEnabled":          c.TryItOutEnabled,
		"requestSnippetsEnabled":   c.RequestSnippetsEnabled,
		"displayRequestDuration":   c.DisplayRequestDuration,
		"filter":                   c.Filter,
		"persistAuthorization":     c.PersistAuthorization,
		"withCredentials":          c.WithCredentials,
		"showExtensions":           c.ShowExtensions,
		"showCommonExtensions":     c.ShowCommonExtensions,
		"syntaxHighlight": map[string]any{
			"activated": c.SyntaxHighlight.Activated,
			"theme":     string(c.SyntaxHighlight.Theme),
		},
	}

	if c.MaxDisplayedTags > 0 {
		m["maxDisplayedTags"] = c.MaxDisplayedTags
	}

	if c.OperationsSorter != "" {
		m["operationsSorter"] = string(c.OperationsSorter)
	}

	if c.TagsSorter != "" {
		m["tagsSorter"] = string(c.TagsSorter)
	}

	if c.ValidatorURL == "" || c.ValidatorURL == "none" {
		m["validatorUrl"] = nil
	} else {
		m["validatorUrl"] = c.ValidatorURL
	}

	if len(c.SupportedSubmitMethods) > 0 {
		methods := make([]string, len(c.SupportedSubmitMethods))
		for i, method := range c.SupportedSubmitMethods {
			methods[i] = string(method)
		}
		m["supportedSubmitMethods"] = methods
	}

	if c.RequestSnippetsEnabled {
		snippets := map[string]any{
			"defaultExpanded": c.RequestSnippets.DefaultExpanded,
		}
		if len(c.RequestSnippets.Languages) > 0 {
			langs := make([]string, len(c.RequestSnippets.Languages))
			for i, lang := range c.RequestSnippets.Languages {
				langs[i] = string(lang)
			}
			snippets["languages"] = langs
		}
		m["requestSnippets"] = snippets
	}

	return m
}

// ToJSON converts uiConfig to a formatted JSON string for embedding in HTML.
//
// The returned JSON string is indented for readability and can be directly
// embedded in the Swagger UI HTML template. The specURL parameter specifies
// the URL where the OpenAPI specification JSON can be fetched.
//
// Returns an error if JSON serialization fails.
func (c *uiConfig) ToJSON(specURL string) (string, error) {
	configMap := c.ToConfigMap(specURL)
	bytes, err := json.MarshalIndent(configMap, "\t\t", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
