// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEncoder_UnregisteredType(t *testing.T) {
	t.Parallel()

	encoder, err := GetEncoder(Type("unknown"))
	require.Error(t, err)
	assert.Nil(t, encoder)
	assert.Contains(t, err.Error(), "encoder not found for type:")
	assert.Contains(t, err.Error(), "unknown")
}

func TestGetDecoder_UnregisteredType(t *testing.T) {
	t.Parallel()

	decoder, err := GetDecoder(Type("unknown"))
	require.Error(t, err)
	assert.Nil(t, decoder)
	assert.Contains(t, err.Error(), "decoder not found for type:")
	assert.Contains(t, err.Error(), "unknown")
}
