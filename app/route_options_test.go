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

package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteOption_NilOption_ValidatedByValidateRoutes(t *testing.T) {
	t.Parallel()

	a, err := New(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NoError(t, err)

	// Register route with a nil option — must not panic
	a.GET("/x", func(c *Context) {}, nil)

	err = a.ValidateRoutes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route option at index 0 cannot be nil")

	var ce *ConfigErrors
	require.True(t, errors.As(err, &ce))
	require.Len(t, ce.Errors, 1)
	assert.Contains(t, ce.Errors[0].Message, "route option at index 0 cannot be nil")
}

func TestRouteOption_NilOptionAtIndex1_ValidatedByValidateRoutes(t *testing.T) {
	t.Parallel()

	a, err := New(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NoError(t, err)

	// Nil at index 1
	a.GET("/y", func(c *Context) {}, WithBefore(func(c *Context) {}), nil)

	err = a.ValidateRoutes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route option at index 1 cannot be nil")
}

func TestRouteOption_MultipleNilOptions_AllReported(t *testing.T) {
	t.Parallel()

	a, err := New(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NoError(t, err)

	a.GET("/a", func(c *Context) {}, nil)
	a.POST("/b", func(c *Context) {}, nil)

	err = a.ValidateRoutes()
	require.Error(t, err)
	var ce *ConfigErrors
	require.True(t, errors.As(err, &ce))
	assert.GreaterOrEqual(t, len(ce.Errors), 2, "should report both nil option errors")
	assert.Contains(t, err.Error(), "route option at index 0 cannot be nil")
}

func TestValidateRoutes_NoErrors_ReturnsNil(t *testing.T) {
	t.Parallel()

	a, err := New(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NoError(t, err)

	a.GET("/health", func(c *Context) {})

	err = a.ValidateRoutes()
	assert.NoError(t, err)
}
