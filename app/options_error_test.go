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

package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"

	riverrors "rivaas.dev/errors"
)

func TestWithErrorFormatterFor_SingleFormatter(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithErrorFormatterFor("", riverrors.WithRFC9457("https://api.example.com/problems")),
	)
	require.NoError(t, err)

	c, err := TestContextWithBodyAndApp(a, "GET", "/test", nil)
	require.NoError(t, err)

	testErr := errors.New("test error")
	c.Fail(testErr)

	assert.True(t, c.IsAborted())
	rec, ok := c.Response.(*httptest.ResponseRecorder)
	require.True(t, ok, "Response must be *httptest.ResponseRecorder")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	ct := rec.Header().Get("Content-Type")
	assert.Contains(t, ct, "application/json")
}

func TestWithErrorFormatterFor_ContentNegotiation_Accumulates(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithErrorFormatterFor("application/problem+json", riverrors.WithRFC9457("https://api.example.com/problems")),
		WithErrorFormatterFor("application/json", riverrors.WithSimple()),
		WithDefaultErrorFormat("application/problem+json"),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	// Two formatters were registered; Fail uses content negotiation and returns JSON (either format).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")
	rc := &router.Context{Request: req, Response: rec}
	c := a.contextPool.Get()
	c.Context = rc
	c.app = a

	c.Fail(errors.New("test"))
	assert.True(t, c.IsAborted())
	ct := rec.Header().Get("Content-Type")
	assert.True(t, strings.Contains(ct, "application/json") || strings.Contains(ct, "application/problem+json"), "Content-Type should be JSON or problem+json, got %q", ct)
}

func TestWithErrorFormatterFor_SingleThenContentNegotiated_ValidationFails(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithErrorFormatterFor("", riverrors.WithRFC9457("https://api.example.com/problems")),
		WithErrorFormatterFor("application/json", riverrors.WithSimple()),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use content-negotiated formatters when single error formatter is configured")
}

func TestWithErrorFormatterFor_ContentNegotiatedThenSingle_ValidationFails(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithErrorFormatterFor("application/json", riverrors.WithSimple()),
		WithErrorFormatterFor("", riverrors.WithRFC9457("https://api.example.com/problems")),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use single error formatter when content-negotiated formatters are configured")
}

func TestWithErrorFormatterFor_InvalidOptions_ValidationFails(t *testing.T) {
	t.Parallel()

	// Passing both WithRFC9457 and WithSimple causes errors.New to fail (conflict)
	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithErrorFormatterFor("", riverrors.WithRFC9457(""), riverrors.WithSimple()),
	)
	require.Error(t, err)
	var ce *ConfigErrors
	require.True(t, errors.As(err, &ce))
	found := false
	for _, e := range ce.Errors {
		if e.Field == "errors" {
			found = true
			assert.Contains(t, e.Message, "errors:")
			break
		}
	}
	assert.True(t, found, "expected errors field in validation result")
}
