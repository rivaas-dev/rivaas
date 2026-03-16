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

package errors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Default(t *testing.T) {
	t.Parallel()

	f, err := New()
	require.NoError(t, err)
	require.NotNil(t, f)

	// Default is RFC9457 with empty base URL
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := f.Format(req, &testError{message: "test"})
	assert.Equal(t, http.StatusInternalServerError, resp.Status)
	assert.Equal(t, "application/problem+json; charset=utf-8", resp.ContentType)

	_, ok := f.(*RFC9457)
	assert.True(t, ok, "default formatter should be *RFC9457")
}

func TestNew_WithRFC9457(t *testing.T) {
	t.Parallel()

	f, err := New(WithRFC9457("https://api.example.com/problems"))
	require.NoError(t, err)
	require.NotNil(t, f)

	rfc, ok := f.(*RFC9457)
	require.True(t, ok)
	assert.Equal(t, "https://api.example.com/problems", rfc.BaseURL)
}

func TestNew_WithJSONAPI(t *testing.T) {
	t.Parallel()

	f, err := New(WithJSONAPI())
	require.NoError(t, err)
	require.NotNil(t, f)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := f.Format(req, &testError{message: "test"})
	assert.Equal(t, "application/vnd.api+json; charset=utf-8", resp.ContentType)

	_, ok := f.(*JSONAPI)
	assert.True(t, ok)
}

func TestNew_WithSimple(t *testing.T) {
	t.Parallel()

	f, err := New(WithSimple())
	require.NoError(t, err)
	require.NotNil(t, f)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := f.Format(req, &testError{message: "test"})
	assert.Equal(t, "application/json; charset=utf-8", resp.ContentType)

	_, ok := f.(*Simple)
	assert.True(t, ok)
}

func TestNew_MultipleFormatterTypes_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(WithRFC9457(""), WithJSONAPI())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple formatter types")

	_, err = New(WithJSONAPI(), WithSimple())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple formatter types")

	_, err = New(WithRFC9457(""), WithSimple())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple formatter types")
}

func TestNew_OptionalRFC9457Options(t *testing.T) {
	t.Parallel()

	f, err := New(
		WithRFC9457("https://api.example.com/problems"),
		WithDisableProblemErrorID(),
		WithProblemTypeResolver(func(error) string { return "custom-type" }),
	)
	require.NoError(t, err)
	require.NotNil(t, f)

	rfc, ok := f.(*RFC9457)
	require.True(t, ok, "formatter should be *RFC9457")
	assert.True(t, rfc.DisableErrorID)
	assert.NotNil(t, rfc.TypeResolver)
	assert.Equal(t, "custom-type", rfc.TypeResolver(nil))
}

func TestMustNew_Default(t *testing.T) {
	t.Parallel()

	f := MustNew()
	require.NotNil(t, f)
	_, ok := f.(*RFC9457)
	assert.True(t, ok)
}

func TestMustNew_WithOptions(t *testing.T) {
	t.Parallel()

	f := MustNew(WithRFC9457("https://example.com/problems"))
	require.NotNil(t, f)
	rfc, ok := f.(*RFC9457)
	require.True(t, ok, "formatter should be *RFC9457")
	assert.Equal(t, "https://example.com/problems", rfc.BaseURL)
}

func TestMustNew_PanicsOnInvalidOptions(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		MustNew(WithRFC9457(""), WithJSONAPI())
	})
}
