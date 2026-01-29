// Copyright 2026 The Rivaas Authors
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

package binding_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding"
)

// TestTimeConverter tests TimeConverter factory.
func TestTimeConverter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		layouts  []string
		input    string
		wantErr  bool
		validate func(t *testing.T, result time.Time)
	}{
		{
			name:    "US format - single layout",
			layouts: []string{"01/02/2006"},
			input:   "12/25/2024",
			wantErr: false,
			validate: func(t *testing.T, result time.Time) {
				t.Helper()
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.December, result.Month())
				assert.Equal(t, 25, result.Day())
			},
		},
		{
			name:    "European format - single layout",
			layouts: []string{"02/01/2006"},
			input:   "25/12/2024",
			wantErr: false,
			validate: func(t *testing.T, result time.Time) {
				t.Helper()
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.December, result.Month())
				assert.Equal(t, 25, result.Day())
			},
		},
		{
			name:    "multiple layouts - first matches",
			layouts: []string{"01/02/2006", "2006-01-02"},
			input:   "12/25/2024",
			wantErr: false,
			validate: func(t *testing.T, result time.Time) {
				t.Helper()
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.December, result.Month())
			},
		},
		{
			name:    "multiple layouts - second matches",
			layouts: []string{"01/02/2006", "2006-01-02"},
			input:   "2024-12-25",
			wantErr: false,
			validate: func(t *testing.T, result time.Time) {
				t.Helper()
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.December, result.Month())
			},
		},
		{
			name:    "empty input",
			layouts: []string{"01/02/2006"},
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			layouts: []string{"01/02/2006"},
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "no layouts provided",
			layouts: []string{},
			input:   "2024-12-25",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := binding.TimeConverter(tt.layouts...)
			result, err := converter(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestTimeConverter_Integration tests TimeConverter with actual binding.
func TestTimeConverter_Integration(t *testing.T) {
	t.Parallel()

	type Request struct {
		Date time.Time `query:"date"`
	}

	binder := binding.MustNew(
		binding.WithConverter(binding.TimeConverter("01/02/2006")),
	)

	values := map[string][]string{"date": {"12/25/2024"}}

	result, err := binding.QueryWith[Request](binder, values)
	require.NoError(t, err)
	assert.Equal(t, 2024, result.Date.Year())
	assert.Equal(t, time.December, result.Date.Month())
	assert.Equal(t, 25, result.Date.Day())
}

// TestDurationConverter tests DurationConverter factory.
func TestDurationConverter(t *testing.T) {
	t.Parallel()

	aliases := map[string]time.Duration{
		"fast":    100 * time.Millisecond,
		"normal":  1 * time.Second,
		"slow":    5 * time.Second,
		"default": 30 * time.Second,
	}

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected time.Duration
	}{
		{
			name:     "alias - exact match",
			input:    "fast",
			wantErr:  false,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "alias - case insensitive",
			input:    "FAST",
			wantErr:  false,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "alias - with whitespace",
			input:    "  slow  ",
			wantErr:  false,
			expected: 5 * time.Second,
		},
		{
			name:     "standard duration string",
			input:    "2h30m",
			wantErr:  false,
			expected: 2*time.Hour + 30*time.Minute,
		},
		{
			name:     "standard duration - seconds",
			input:    "45s",
			wantErr:  false,
			expected: 45 * time.Second,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid duration",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := binding.DurationConverter(aliases)
			result, err := converter(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestDurationConverter_NoAliases tests DurationConverter without aliases.
func TestDurationConverter_NoAliases(t *testing.T) {
	t.Parallel()

	converter := binding.DurationConverter(nil)

	result, err := converter("1h30m")
	require.NoError(t, err)
	assert.Equal(t, 90*time.Minute, result)

	_, err = converter("invalid")
	require.Error(t, err)
}

// TestEnumConverter tests EnumConverter factory.
func TestEnumConverter(t *testing.T) {
	t.Parallel()

	type Status string

	const (
		StatusActive   Status = "active"
		StatusPending  Status = "pending"
		StatusDisabled Status = "disabled"
	)

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected Status
	}{
		{
			name:     "valid value - exact match",
			input:    "active",
			wantErr:  false,
			expected: StatusActive,
		},
		{
			name:     "valid value - case insensitive",
			input:    "ACTIVE",
			wantErr:  false,
			expected: StatusActive,
		},
		{
			name:     "valid value - with whitespace",
			input:    "  pending  ",
			wantErr:  false,
			expected: StatusPending,
		},
		{
			name:    "invalid value",
			input:   "unknown",
			wantErr: true,
		},
		{
			name:    "empty value",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := binding.EnumConverter(
				StatusActive,
				StatusPending,
				StatusDisabled,
			)
			result, err := converter(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.input != "" {
					assert.Contains(t, err.Error(), "must be one of")
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestEnumConverter_NoValues tests EnumConverter with no allowed values.
func TestEnumConverter_NoValues(t *testing.T) {
	t.Parallel()

	type Status string

	converter := binding.EnumConverter[Status]()
	_, err := converter("anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no allowed values provided")
}

// TestEnumConverter_Integration tests EnumConverter with actual binding.
func TestEnumConverter_Integration(t *testing.T) {
	t.Parallel()

	type Priority string

	const (
		PriorityLow    Priority = "low"
		PriorityMedium Priority = "medium"
		PriorityHigh   Priority = "high"
	)

	type Request struct {
		Priority Priority `query:"priority"`
	}

	binder := binding.MustNew(
		binding.WithConverter(binding.EnumConverter(
			PriorityLow,
			PriorityMedium,
			PriorityHigh,
		)),
	)

	values := map[string][]string{"priority": {"HIGH"}}

	result, err := binding.QueryWith[Request](binder, values)
	require.NoError(t, err)
	assert.Equal(t, PriorityHigh, result.Priority)
}

// TestBoolConverter tests BoolConverter factory.
func TestBoolConverter(t *testing.T) {
	t.Parallel()

	truthy := []string{"enabled", "active", "on"}
	falsy := []string{"disabled", "inactive", "off"}

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected bool
	}{
		{
			name:     "truthy - enabled",
			input:    "enabled",
			wantErr:  false,
			expected: true,
		},
		{
			name:     "truthy - case insensitive",
			input:    "ACTIVE",
			wantErr:  false,
			expected: true,
		},
		{
			name:     "truthy - with whitespace",
			input:    "  on  ",
			wantErr:  false,
			expected: true,
		},
		{
			name:     "falsy - disabled",
			input:    "disabled",
			wantErr:  false,
			expected: false,
		},
		{
			name:     "falsy - case insensitive",
			input:    "INACTIVE",
			wantErr:  false,
			expected: false,
		},
		{
			name:     "empty defaults to false",
			input:    "",
			wantErr:  false,
			expected: false,
		},
		{
			name:    "invalid value",
			input:   "maybe",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := binding.BoolConverter(truthy, falsy)
			result, err := converter(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "accepted values")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestBoolConverter_Integration tests BoolConverter with actual binding.
func TestBoolConverter_Integration(t *testing.T) {
	t.Parallel()

	type Request struct {
		Feature bool `query:"feature"`
	}

	binder := binding.MustNew(
		binding.WithConverter(binding.BoolConverter(
			[]string{"enabled", "yes"},
			[]string{"disabled", "no"},
		)),
	)

	values := map[string][]string{"feature": {"ENABLED"}}

	result, err := binding.QueryWith[Request](binder, values)
	require.NoError(t, err)
	assert.True(t, result.Feature)
}

// TestConverters_CombinedUsage tests using multiple converter factories together.
func TestConverters_CombinedUsage(t *testing.T) {
	t.Parallel()

	type Status string
	const (
		StatusActive   Status = "active"
		StatusInactive Status = "inactive"
	)

	type Config struct {
		Timeout  time.Duration `query:"timeout"`
		Status   Status        `query:"status"`
		Enabled  bool          `query:"enabled"`
		DeadLine time.Time     `query:"deadline"`
	}

	binder := binding.MustNew(
		binding.WithConverter(binding.DurationConverter(map[string]time.Duration{
			"fast": 100 * time.Millisecond,
			"slow": 5 * time.Second,
		})),
		binding.WithConverter(binding.EnumConverter(StatusActive, StatusInactive)),
		binding.WithConverter(binding.BoolConverter(
			[]string{"yes", "on"},
			[]string{"no", "off"},
		)),
		binding.WithConverter(binding.TimeConverter("2006-01-02")),
	)

	values := map[string][]string{
		"timeout":  {"fast"},
		"status":   {"ACTIVE"},
		"enabled":  {"yes"},
		"deadline": {"2024-12-25"},
	}

	result, err := binding.QueryWith[Config](binder, values)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, result.Timeout)
	assert.Equal(t, StatusActive, result.Status)
	assert.True(t, result.Enabled)
	assert.Equal(t, 2024, result.DeadLine.Year())
}
