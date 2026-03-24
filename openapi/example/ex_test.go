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

package example

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		exName      string
		value       any
		opts        []Option
		wantName    string
		wantValue   any
		wantSummary string
		wantDesc    string
	}{
		{
			name:      "basic example",
			exName:    "success",
			value:     map[string]any{"id": 123},
			wantName:  "success",
			wantValue: map[string]any{"id": 123},
		},
		{
			name:        "with summary",
			exName:      "test",
			value:       "value",
			opts:        []Option{WithSummary("Test summary")},
			wantName:    "test",
			wantValue:   "value",
			wantSummary: "Test summary",
		},
		{
			name:      "with description",
			exName:    "test",
			value:     42,
			opts:      []Option{WithDescription("Test description")},
			wantName:  "test",
			wantValue: 42,
			wantDesc:  "Test description",
		},
		{
			name:   "with all options",
			exName: "complete",
			value:  struct{ ID int }{ID: 1},
			opts: []Option{
				WithSummary("Summary"),
				WithDescription("Description"),
			},
			wantName:    "complete",
			wantValue:   struct{ ID int }{ID: 1},
			wantSummary: "Summary",
			wantDesc:    "Description",
		},
		{
			name:      "nil value",
			exName:    "empty",
			value:     nil,
			wantName:  "empty",
			wantValue: nil,
		},
		{
			name:      "empty name",
			exName:    "",
			value:     "value",
			wantName:  "",
			wantValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ex, err := New(tt.exName, tt.value, tt.opts...)
			require.NoError(t, err)

			assert.Equal(t, tt.wantName, ex.Name())
			assert.Equal(t, tt.wantValue, ex.Value())
			assert.Equal(t, tt.wantSummary, ex.Summary())
			assert.Equal(t, tt.wantDesc, ex.Description())
			assert.False(t, ex.IsExternal())
			assert.Empty(t, ex.ExternalValue())
		})
	}
}

func TestNewExternal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		exName      string
		url         string
		opts        []Option
		wantName    string
		wantURL     string
		wantSummary string
	}{
		{
			name:     "basic external",
			exName:   "large",
			url:      "https://example.com/large.json",
			wantName: "large",
			wantURL:  "https://example.com/large.json",
		},
		{
			name:        "with summary",
			exName:      "xml",
			url:         "https://example.com/data.xml",
			opts:        []Option{WithSummary("XML format")},
			wantName:    "xml",
			wantURL:     "https://example.com/data.xml",
			wantSummary: "XML format",
		},
		{
			name:     "empty URL",
			exName:   "empty",
			url:      "",
			wantName: "empty",
			wantURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ex, err := NewExternal(tt.exName, tt.url, tt.opts...)
			require.NoError(t, err)

			assert.Equal(t, tt.wantName, ex.Name())
			assert.Equal(t, tt.wantURL, ex.ExternalValue())
			assert.Equal(t, tt.wantSummary, ex.Summary())
			assert.Equal(t, tt.url != "", ex.IsExternal())
			assert.Nil(t, ex.Value())
		})
	}
}

func TestWithSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary string
		want    string
	}{
		{name: "normal summary", summary: "Test summary", want: "Test summary"},
		{name: "empty summary", summary: "", want: ""},
		{name: "unicode summary", summary: "Résumé 日本語", want: "Résumé 日本語"},
		{name: "markdown summary", summary: "**Bold** text", want: "**Bold** text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ex, err := New("test", nil, WithSummary(tt.summary))
			require.NoError(t, err)

			assert.Equal(t, tt.want, ex.Summary())
		})
	}
}

func TestWithDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		desc string
		want string
	}{
		{name: "normal description", desc: "Detailed description", want: "Detailed description"},
		{name: "multiline", desc: "Line 1\nLine 2", want: "Line 1\nLine 2"},
		{name: "CommonMark", desc: "## Heading\n\n- Item 1\n- Item 2", want: "## Heading\n\n- Item 1\n- Item 2"},
		{name: "empty", desc: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ex, err := New("test", nil, WithDescription(tt.desc))
			require.NoError(t, err)

			assert.Equal(t, tt.want, ex.Description())
		})
	}
}

func TestExample_IsExternal(t *testing.T) {
	t.Parallel()

	t.Run("inline example is not external", func(t *testing.T) {
		t.Parallel()
		ex, err := New("test", "value")
		require.NoError(t, err)
		assert.False(t, ex.IsExternal())
	})

	t.Run("external example is external", func(t *testing.T) {
		t.Parallel()
		ex, err := NewExternal("test", "https://example.com/data.json")
		require.NoError(t, err)
		assert.True(t, ex.IsExternal())
	})

	t.Run("external with empty URL is not external", func(t *testing.T) {
		t.Parallel()
		ex, err := NewExternal("test", "")
		require.NoError(t, err)
		assert.False(t, ex.IsExternal())
	})
}

func TestNew_NilOptionReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New("test", "value", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option at index 0 cannot be nil")
}

func TestNewExternal_NilOptionReturnsError(t *testing.T) {
	t.Parallel()

	_, err := NewExternal("test", "https://example.com/data.json", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option at index 0 cannot be nil")
}

func TestMustNew_PanicsOnNilOption(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "example.MustNew: openapi/example: option at index 0 cannot be nil", func() {
		_ = MustNew("test", "value", nil)
	})
}

func TestMustNewExternal_PanicsOnNilOption(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "example.MustNewExternal: openapi/example: option at index 0 cannot be nil", func() {
		_ = MustNewExternal("test", "https://example.com/data.json", nil)
	})
}
