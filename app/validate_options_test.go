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

	"rivaas.dev/validation"
)

func TestWithValidatePartial(t *testing.T) {
	t.Parallel()

	cfg := applyValidateOptions([]ValidateOption{WithValidatePartial()})
	assert.True(t, cfg.partial)
	assert.False(t, cfg.strict)
	assert.Nil(t, cfg.validationOpts)
}

func TestWithValidateStrict(t *testing.T) {
	t.Parallel()

	cfg := applyValidateOptions([]ValidateOption{WithValidateStrict()})
	assert.False(t, cfg.partial)
	assert.True(t, cfg.strict)
	assert.Nil(t, cfg.validationOpts)
}

func TestWithValidateOptions(t *testing.T) {
	t.Parallel()

	opt := validation.WithMaxErrors(3)
	cfg := applyValidateOptions([]ValidateOption{WithValidateOptions(opt)})
	assert.False(t, cfg.partial)
	assert.False(t, cfg.strict)
	assert.Len(t, cfg.validationOpts, 1)
}

func TestApplyValidateOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		opts        []ValidateOption
		wantPartial bool
		wantStrict  bool
		wantOptsLen int
	}{
		{
			name:        "no options",
			opts:        nil,
			wantPartial: false,
			wantStrict:  false,
			wantOptsLen: 0,
		},
		{
			name:        "with partial",
			opts:        []ValidateOption{WithValidatePartial()},
			wantPartial: true,
			wantStrict:  false,
			wantOptsLen: 0,
		},
		{
			name:        "with strict",
			opts:        []ValidateOption{WithValidateStrict()},
			wantPartial: false,
			wantStrict:  true,
			wantOptsLen: 0,
		},
		{
			name: "multiple options",
			opts: []ValidateOption{
				WithValidatePartial(),
				WithValidateStrict(),
				WithValidateOptions(validation.WithMaxErrors(5)),
			},
			wantPartial: true,
			wantStrict:  true,
			wantOptsLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := applyValidateOptions(tt.opts)

			assert.Equal(t, tt.wantPartial, cfg.partial)
			assert.Equal(t, tt.wantStrict, cfg.strict)
			assert.Len(t, cfg.validationOpts, tt.wantOptsLen)
		})
	}
}
