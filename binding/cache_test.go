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
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWarmupCache tests WarmupCache skips invalid types and populates cache for valid structs.
func TestWarmupCache(t *testing.T) {
	t.Parallel()

	t.Run("valid struct does not panic and populates cache", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Name string `query:"name"`
		}
		assert.NotPanics(t, func() {
			WarmupCache(Params{})
		})

		// Subsequent bind should succeed using cached struct info
		values := url.Values{}
		values.Set("name", "alice")
		var out Params
		err := Raw(NewQueryGetter(values), TagQuery, &out)
		require.NoError(t, err)
		assert.Equal(t, "alice", out.Name)
	})

	t.Run("nil type is skipped without panic", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() {
			WarmupCache(nil)
		})
	})

	t.Run("non-struct type is skipped without panic", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() {
			WarmupCache(42)
		})
	})

	t.Run("pointer to non-struct is skipped without panic", func(t *testing.T) {
		t.Parallel()

		var x int
		assert.NotPanics(t, func() {
			WarmupCache(&x)
		})
	})

	t.Run("mixed valid and invalid types skips invalid only", func(t *testing.T) {
		t.Parallel()

		type Valid struct {
			Page int `query:"page"`
		}
		assert.NotPanics(t, func() {
			WarmupCache(nil, Valid{}, 42)
		})

		values := url.Values{}
		values.Set("page", "2")
		var out Valid
		err := Raw(NewQueryGetter(values), TagQuery, &out)
		require.NoError(t, err)
		assert.Equal(t, 2, out.Page)
	})
}

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
