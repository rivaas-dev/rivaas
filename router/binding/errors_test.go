package binding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindError_Unwrap tests the Unwrap method
func TestBindError_Unwrap(t *testing.T) {
	t.Parallel()

	originalErr := &BindError{
		Field: "age",
		Value: "invalid",
		Type:  "int",
		Tag:   "form",
		Err:   nil,
	}

	innerErr := &BindError{
		Field: "nested",
		Value: "bad",
		Type:  "string",
		Tag:   "json",
	}

	outerErr := &BindError{
		Field: "age",
		Value: "invalid",
		Type:  "int",
		Tag:   "form",
		Err:   innerErr,
	}

	unwrapped := outerErr.Unwrap()
	require.ErrorIs(t, unwrapped, innerErr)

	assert.Nil(t, originalErr.Unwrap())
}
