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

// TestMaxDepth_Enforcement tests that MaxDepth is enforced to prevent stack overflow
func TestMaxDepth_Enforcement(t *testing.T) {
	t.Parallel()

	t.Run("default max depth", func(t *testing.T) {
		t.Parallel()

		// Create a deeply nested struct that exceeds DefaultMaxDepth (32)
		// We need 33 levels to trigger the error (depth 33 > 32)
		type Level1 struct {
			Level2 struct {
				Level3 struct {
					Level4 struct {
						Level5 struct {
							Level6 struct {
								Level7 struct {
									Level8 struct {
										Level9 struct {
											Level10 struct {
												Level11 struct {
													Level12 struct {
														Level13 struct {
															Level14 struct {
																Level15 struct {
																	Level16 struct {
																		Level17 struct {
																			Level18 struct {
																				Level19 struct {
																					Level20 struct {
																						Level21 struct {
																							Level22 struct {
																								Level23 struct {
																									Level24 struct {
																										Level25 struct {
																											Level26 struct {
																												Level27 struct {
																													Level28 struct {
																														Level29 struct {
																															Level30 struct {
																																Level31 struct {
																																	Level32 struct {
																																		Level33 struct {
																																			Level34 struct {
																																				Value string `query:"value"`
																																			} `query:"level34"`
																																		} `query:"level33"`
																																	} `query:"level32"`
																																} `query:"level31"`
																															} `query:"level30"`
																														} `query:"level29"`
																													} `query:"level28"`
																												} `query:"level27"`
																											} `query:"level26"`
																										} `query:"level25"`
																									} `query:"level24"`
																								} `query:"level23"`
																							} `query:"level22"`
																						} `query:"level21"`
																					} `query:"level20"`
																				} `query:"level19"`
																			} `query:"level18"`
																		} `query:"level17"`
																	} `query:"level16"`
																} `query:"level15"`
															} `query:"level14"`
														} `query:"level13"`
													} `query:"level12"`
												} `query:"level11"`
											} `query:"level10"`
										} `query:"level9"`
									} `query:"level8"`
								} `query:"level7"`
							} `query:"level6"`
						} `query:"level5"`
					} `query:"level4"`
				} `query:"level3"`
			} `query:"level2"`
		}

		var params Level1
		values := url.Values{}
		// This path has 34 levels of nesting (depth 33 when binding Level34)
		values.Set("level2.level3.level4.level5.level6.level7.level8.level9.level10.level11.level12.level13.level14.level15.level16.level17.level18.level19.level20.level21.level22.level23.level24.level25.level26.level27.level28.level29.level30.level31.level32.level33.level34.value", "test")

		err := Bind(&params, NewQueryGetter(values), TagQuery)
		require.Error(t, err)
		require.ErrorContains(t, err, "exceeded maximum nesting depth")
		require.ErrorContains(t, err, "32")
	})

	t.Run("custom max depth", func(t *testing.T) {
		t.Parallel()

		type Nested struct {
			Inner struct {
				Deeper struct {
					Value string `query:"value"`
				} `query:"deeper"`
			} `query:"inner"`
		}

		var params Nested
		values := url.Values{}
		values.Set("inner.deeper.value", "test")

		// Set custom max depth to 1 (should fail at depth 2)
		// Depth 0: Nested, Depth 1: Inner, Depth 2: Deeper (exceeds limit)
		err := Bind(&params, NewQueryGetter(values), TagQuery, WithMaxDepth(1))
		require.Error(t, err)
		require.ErrorContains(t, err, "exceeded maximum nesting depth")
		require.ErrorContains(t, err, "1")
	})

	t.Run("within max depth", func(t *testing.T) {
		t.Parallel()

		type Nested struct {
			Inner struct {
				Value string `query:"value"`
			} `query:"inner"`
		}

		var params Nested
		values := url.Values{}
		values.Set("inner.value", "test")

		// Should succeed with default depth (32)
		err := Bind(&params, NewQueryGetter(values), TagQuery)
		require.NoError(t, err)
		assert.Equal(t, "test", params.Inner.Value)
	})
}

// TestMaxMapSize_Enforcement tests that map size limits are enforced
func TestMaxMapSize_Enforcement(t *testing.T) {
	t.Parallel()

	type Params struct {
		Metadata map[string]string `query:"metadata"`
	}

	tests := []struct {
		name          string
		setupValues   func() url.Values
		opts          []Option
		wantError     bool
		errorContains []string
		validate      func(t *testing.T, params Params)
	}{
		{
			name: "exceeds default max map size",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create more than DefaultMaxMapSize (1000) entries
				for i := range 1001 {
					values.Set("metadata.key"+url.QueryEscape(string(rune(i))), "value")
					_ = i // used in string conversion
				}
				return values
			},
			opts:          nil,
			wantError:     true,
			errorContains: []string{"map exceeds max size", "1000"},
			validate:      nil,
		},
		{
			name: "custom max map size",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create 11 entries (should exceed custom limit of 10)
				for i := range 11 {
					values.Set("metadata.key"+url.QueryEscape(string(rune(i))), "value")
				}
				return values
			},
			opts:          []Option{WithMaxMapSize(10)},
			wantError:     true,
			errorContains: []string{"map exceeds max size", "10"},
			validate:      nil,
		},
		{
			name: "within max map size",
			setupValues: func() url.Values {
				values := url.Values{}
				values.Set("metadata.key1", "value1")
				values.Set("metadata.key2", "value2")
				return values
			},
			opts:          nil,
			wantError:     false,
			errorContains: nil,
			validate: func(t *testing.T, params Params) {
				assert.Equal(t, "value1", params.Metadata["key1"])
				assert.Equal(t, "value2", params.Metadata["key2"])
			},
		},
		{
			name: "disabled map size limit",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create 100 entries (should work with limit disabled)
				for i := range 100 {
					values.Set("metadata.key"+url.QueryEscape(string(rune(i))), "value")
				}
				return values
			},
			opts:          []Option{WithMaxMapSize(0)},
			wantError:     false,
			errorContains: nil,
			validate: func(t *testing.T, params Params) {
				assert.Len(t, params.Metadata, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var params Params
			values := tt.setupValues()

			err := Bind(&params, NewQueryGetter(values), TagQuery, tt.opts...)
			if tt.wantError {
				require.Error(t, err)
				for _, contains := range tt.errorContains {
					assert.ErrorContains(t, err, contains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, params)
				}
			}
		})
	}
}

