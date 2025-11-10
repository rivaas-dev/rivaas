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
		tt := tt
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
			typ:       reflect.TypeOf(Params{}),
			tag:       "",
			wantPanic: true,
			panicMsg:  "getStructInfo should panic on empty tag",
		},
		{
			name:      "non-struct type panics",
			typ:       reflect.TypeOf(42),
			tag:       TagQuery,
			wantPanic: true,
			panicMsg:  "getStructInfo should panic on non-struct type",
		},
		{
			name:      "pointer type normalized",
			typ:       reflect.TypeOf(&Params{}),
			tag:       TagQuery,
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		tt := tt
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
