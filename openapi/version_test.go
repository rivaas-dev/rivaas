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
)

func TestVersion_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version Version
		want    string
	}{
		{
			name:    "V30x returns 3.0.x",
			version: V30x,
			want:    "3.0.x",
		},
		{
			name:    "V31x returns 3.1.x",
			version: V31x,
			want:    "3.1.x",
		},
		{
			name:    "custom version returns same string",
			version: Version("2.0"),
			want:    "2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.version.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersion_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Version("3.0.x"), V30x)
	assert.Equal(t, Version("3.1.x"), V31x)
}