// TestMaxSliceLen_Enforcement tests that slice length limits are enforced
func TestMaxSliceLen_Enforcement(t *testing.T) {
	t.Parallel()

	type Params struct {
		Tags []string `query:"tags"`
	}

	tests := []struct {
		name          string
		setupValues   func() url.Values
		opts          []Option
		wantError     bool
		errorContains []string
		validate      func(t *testing.T, params Params)
	}{
		{
			name: "exceeds default max slice len",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create more than DefaultMaxSliceLen (10,000) entries
				for range 10001 {
					values.Add("tags", "tag")
				}
				return values
			},
			opts:          nil,
			wantError:     true,
			errorContains: []string{"slice exceeds max length", "10000"},
			validate:      nil,
		},
		{
			name: "custom max slice len",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create 11 entries (should exceed custom limit of 10)
				for range 11 {
					values.Add("tags", "tag")
				}
				return values
			},
			opts:          []Option{WithMaxSliceLen(10)},
			wantError:     true,
			errorContains: []string{"slice exceeds max length", "10"},
			validate:      nil,
		},
		{
			name: "within max slice len",
			setupValues: func() url.Values {
				values := url.Values{}
				values.Add("tags", "tag1")
				values.Add("tags", "tag2")
				return values
			},
			opts:          nil,
			wantError:     false,
			errorContains: nil,
			validate: func(t *testing.T, params Params) {
				assert.Equal(t, []string{"tag1", "tag2"}, params.Tags)
			},
		},
		{
			name: "CSV mode respects limit",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create CSV with 11 entries (should exceed custom limit of 10)
				values.Set("tags", "tag,tag,tag,tag,tag,tag,tag,tag,tag,tag,tag")
				return values
			},
			opts:          []Option{WithSliceParseMode(SliceCSV), WithMaxSliceLen(10)},
			wantError:     true,
			errorContains: []string{"slice exceeds max length"},
			validate:      nil,
		},
		{
			name: "disabled slice len limit",
			setupValues: func() url.Values {
				values := url.Values{}
				// Create 100 entries (should work with limit disabled)
				for range 100 {
					values.Add("tags", "tag")
				}
				return values
			},
			opts:          []Option{WithMaxSliceLen(0)},
			wantError:     false,
			errorContains: nil,
			validate: func(t *testing.T, params Params) {
				assert.Len(t, params.Tags, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var params Params
			values := tt.setupValues()

			err := Bind(&params, NewQueryGetter(values), TagQuery, tt.opts...)
			if tt.wantError {
				require.Error(t, err)
				for _, contains := range tt.errorContains {
					assert.ErrorContains(t, err, contains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, params)
				}
			}
		})
	}
}

// TestDefaultOptions_SafeDefaults tests that default options have safe values
func TestDefaultOptions_SafeDefaults(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()

	assert.Equal(t, DefaultMaxDepth, opts.MaxDepth, "MaxDepth should default to DefaultMaxDepth")
	assert.Equal(t, DefaultMaxMapSize, opts.maxMapSize, "maxMapSize should default to DefaultMaxMapSize")
	assert.Equal(t, DefaultMaxSliceLen, opts.maxSliceLen, "maxSliceLen should default to DefaultMaxSliceLen")
	assert.Equal(t, UnknownIgnore, opts.UnknownFields, "UnknownFields should default to UnknownIgnore")
}

// TestOptions_ConcurrencySafety tests that options can be safely reused
func TestOptions_ConcurrencySafety(t *testing.T) {
	t.Parallel()

	t.Run("option functions are reusable", func(t *testing.T) {
		t.Parallel()

		opt := WithMaxDepth(10)

		// Apply multiple times - should work
		opts1 := applyOptions([]Option{opt})
		opts2 := applyOptions([]Option{opt})

		assert.Equal(t, 10, opts1.MaxDepth)
		assert.Equal(t, 10, opts2.MaxDepth)
		assert.NotSame(t, opts1, opts2, "Each call should create a new Options instance")
	})

	t.Run("options instances are independent", func(t *testing.T) {
		t.Parallel()

		opts1 := applyOptions([]Option{WithMaxDepth(10)})
		opts2 := applyOptions([]Option{WithMaxDepth(20)})

		assert.Equal(t, 10, opts1.MaxDepth)
		assert.Equal(t, 20, opts2.MaxDepth)
	})
}
