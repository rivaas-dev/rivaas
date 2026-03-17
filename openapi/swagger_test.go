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
	"github.com/stretchr/testify/require"
)

func TestWithSwaggerUI_appliesUIOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opt    UIOption
		assert func(t *testing.T, snap *uiSnapshot)
	}{
		{
			name: "WithUIDeepLinking sets DeepLinking",
			opt:  WithUIDeepLinking(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.DeepLinking)
			},
		},
		{
			name: "WithUIDisplayOperationID sets DisplayOperationID",
			opt:  WithUIDisplayOperationID(true),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.True(t, snap.c.DisplayOperationID)
			},
		},
		{
			name: "WithUIExpansion sets DocExpansion",
			opt:  WithUIExpansion(DocExpansionFull),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, DocExpansionFull, snap.c.DocExpansion)
			},
		},
		{
			name: "WithUIModelsExpandDepth sets DefaultModelsExpandDepth",
			opt:  WithUIModelsExpandDepth(2),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, 2, snap.c.DefaultModelsExpandDepth)
			},
		},
		{
			name: "WithUIModelExpandDepth sets DefaultModelExpandDepth",
			opt:  WithUIModelExpandDepth(3),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, 3, snap.c.DefaultModelExpandDepth)
			},
		},
		{
			name: "WithUIDefaultModelRendering sets DefaultModelRendering",
			opt:  WithUIDefaultModelRendering(ModelRenderingModel),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, ModelRenderingModel, snap.c.DefaultModelRendering)
			},
		},
		{
			name: "WithUITryItOut sets TryItOutEnabled",
			opt:  WithUITryItOut(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.TryItOutEnabled)
			},
		},
		{
			name: "WithUIRequestSnippets sets RequestSnippetsEnabled and Languages",
			opt:  WithUIRequestSnippets(true, SnippetCurlBash, SnippetCurlPowerShell),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.True(t, snap.c.RequestSnippetsEnabled)
				assert.Equal(t, []RequestSnippetLanguage{SnippetCurlBash, SnippetCurlPowerShell}, snap.c.RequestSnippets.Languages)
			},
		},
		{
			name: "WithUIRequestSnippetsExpanded sets RequestSnippets DefaultExpanded",
			opt:  WithUIRequestSnippetsExpanded(true),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.True(t, snap.c.RequestSnippets.DefaultExpanded)
			},
		},
		{
			name: "WithUIDisplayRequestDuration sets DisplayRequestDuration",
			opt:  WithUIDisplayRequestDuration(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.DisplayRequestDuration)
			},
		},
		{
			name: "WithUIFilter sets Filter",
			opt:  WithUIFilter(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.Filter)
			},
		},
		{
			name: "WithUIMaxDisplayedTags sets MaxDisplayedTags",
			opt:  WithUIMaxDisplayedTags(10),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, 10, snap.c.MaxDisplayedTags)
			},
		},
		{
			name: "WithUIOperationsSorter sets OperationsSorter",
			opt:  WithUIOperationsSorter(OperationsSorterAlpha),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, OperationsSorterAlpha, snap.c.OperationsSorter)
			},
		},
		{
			name: "WithUITagsSorter sets TagsSorter",
			opt:  WithUITagsSorter(TagsSorterAlpha),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, TagsSorterAlpha, snap.c.TagsSorter)
			},
		},
		{
			name: "WithUISyntaxHighlight sets SyntaxHighlight Activated",
			opt:  WithUISyntaxHighlight(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.SyntaxHighlight.Activated)
			},
		},
		{
			name: "WithUISyntaxTheme sets SyntaxHighlight Theme",
			opt:  WithUISyntaxTheme(SyntaxThemeAgate),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, SyntaxThemeAgate, snap.c.SyntaxHighlight.Theme)
			},
		},
		{
			name: "WithUIValidator sets ValidatorURL",
			opt:  WithUIValidator(ValidatorLocal),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, ValidatorLocal, snap.c.ValidatorURL)
			},
		},
		{
			name: "WithUIPersistAuth sets PersistAuthorization",
			opt:  WithUIPersistAuth(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.PersistAuthorization)
			},
		},
		{
			name: "WithUIWithCredentials sets WithCredentials",
			opt:  WithUIWithCredentials(true),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.True(t, snap.c.WithCredentials)
			},
		},
		{
			name: "WithUISupportedMethods sets SupportedSubmitMethods",
			opt:  WithUISupportedMethods(MethodGet, MethodPost),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.Equal(t, []HTTPMethod{MethodGet, MethodPost}, snap.c.SupportedSubmitMethods)
			},
		},
		{
			name: "WithUIShowExtensions sets ShowExtensions",
			opt:  WithUIShowExtensions(true),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.True(t, snap.c.ShowExtensions)
			},
		},
		{
			name: "WithUIShowCommonExtensions sets ShowCommonExtensions",
			opt:  WithUIShowCommonExtensions(false),
			assert: func(t *testing.T, snap *uiSnapshot) {
				t.Helper()
				assert.False(t, snap.c.ShowCommonExtensions)
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
			snap, ok := api.UI().(*uiSnapshot)
			require.True(t, ok)
			tt.assert(t, snap)
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

	snap, ok := api.UI().(*uiSnapshot)
	require.True(t, ok)
	assert.True(t, snap.c.DeepLinking)
	assert.Equal(t, DocExpansionFull, snap.c.DocExpansion)
	assert.Equal(t, 2, snap.c.DefaultModelsExpandDepth)
	assert.False(t, snap.c.TryItOutEnabled)
	assert.Equal(t, ValidatorLocal, snap.c.ValidatorURL)
}

func TestWithoutSwaggerUI_setsServeUIFalse(t *testing.T) {
	t.Parallel()

	api := MustNew(
		WithTitle("API", "1.0.0"),
		WithoutSwaggerUI(),
	)

	assert.False(t, api.ServeUI())
}
