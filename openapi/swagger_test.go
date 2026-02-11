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

//go:build !integration

package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithSwaggerUI_appliesUIOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opt    UIOption
		assert func(t *testing.T, ui UIConfig)
	}{
		{
			name: "WithUIDeepLinking sets DeepLinking",
			opt:  WithUIDeepLinking(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.DeepLinking)
			},
		},
		{
			name: "WithUIDisplayOperationID sets DisplayOperationID",
			opt:  WithUIDisplayOperationID(true),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.True(t, ui.DisplayOperationID)
			},
		},
		{
			name: "WithUIExpansion sets DocExpansion",
			opt:  WithUIExpansion(DocExpansionFull),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, DocExpansionFull, ui.DocExpansion)
			},
		},
		{
			name: "WithUIModelsExpandDepth sets DefaultModelsExpandDepth",
			opt:  WithUIModelsExpandDepth(2),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, 2, ui.DefaultModelsExpandDepth)
			},
		},
		{
			name: "WithUIModelExpandDepth sets DefaultModelExpandDepth",
			opt:  WithUIModelExpandDepth(3),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, 3, ui.DefaultModelExpandDepth)
			},
		},
		{
			name: "WithUIDefaultModelRendering sets DefaultModelRendering",
			opt:  WithUIDefaultModelRendering(ModelRenderingModel),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, ModelRenderingModel, ui.DefaultModelRendering)
			},
		},
		{
			name: "WithUITryItOut sets TryItOutEnabled",
			opt:  WithUITryItOut(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.TryItOutEnabled)
			},
		},
		{
			name: "WithUIRequestSnippets sets RequestSnippetsEnabled and Languages",
			opt:  WithUIRequestSnippets(true, SnippetCurlBash, SnippetCurlPowerShell),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.True(t, ui.RequestSnippetsEnabled)
				assert.Equal(t, []RequestSnippetLanguage{SnippetCurlBash, SnippetCurlPowerShell}, ui.RequestSnippets.Languages)
			},
		},
		{
			name: "WithUIRequestSnippetsExpanded sets RequestSnippets DefaultExpanded",
			opt:  WithUIRequestSnippetsExpanded(true),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.True(t, ui.RequestSnippets.DefaultExpanded)
			},
		},
		{
			name: "WithUIDisplayRequestDuration sets DisplayRequestDuration",
			opt:  WithUIDisplayRequestDuration(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.DisplayRequestDuration)
			},
		},
		{
			name: "WithUIFilter sets Filter",
			opt:  WithUIFilter(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.Filter)
			},
		},
		{
			name: "WithUIMaxDisplayedTags sets MaxDisplayedTags",
			opt:  WithUIMaxDisplayedTags(10),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, 10, ui.MaxDisplayedTags)
			},
		},
		{
			name: "WithUIOperationsSorter sets OperationsSorter",
			opt:  WithUIOperationsSorter(OperationsSorterAlpha),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, OperationsSorterAlpha, ui.OperationsSorter)
			},
		},
		{
			name: "WithUITagsSorter sets TagsSorter",
			opt:  WithUITagsSorter(TagsSorterAlpha),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, TagsSorterAlpha, ui.TagsSorter)
			},
		},
		{
			name: "WithUISyntaxHighlight sets SyntaxHighlight Activated",
			opt:  WithUISyntaxHighlight(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.SyntaxHighlight.Activated)
			},
		},
		{
			name: "WithUISyntaxTheme sets SyntaxHighlight Theme",
			opt:  WithUISyntaxTheme(SyntaxThemeAgate),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, SyntaxThemeAgate, ui.SyntaxHighlight.Theme)
			},
		},
		{
			name: "WithUIValidator sets ValidatorURL",
			opt:  WithUIValidator(ValidatorLocal),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, ValidatorLocal, ui.ValidatorURL)
			},
		},
		{
			name: "WithUIPersistAuth sets PersistAuthorization",
			opt:  WithUIPersistAuth(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.PersistAuthorization)
			},
		},
		{
			name: "WithUIWithCredentials sets WithCredentials",
			opt:  WithUIWithCredentials(true),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.True(t, ui.WithCredentials)
			},
		},
		{
			name: "WithUISupportedMethods sets SupportedSubmitMethods",
			opt:  WithUISupportedMethods(MethodGet, MethodPost),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.Equal(t, []HTTPMethod{MethodGet, MethodPost}, ui.SupportedSubmitMethods)
			},
		},
		{
			name: "WithUIShowExtensions sets ShowExtensions",
			opt:  WithUIShowExtensions(true),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.True(t, ui.ShowExtensions)
			},
		},
		{
			name: "WithUIShowCommonExtensions sets ShowCommonExtensions",
			opt:  WithUIShowCommonExtensions(false),
			assert: func(t *testing.T, ui UIConfig) {
				t.Helper()
				assert.False(t, ui.ShowCommonExtensions)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := MustNew(
				WithTitle("API", "1.0.0"),
				WithSwaggerUI("/docs", tt.opt),
			)
			tt.assert(t, api.UI())
		})
	}
}

func TestWithSwaggerUI_multipleOptions(t *testing.T) {
	t.Parallel()

	api := MustNew(
		WithTitle("API", "1.0.0"),
		WithSwaggerUI("/docs",
			WithUIDeepLinking(true),
			WithUIExpansion(DocExpansionFull),
			WithUIModelsExpandDepth(2),
			WithUITryItOut(false),
			WithUIValidator(ValidatorLocal),
		),
	)

	ui := api.UI()
	assert.True(t, ui.DeepLinking)
	assert.Equal(t, DocExpansionFull, ui.DocExpansion)
	assert.Equal(t, 2, ui.DefaultModelsExpandDepth)
	assert.False(t, ui.TryItOutEnabled)
	assert.Equal(t, ValidatorLocal, ui.ValidatorURL)
}

func TestWithoutSwaggerUI_setsServeUIFalse(t *testing.T) {
	t.Parallel()

	api := MustNew(
		WithTitle("API", "1.0.0"),
		WithoutSwaggerUI(),
	)

	assert.False(t, api.ServeUI)
}
