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

package binding

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidation tests validation logic including enum and type validation
func TestValidation(t *testing.T) {
	t.Parallel()

	t.Run("Enum", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Status string `query:"status" enum:"active,inactive,pending"`
		}

		tests := []struct {
			name           string
			values         url.Values
			wantErr        bool
			expectedErrMsg string
			validate       func(t *testing.T, params Params)
		}{
			{
				name: "valid enum value - active",
				values: func() url.Values {
					v := url.Values{}
					v.Set("status", "active")
					return v
				}(),
				wantErr:        false,
				expectedErrMsg: "",
				validate: func(t *testing.T, params Params) {
					assert.Equal(t, "active", params.Status)
				},
			},
			{
				name: "valid enum value - inactive",
				values: func() url.Values {
					v := url.Values{}
					v.Set("status", "inactive")
					return v
				}(),
				wantErr:        false,
				expectedErrMsg: "",
				validate: func(t *testing.T, params Params) {
					assert.Equal(t, "inactive", params.Status)
				},
			},
			{
				name: "valid enum value - pending",
				values: func() url.Values {
					v := url.Values{}
					v.Set("status", "pending")
					return v
				}(),
				wantErr:        false,
				expectedErrMsg: "",
				validate: func(t *testing.T, params Params) {
					assert.Equal(t, "pending", params.Status)
				},
			},
			{
				name: "invalid enum value",
				values: func() url.Values {
					v := url.Values{}
					v.Set("status", "invalid")
					return v
				}(),
				wantErr:        true,
				expectedErrMsg: "not in allowed values",
				validate:       func(t *testing.T, params Params) {},
			},
			{
				name: "empty value skips validation",
				values: func() url.Values {
					v := url.Values{}
					v.Set("status", "")
					return v
				}(),
				wantErr:        false,
				expectedErrMsg: "",
				validate: func(t *testing.T, params Params) {
					assert.Empty(t, params.Status, "Expected empty status")
				},
			},
			{
				name:           "missing parameter skips validation",
				values:         url.Values{},
				wantErr:        false,
				expectedErrMsg: "",
				validate: func(t *testing.T, params Params) {
					assert.Empty(t, params.Status, "Expected empty status")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var params Params
				err := Raw(NewQueryGetter(tt.values), TagQuery, &params)

				if tt.wantErr {
					require.Error(t, err, "Expected error for %s", tt.name)
					require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
				} else {
					require.NoError(t, err, "Expected no error for %s", tt.name)
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("UnsupportedTypes", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			values         url.Values
			params         any
			wantErr        bool
			expectedErrMsg string
			validate       func(t *testing.T, err error)
		}{
			{
				name: "unsupported type - Array",
				values: func() url.Values {
					v := url.Values{}
					v.Set("data", "1,2,3")
					return v
				}(),
				params: &struct {
					Data [5]int `query:"data"`
				}{},
				wantErr:        true,
				expectedErrMsg: "unsupported type",
				validate: func(t *testing.T, err error) {
					assert.ErrorContains(t, err, "array", "Error should mention 'array'")
				},
			},
			{
				name: "unsupported type - Chan",
				values: func() url.Values {
					v := url.Values{}
					v.Set("channel", "test")
					return v
				}(),
				params: &struct {
					Channel chan int `query:"channel"`
				}{},
				wantErr:        true,
				expectedErrMsg: "unsupported type",
				validate: func(t *testing.T, err error) {
					assert.ErrorContains(t, err, "Chan", "Error should mention 'Chan'")
				},
			},
			{
				name: "unsupported type - Func",
				values: func() url.Values {
					v := url.Values{}
					v.Set("handler", "test")
					return v
				}(),
				params: &struct {
					Handler func() `query:"handler"`
				}{},
				wantErr:        true,
				expectedErrMsg: "unsupported type",
				validate: func(t *testing.T, err error) {
					assert.ErrorContains(t, err, "func", "Error should mention 'func'")
				},
			},
			{
				name: "unsupported type - Complex64",
				values: func() url.Values {
					v := url.Values{}
					v.Set("complex", "1+2i")
					return v
				}(),
				params: &struct {
					Complex complex64 `query:"complex"`
				}{},
				wantErr:        true,
				expectedErrMsg: "unsupported type",
				validate:       func(t *testing.T, err error) {},
			},
			{
				name: "unsupported type - Complex128",
				values: func() url.Values {
					v := url.Values{}
					v.Set("complex", "1+2i")
					return v
				}(),
				params: &struct {
					Complex complex128 `query:"complex"`
				}{},
				wantErr:        true,
				expectedErrMsg: "unsupported type",
				validate:       func(t *testing.T, err error) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

				if tt.wantErr {
					require.Error(t, err, "Expected error for %s", tt.name)
					if tt.expectedErrMsg != "" {
						require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
					}
					tt.validate(t, err)
				} else {
					// May or may not error, just test the path
					_ = err
				}
			})
		}
	})
}
