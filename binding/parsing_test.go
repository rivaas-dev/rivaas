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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsing tests parsing of various types through the Bind API
func TestParsing(t *testing.T) {
	t.Parallel()

	t.Run("Bool", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input    string
			expected bool
			wantErr  bool
		}{
			// True values
			{"true", true, false},
			{"True", true, false},
			{"TRUE", true, false},
			{"1", true, false},
			{"yes", true, false},
			{"Yes", true, false},
			{"on", true, false},
			{"t", true, false},
			{"y", true, false},

			// False values
			{"false", false, false},
			{"False", false, false},
			{"0", false, false},
			{"no", false, false},
			{"off", false, false},
			{"f", false, false},
			{"n", false, false},
			{"", false, false},

			// Invalid values
			{"invalid", false, true},
			{"maybe", false, true},
			{"2", false, true},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				t.Parallel()

				type TestStruct struct {
					Value bool `query:"value"`
				}
				var result TestStruct
				values := url.Values{}
				values.Set("value", tt.input)
				err := Raw(NewQueryGetter(values), TagQuery, &result)
				if tt.wantErr {
					assert.Error(t, err, "Bind(%q) should return error", tt.input)
				} else {
					require.NoError(t, err, "Bind(%q) should not return error", tt.input)
					assert.Equal(t, tt.expected, result.Value, "Bind(%q) = %v, want %v", tt.input, result.Value, tt.expected)
				}
			})
		}
	})

	t.Run("Time", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"RFC3339", "2024-01-15T10:30:00Z", false},
			{"RFC3339Nano", "2024-01-15T10:30:00.123456789Z", false},
			{"RFC3339 with timezone", "2024-01-15T10:30:00+02:00", false},
			{"Date only", "2024-01-15", false},
			{"DateTime", "2024-01-15 10:30:00", false},
			{"RFC1123", "Mon, 15 Jan 2024 10:30:00 MST", false},
			{"RFC1123Z", "Mon, 15 Jan 2024 10:30:00 -0700", false},
			{"RFC822", "15 Jan 24 10:30 MST", false},
			{"RFC822Z", "15 Jan 24 10:30 -0700", false},
			{"RFC850", "Monday, 15-Jan-24 10:30:00 MST", false},
			{"DateTime without timezone", "2024-01-15T10:30:00", false},
			{"Empty value", "", true},
			{"Whitespace only", "   ", true},
			{"Invalid format", "not-a-date", true},
			{"Invalid date", "2024-13-45", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				type Params struct {
					Time time.Time `query:"time"`
				}

				values := url.Values{}
				values.Set("time", tt.value)

				var params Params
				err := Raw(NewQueryGetter(values), TagQuery, &params)

				if tt.wantErr {
					assert.Error(t, err, "Expected error for %q", tt.value)
				} else {
					require.NoError(t, err, "Unexpected error for %q", tt.value)
					assert.False(t, params.Time.IsZero(), "Time should not be zero for valid format %q", tt.value)
				}
			})
		}
	})
}
