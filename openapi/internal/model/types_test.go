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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoAdditionalProps(t *testing.T) {
	t.Parallel()

	additional := NoAdditionalProps()

	assert.NotNil(t, additional)
	assert.NotNil(t, additional.Allow)
	assert.False(t, *additional.Allow)
	assert.Nil(t, additional.Schema)
}

func TestAdditionalPropsSchema(t *testing.T) {
	t.Parallel()

	schema := &Schema{
		Kind: KindString,
	}

	additional := AdditionalPropsSchema(schema)

	assert.NotNil(t, additional)
	assert.Nil(t, additional.Allow)
	assert.Equal(t, schema, additional.Schema)
}

func TestBound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     float64
		exclusive bool
	}{
		{"inclusive minimum", 0.0, false},
		{"exclusive minimum", 0.0, true},
		{"inclusive maximum", 100.0, false},
		{"exclusive maximum", 100.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bound := &Bound{
				Value:     tt.value,
				Exclusive: tt.exclusive,
			}

			assert.InDelta(t, tt.value, bound.Value, 0.001)
			assert.Equal(t, tt.exclusive, bound.Exclusive)
		})
	}
}
