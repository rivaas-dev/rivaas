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

package binding

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMustWarmupCache tests MustWarmupCache behavior
func TestMustWarmupCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []any
		wantPanic bool
		panicMsg  string
	}{
		{
			name: "valid structs",
			args: []any{
				struct {
					Name string `query:"name"`
				}{},
			},
			wantPanic: false,
		},
		{
			name:      "nil type panics",
			args:      []any{nil},
			wantPanic: true,
			panicMsg:  "MustWarmupCache should panic on nil type",
		},
		{
			name:      "non-struct type panics",
			args:      []any{42},
			wantPanic: true,
			panicMsg:  "MustWarmupCache should panic on non-struct type",
		},
		{
			name: "pointer to non-struct panics",
			args: func() []any {
				var x int
				return []any{&x}
			}(),
			wantPanic: true,
			panicMsg:  "MustWarmupCache should panic on pointer to non-struct",
		},
		{
			name: "multiple valid structs",
			args: []any{
				struct {
					Name string `query:"name"`
				}{},
				struct {
					Age int `query:"age"`
				}{},
			},
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					MustWarmupCache(tt.args...)
				}, tt.panicMsg)
			} else {
				assert.NotPanics(t, func() {
					MustWarmupCache(tt.args...)
				})
			}
		})
	}
}

// TestGetStructInfo_DefensiveChecks tests defensive checks in getStructInfo
func TestGetStructInfo_DefensiveChecks(t *testing.T) {
	t.Parallel()

	type Params struct {
		Name string `query:"name"`
	}

	tests := []struct {
		name      string
		typ       reflect.Type
		tag       string
		wantPanic bool
		panicMsg  string
	}{
		{
			name:      "nil type panics",
			typ:       nil,
			tag:       TagQuery,
			wantPanic: true,
			panicMsg:  "getStructInfo should panic on nil type",
		},
		{
			name:      "empty tag panics",
			typ:       reflect.TypeFor[Params](),
			tag:       "",
			wantPanic: true,
			panicMsg:  "getStructInfo should panic on empty tag",
		},
		{
			name:      "non-struct type panics",
			typ:       reflect.TypeFor[int](),
			tag:       TagQuery,
			wantPanic: true,
			panicMsg:  "getStructInfo should panic on non-struct type",
		},
		{
			name:      "pointer type normalized",
			typ:       reflect.TypeFor[*Params](),
			tag:       TagQuery,
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					getStructInfo(tt.typ, tt.tag)
				}, tt.panicMsg)
			} else {
				assert.NotPanics(t, func() {
					getStructInfo(tt.typ, tt.tag)
				})
			}
		})
	}
}
