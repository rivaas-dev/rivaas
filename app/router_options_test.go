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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestWithRouter(t *testing.T) {
	t.Parallel()

	t.Run("creates app with router options", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
			WithRouter(
				router.WithBloomFilterSize(2000),
				router.WithCancellationCheck(false),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})

	t.Run("creates app without router options (defaults)", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})

	t.Run("multiple WithRouter calls accumulate", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
			WithRouter(
				router.WithBloomFilterSize(2000),
			),
			WithRouter(
				router.WithCancellationCheck(false),
				router.WithRouteCompilation(true),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})
}
