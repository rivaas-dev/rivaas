package app

import (
	"testing"

	"rivaas.dev/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRouterOptions(t *testing.T) {
	t.Run("creates app with router options", func(t *testing.T) {
		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
			WithRouterOptions(
				router.WithBloomFilterSize(2000),
				router.WithCancellationCheck(false),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})

	t.Run("creates app without router options (defaults)", func(t *testing.T) {
		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})

	t.Run("multiple WithRouterOptions calls accumulate", func(t *testing.T) {
		app, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("1.0.0"),
			WithRouterOptions(
				router.WithBloomFilterSize(2000),
			),
			WithRouterOptions(
				router.WithCancellationCheck(false),
				router.WithTemplateRouting(true),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})
}
