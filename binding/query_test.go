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
	"net"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBind_QueryBasic tests basic query parameter binding
func TestBind_QueryBasic(t *testing.T) {
	t.Parallel()

	type SearchParams struct {
		Query    string `query:"q"`
		Page     int    `query:"page"`
		PageSize int    `query:"page_size"`
		Active   bool   `query:"active"`
	}

	tests := []struct {
		name     string
		values   url.Values
		wantErr  bool
		validate func(t *testing.T, params SearchParams, err error)
	}{
		{
			name: "all fields",
			values: func() url.Values {
				v := url.Values{}
				v.Set("q", "golang")
				v.Set("page", "2")
				v.Set("page_size", "20")
				v.Set("active", "true")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params SearchParams, err error) {
				assert.Equal(t, "golang", params.Query)
				assert.Equal(t, 2, params.Page)
				assert.Equal(t, 20, params.PageSize)
				assert.True(t, params.Active)
			},
		},
		{
			name: "partial fields",
			values: func() url.Values {
				v := url.Values{}
				v.Set("q", "test")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params SearchParams, err error) {
				assert.Equal(t, "test", params.Query)
				assert.Equal(t, 0, params.Page, "Page should be zero value when not provided")
			},
		},
		{
			name: "invalid integer",
			values: func() url.Values {
				v := url.Values{}
				v.Set("page", "invalid")

				return v
			}(),
			wantErr: true,
			validate: func(t *testing.T, params SearchParams, err error) {
				var bindErr *BindError
				require.ErrorAs(t, err, &bindErr, "Expected BindError")
				assert.Equal(t, "Page", bindErr.Field)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params SearchParams
			err := Raw(getter, TagQuery, &params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
			}
			tt.validate(t, params, err)
		})
	}
}

// TestBind_QuerySlices tests slice binding in query parameters
func TestBind_QuerySlices(t *testing.T) {
	t.Parallel()

	type TagRequest struct {
		Tags []string `query:"tags"`
		IDs  []int    `query:"ids"`
	}

	tests := []struct {
		name     string
		values   url.Values
		wantErr  bool
		validate func(t *testing.T, params TagRequest)
	}{
		{
			name: "string slice",
			values: func() url.Values {
				v := url.Values{}
				v.Add("tags", "go")
				v.Add("tags", "rust")
				v.Add("tags", "python")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params TagRequest) {
				require.Len(t, params.Tags, 3)
				assert.Equal(t, "go", params.Tags[0])
				assert.Equal(t, "rust", params.Tags[1])
				assert.Equal(t, "python", params.Tags[2])
			},
		},
		{
			name: "int slice",
			values: func() url.Values {
				v := url.Values{}
				v.Add("ids", "1")
				v.Add("ids", "2")
				v.Add("ids", "3")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params TagRequest) {
				require.Len(t, params.IDs, 3)
				assert.Equal(t, 1, params.IDs[0])
				assert.Equal(t, 2, params.IDs[1])
				assert.Equal(t, 3, params.IDs[2])
			},
		},
		{
			name: "invalid int in slice",
			values: func() url.Values {
				v := url.Values{}
				v.Add("ids", "1")
				v.Add("ids", "invalid")
				v.Add("ids", "3")

				return v
			}(),
			wantErr:  true,
			validate: func(t *testing.T, params TagRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params TagRequest
			err := Raw(getter, TagQuery, &params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBind_QueryPointers tests pointer field binding in query parameters
func TestBind_QueryPointers(t *testing.T) {
	t.Parallel()

	type OptionalParams struct {
		Name   *string `query:"name"`
		Age    *int    `query:"age"`
		Active *bool   `query:"active"`
	}

	tests := []struct {
		name     string
		values   url.Values
		validate func(t *testing.T, params OptionalParams)
	}{
		{
			name: "all values present",
			values: func() url.Values {
				v := url.Values{}
				v.Set("name", "John")
				v.Set("age", "30")
				v.Set("active", "true")

				return v
			}(),
			validate: func(t *testing.T, params OptionalParams) {
				require.NotNil(t, params.Name)
				assert.Equal(t, "John", *params.Name)
				require.NotNil(t, params.Age)
				assert.Equal(t, 30, *params.Age)
				require.NotNil(t, params.Active)
				assert.True(t, *params.Active)
			},
		},
		{
			name: "missing values remain nil",
			values: func() url.Values {
				v := url.Values{}
				v.Set("name", "John")

				return v
			}(),
			validate: func(t *testing.T, params OptionalParams) {
				require.NotNil(t, params.Name)
				assert.Equal(t, "John", *params.Name)
				assert.Nil(t, params.Age, "Age should be nil when not provided")
			},
		},
		{
			name: "empty value remains nil",
			values: func() url.Values {
				v := url.Values{}
				v.Set("name", "")
				v.Set("age", "")

				return v
			}(),
			validate: func(t *testing.T, params OptionalParams) {
				assert.Nil(t, params.Name, "Name should be nil for empty value")
				assert.Nil(t, params.Age, "Age should be nil for empty value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params OptionalParams
			require.NoError(t, Raw(getter, TagQuery, &params))
			tt.validate(t, params)
		})
	}
}

// TestBind_QueryDataTypes tests binding of various data types
func TestBind_QueryDataTypes(t *testing.T) {
	t.Parallel()

	type AllTypes struct {
		String  string  `query:"string"`
		Int     int     `query:"int"`
		Int8    int8    `query:"int8"`
		Int16   int16   `query:"int16"`
		Int32   int32   `query:"int32"`
		Uint    uint    `query:"uint"`
		Float32 float32 `query:"float32"`
		Float64 float64 `query:"float64"`
		Bool    bool    `query:"bool"`
	}

	values := url.Values{}
	values.Set("string", "test")
	values.Set("int", "-42")
	values.Set("int8", "127")
	values.Set("int16", "32000")
	values.Set("int32", "2147483647")
	values.Set("uint", "42")
	values.Set("float32", "3.14")
	values.Set("float64", "2.718281828")
	values.Set("bool", "true")

	getter := NewQueryGetter(values)

	var params AllTypes
	err := Raw(getter, TagQuery, &params)

	require.NoError(t, err)
	assert.Equal(t, "test", params.String)
	assert.Equal(t, -42, params.Int)
	assert.Equal(t, int8(127), params.Int8)
	assert.True(t, params.Bool)
	assert.InDelta(t, 3.14, params.Float32, 0.01)
}

// TestBind_QueryComplexTypes tests binding of complex types (time, duration, IP, URL, regexp)
func TestBind_QueryComplexTypes(t *testing.T) {
	t.Parallel()

	t.Run("Time", func(t *testing.T) {
		t.Parallel()

		type EventParams struct {
			StartDate time.Time  `query:"start"`
			EndDate   time.Time  `query:"end"`
			Created   *time.Time `query:"created"`
		}

		tests := []struct {
			name     string
			values   url.Values
			wantErr  bool
			validate func(t *testing.T, params EventParams)
		}{
			{
				name: "RFC3339 format",
				values: func() url.Values {
					v := url.Values{}
					v.Set("start", "2024-01-15T10:30:00Z")
					v.Set("end", "2024-01-20T15:45:00Z")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params EventParams) {
					expectedStart, err := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
					require.NoError(t, err)
					assert.True(t, params.StartDate.Equal(expectedStart), "StartDate should match expected time")
				},
			},
			{
				name: "date only format",
				values: func() url.Values {
					v := url.Values{}
					v.Set("start", "2024-01-15")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params EventParams) {
					expected, err := time.Parse(time.DateOnly, "2024-01-15")
					require.NoError(t, err)
					assert.True(t, params.StartDate.Equal(expected), "StartDate should match expected date")
				},
			},
			{
				name: "pointer time field",
				values: func() url.Values {
					v := url.Values{}
					v.Set("created", "2024-01-15T10:00:00Z")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params EventParams) {
					require.NotNil(t, params.Created, "Created should not be nil")
					expected, err := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
					require.NoError(t, err)
					assert.True(t, params.Created.Equal(expected), "Created should match expected time")
				},
			},
			{
				name: "invalid time format",
				values: func() url.Values {
					v := url.Values{}
					v.Set("start", "invalid-date")

					return v
				}(),
				wantErr:  true,
				validate: func(t *testing.T, params EventParams) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params EventParams
				err := Raw(getter, TagQuery, &params)

				if tt.wantErr {
					assert.Error(t, err, "Expected error for %s", tt.name)
				} else {
					require.NoError(t, err, "Bind should succeed for %s", tt.name)
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("Duration", func(t *testing.T) {
		t.Parallel()

		type TimeoutParams struct {
			Timeout  time.Duration  `query:"timeout"`
			Interval time.Duration  `query:"interval"`
			TTL      *time.Duration `query:"ttl"`
		}

		tests := []struct {
			name     string
			values   url.Values
			wantErr  bool
			validate func(t *testing.T, params TimeoutParams)
		}{
			{
				name: "valid durations",
				values: func() url.Values {
					v := url.Values{}
					v.Set("timeout", "5s")
					v.Set("interval", "10m")
					v.Set("ttl", "1h")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params TimeoutParams) {
					assert.Equal(t, 5*time.Second, params.Timeout)
					assert.Equal(t, 10*time.Minute, params.Interval)
					require.NotNil(t, params.TTL)
					assert.Equal(t, time.Hour, *params.TTL)
				},
			},
			{
				name: "complex duration",
				values: func() url.Values {
					v := url.Values{}
					v.Set("timeout", "1h30m45s")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params TimeoutParams) {
					expected := time.Hour + 30*time.Minute + 45*time.Second
					assert.Equal(t, expected, params.Timeout)
				},
			},
			{
				name: "invalid duration",
				values: func() url.Values {
					v := url.Values{}
					v.Set("timeout", "invalid")

					return v
				}(),
				wantErr:  true,
				validate: func(t *testing.T, params TimeoutParams) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params TimeoutParams
				err := Raw(getter, TagQuery, &params)

				if tt.wantErr {
					assert.Error(t, err, "Expected error for %s", tt.name)
				} else {
					require.NoError(t, err)
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("IP", func(t *testing.T) {
		t.Parallel()

		type NetworkParams struct {
			AllowedIP net.IP   `query:"allowed_ip"`
			BlockedIP net.IP   `query:"blocked_ip"`
			IPs       []net.IP `query:"ips"`
		}

		tests := []struct {
			name     string
			values   url.Values
			wantErr  bool
			validate func(t *testing.T, params NetworkParams)
		}{
			{
				name: "IPv4 address",
				values: func() url.Values {
					v := url.Values{}
					v.Set("allowed_ip", "192.168.1.1")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					expected := net.ParseIP("192.168.1.1")
					assert.True(t, params.AllowedIP.Equal(expected))
				},
			},
			{
				name: "IPv6 address",
				values: func() url.Values {
					v := url.Values{}
					v.Set("allowed_ip", "2001:0db8:85a3:0000:0000:8a2e:0370:7334")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					assert.NotNil(t, params.AllowedIP, "AllowedIP should not be nil")
				},
			},
			{
				name: "IP slice",
				values: func() url.Values {
					v := url.Values{}
					v.Add("ips", "192.168.1.1")
					v.Add("ips", "10.0.0.1")
					v.Add("ips", "172.16.0.1")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					require.Len(t, params.IPs, 3)
				},
			},
			{
				name: "invalid IP",
				values: func() url.Values {
					v := url.Values{}
					v.Set("allowed_ip", "invalid-ip")

					return v
				}(),
				wantErr:  true,
				validate: func(t *testing.T, params NetworkParams) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params NetworkParams
				err := Raw(getter, TagQuery, &params)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("IPNet", func(t *testing.T) {
		t.Parallel()

		type NetworkParams struct {
			Subnet        net.IPNet   `query:"subnet"`
			AllowedRanges []net.IPNet `query:"ranges"`
			OptionalCIDR  *net.IPNet  `query:"optional"`
		}

		tests := []struct {
			name     string
			values   url.Values
			wantErr  bool
			validate func(t *testing.T, params NetworkParams)
		}{
			{
				name: "valid CIDR",
				values: func() url.Values {
					v := url.Values{}
					v.Set("subnet", "192.168.1.0/24")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					_, expected, err := net.ParseCIDR("192.168.1.0/24")
					require.NoError(t, err)
					assert.Equal(t, expected.String(), params.Subnet.String())
				},
			},
			{
				name: "IPv6 CIDR",
				values: func() url.Values {
					v := url.Values{}
					v.Set("subnet", "2001:db8::/32")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					assert.NotNil(t, params.Subnet.IP, "Subnet IP should not be nil")
				},
			},
			{
				name: "CIDR slice",
				values: func() url.Values {
					v := url.Values{}
					v.Add("ranges", "10.0.0.0/8")
					v.Add("ranges", "172.16.0.0/12")
					v.Add("ranges", "192.168.0.0/16")

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params NetworkParams) {
					require.Len(t, params.AllowedRanges, 3)
				},
			},
			{
				name: "invalid CIDR",
				values: func() url.Values {
					v := url.Values{}
					v.Set("subnet", "invalid-cidr")

					return v
				}(),
				wantErr:  true,
				validate: func(t *testing.T, params NetworkParams) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params NetworkParams
				err := Raw(getter, TagQuery, &params)

				if tt.wantErr {
					require.Error(t, err, "Expected error for %s", tt.name)
					assert.ErrorContains(t, err, "invalid CIDR notation")
				} else {
					require.NoError(t, err, "Bind should succeed for %s", tt.name)
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("URL", func(t *testing.T) {
		t.Parallel()

		type WebhookParams struct {
			CallbackURL url.URL  `query:"callback"`
			RedirectURL url.URL  `query:"redirect"`
			OptionalURL *url.URL `query:"optional"`
		}

		tests := []struct {
			name     string
			values   url.Values
			validate func(t *testing.T, params WebhookParams)
		}{
			{
				name: "valid URL",
				values: func() url.Values {
					v := url.Values{}
					v.Set("callback", "https://example.com/webhook")

					return v
				}(),
				validate: func(t *testing.T, params WebhookParams) {
					expected := "https://example.com/webhook"
					assert.Equal(t, expected, params.CallbackURL.String())
				},
			},
			{
				name: "URL with query params",
				values: func() url.Values {
					v := url.Values{}
					v.Set("callback", "https://example.com/hook?token=abc&id=123")

					return v
				}(),
				validate: func(t *testing.T, params WebhookParams) {
					assert.Equal(t, "example.com", params.CallbackURL.Host)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params WebhookParams
				require.NoError(t, Raw(getter, TagQuery, &params))
				tt.validate(t, params)
			})
		}
	})

	t.Run("Regexp", func(t *testing.T) {
		t.Parallel()

		type PatternParams struct {
			Pattern       regexp.Regexp  `query:"pattern"`
			OptionalRegex *regexp.Regexp `query:"optional"`
		}

		tests := []struct {
			name     string
			values   url.Values
			wantErr  bool
			validate func(t *testing.T, params PatternParams)
		}{
			{
				name: "valid regexp",
				values: func() url.Values {
					v := url.Values{}
					v.Set("pattern", `^user-[0-9]+$`)

					return v
				}(),
				wantErr: false,
				validate: func(t *testing.T, params PatternParams) {
					expected := `^user-[0-9]+$`
					assert.Equal(t, expected, params.Pattern.String())
					assert.True(t, params.Pattern.MatchString("user-123"), "Pattern should match user-123")
					assert.False(t, params.Pattern.MatchString("admin-123"), "Pattern should not match admin-123")
				},
			},
			{
				name: "invalid regexp",
				values: func() url.Values {
					v := url.Values{}
					v.Set("pattern", "[invalid")

					return v
				}(),
				wantErr:  true,
				validate: func(t *testing.T, params PatternParams) {},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				getter := NewQueryGetter(tt.values)
				var params PatternParams
				err := Raw(getter, TagQuery, &params)

				if tt.wantErr {
					require.Error(t, err, "Expected error for %s", tt.name)
				} else {
					require.NoError(t, err, "Bind should succeed for %s", tt.name)
					tt.validate(t, params)
				}
			})
		}
	})
}

// TestBind_QueryMaps tests map binding with dot and bracket notation
func TestBind_QueryMaps(t *testing.T) {
	t.Parallel()

	type MapParams struct {
		Metadata map[string]string `query:"metadata"`
		Scores   map[string]int    `query:"scores"`
	}

	tests := []struct {
		name     string
		values   url.Values
		wantErr  bool
		validate func(t *testing.T, params MapParams)
	}{
		{
			name: "simple bracket notation",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata[name]", "John")
				v.Set("metadata[age]", "30")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				require.NotNil(t, params.Metadata, "Metadata should not be nil")
				assert.Equal(t, "John", params.Metadata["name"])
				assert.Equal(t, "30", params.Metadata["age"])
			},
		},
		{
			name: "dot notation",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata.name", "John")
				v.Set("metadata.age", "30")
				v.Set("metadata.city", "NYC")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				require.NotNil(t, params.Metadata, "Metadata map should not be nil")
				assert.Equal(t, "John", params.Metadata["name"])
				assert.Equal(t, "30", params.Metadata["age"])
				require.Len(t, params.Metadata, 3)
			},
		},
		{
			name: "typed map values",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores.math", "95")
				v.Set("scores.science", "88")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params MapParams) {
				assert.Equal(t, 95, params.Scores["math"])
				assert.Equal(t, 88, params.Scores["science"])
				require.Len(t, params.Scores, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params MapParams
			err := Raw(getter, TagQuery, &params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
				tt.validate(t, params)
			}
		})
	}
}

// TestBind_QueryNestedStructs tests nested struct support with dot notation
func TestBind_QueryNestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street  string `query:"street"`
		City    string `query:"city"`
		ZipCode string `query:"zip_code"`
	}

	type UserRequest struct {
		Name    string  `query:"name"`
		Email   string  `query:"email"`
		Address Address `query:"address"`
	}

	values := url.Values{}
	values.Set("name", "John")
	values.Set("email", "john@example.com")
	values.Set("address.street", "123 Main St")
	values.Set("address.city", "NYC")
	values.Set("address.zip_code", "10001")

	getter := NewQueryGetter(values)

	var params UserRequest
	err := Raw(getter, TagQuery, &params)

	require.NoError(t, err)
	assert.Equal(t, "John", params.Name)
	assert.Equal(t, "john@example.com", params.Email)
	assert.Equal(t, "123 Main St", params.Address.Street)
	assert.Equal(t, "NYC", params.Address.City)
	assert.Equal(t, "10001", params.Address.ZipCode)
}

// TestBind_QueryEnumValidation tests enum validation
func TestBind_QueryEnumValidation(t *testing.T) {
	t.Parallel()

	type StatusParams struct {
		Status   string `query:"status" enum:"active,inactive,pending"`
		Role     string `query:"role" enum:"admin,user,guest"`
		Priority string `query:"priority" enum:"low,medium,high"`
	}

	tests := []struct {
		name     string
		values   url.Values
		wantErr  bool
		validate func(t *testing.T, params StatusParams)
	}{
		{
			name: "valid enum values",
			values: func() url.Values {
				v := url.Values{}
				v.Set("status", "active")
				v.Set("role", "admin")
				v.Set("priority", "high")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params StatusParams) {
				assert.Equal(t, "active", params.Status)
				assert.Equal(t, "admin", params.Role)
				assert.Equal(t, "high", params.Priority)
			},
		},
		{
			name: "invalid enum value",
			values: func() url.Values {
				v := url.Values{}
				v.Set("status", "invalid-status")

				return v
			}(),
			wantErr:  true,
			validate: func(t *testing.T, params StatusParams) {},
		},
		{
			name: "empty value passes enum validation",
			values: func() url.Values {
				v := url.Values{}
				v.Set("role", "admin")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params StatusParams) {
				assert.Equal(t, "admin", params.Role)
				assert.Empty(t, params.Status, "Status should be empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params StatusParams
			err := Raw(getter, TagQuery, &params)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, "not in allowed values")
			} else {
				require.NoError(t, err)
				tt.validate(t, params)
			}
		})
	}
}

// TestBind_QueryDefaultValues tests default values
func TestBind_QueryDefaultValues(t *testing.T) {
	t.Parallel()

	type ParamsWithDefaults struct {
		Page     int    `query:"page" default:"1"`
		PageSize int    `query:"page_size" default:"10"`
		Sort     string `query:"sort" default:"created_at"`
		Order    string `query:"order" default:"desc"`
		Active   bool   `query:"active" default:"true"`
		Limit    int    `query:"limit" default:"100"`
	}

	tests := []struct {
		name     string
		values   url.Values
		validate func(t *testing.T, params ParamsWithDefaults)
	}{
		{
			name:   "all defaults applied",
			values: url.Values{},
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 1, params.Page, "Page should default to 1")
				assert.Equal(t, 10, params.PageSize, "PageSize should default to 10")
				assert.Equal(t, "created_at", params.Sort, "Sort should default to created_at")
				assert.Equal(t, "desc", params.Order, "Order should default to desc")
				assert.True(t, params.Active, "Active should default to true")
				assert.Equal(t, 100, params.Limit, "Limit should default to 100")
			},
		},
		{
			name: "user values override defaults",
			values: func() url.Values {
				v := url.Values{}
				v.Set("page", "5")
				v.Set("page_size", "50")
				v.Set("active", "false")

				return v
			}(),
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 5, params.Page, "Page should be user value")
				assert.Equal(t, 50, params.PageSize, "PageSize should be user value")
				assert.False(t, params.Active, "Active should be user value")
				assert.Equal(t, "created_at", params.Sort, "Sort should be default")
			},
		},
		{
			name: "partial user values with defaults",
			values: func() url.Values {
				v := url.Values{}
				v.Set("page", "3")

				return v
			}(),
			validate: func(t *testing.T, params ParamsWithDefaults) {
				assert.Equal(t, 3, params.Page, "Page should be user value")
				assert.Equal(t, 10, params.PageSize, "PageSize should be default")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			var params ParamsWithDefaults
			require.NoError(t, Raw(getter, TagQuery, &params))
			tt.validate(t, params)
		})
	}
}

// TestBind_QueryErrorCases tests various error scenarios
func TestBind_QueryErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupParams    func() any
		values         url.Values
		expectedErrMsg string
	}{
		{
			name: "invalid int conversion",
			setupParams: func() any {
				return &struct {
					Age int `query:"age"`
				}{}
			},
			values: func() url.Values {
				v := url.Values{}
				v.Set("age", "not-a-number")

				return v
			}(),
			expectedErrMsg: "Age",
		},
		{
			name: "invalid time format",
			setupParams: func() any {
				return &struct {
					Date time.Time `query:"date"`
				}{}
			},
			values: func() url.Values {
				v := url.Values{}
				v.Set("date", "invalid-date")

				return v
			}(),
			expectedErrMsg: "", // Any error is acceptable
		},
		{
			name: "invalid IP address",
			setupParams: func() any {
				return &struct {
					IP net.IP `query:"ip"`
				}{}
			},
			values: func() url.Values {
				v := url.Values{}
				v.Set("ip", "invalid-ip")

				return v
			}(),
			expectedErrMsg: "invalid IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := tt.setupParams()
			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, params)

			require.Error(t, err)
			if tt.expectedErrMsg != "" {
				assert.ErrorContains(t, err, tt.expectedErrMsg)
			}
		})
	}
}

// TestBind_QueryRealWorld tests real-world query binding scenarios
func TestBind_QueryRealWorld(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   url.Values
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "pagination",
			values: func() url.Values {
				v := url.Values{}
				v.Set("page", "3")
				v.Set("page_size", "50")
				v.Set("sort", "created_at")
				v.Set("order", "desc")

				return v
			}(),
			params: &struct {
				Page     int    `query:"page"`
				PageSize int    `query:"page_size"`
				Sort     string `query:"sort"`
				Order    string `query:"order"`
			}{},
			validate: func(t *testing.T, params any) {
				p, ok := params.(*struct {
					Page     int    `query:"page"`
					PageSize int    `query:"page_size"`
					Sort     string `query:"sort"`
					Order    string `query:"order"`
				})
				require.True(t, ok)
				assert.Equal(t, 3, p.Page)
				assert.Equal(t, 50, p.PageSize)
				assert.Equal(t, "created_at", p.Sort)
				assert.Equal(t, "desc", p.Order)
			},
		},
		{
			name: "filters",
			values: func() url.Values {
				v := url.Values{}
				v.Add("status", "active")
				v.Add("status", "pending")
				v.Add("category", "electronics")
				v.Set("min_price", "10.50")
				v.Set("max_price", "99.99")

				return v
			}(),
			params: &struct {
				Status   []string `query:"status"`
				Category []string `query:"category"`
				MinPrice float64  `query:"min_price"`
				MaxPrice float64  `query:"max_price"`
			}{},
			validate: func(t *testing.T, params any) {
				f, ok := params.(*struct {
					Status   []string `query:"status"`
					Category []string `query:"category"`
					MinPrice float64  `query:"min_price"`
					MaxPrice float64  `query:"max_price"`
				})
				require.True(t, ok)
				require.Len(t, f.Status, 2)
				assert.Equal(t, "active", f.Status[0])
				assert.Equal(t, "pending", f.Status[1])
				assert.InDelta(t, 10.50, f.MinPrice, 0.001)
				assert.InDelta(t, 99.99, f.MaxPrice, 0.001)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, tt.params)

			require.NoError(t, err)
			tt.validate(t, tt.params)
		})
	}
}

// customUUID is a custom type for testing encoding.TextUnmarshaler interface
type customUUID string

// UnmarshalText implements encoding.TextUnmarshaler
func (u *customUUID) UnmarshalText(text []byte) error {
	s := string(text)
	// Simple UUID validation (just check length, not full RFC4122)
	if len(s) != 36 {
		return ErrInvalidUUIDFormat
	}
	*u = customUUID(s)

	return nil
}

// TestBind_QueryTextUnmarshaler tests encoding.TextUnmarshaler interface
func TestBind_QueryTextUnmarshaler(t *testing.T) {
	t.Parallel()

	type Request struct {
		ID       customUUID  `query:"id"`
		TraceID  customUUID  `query:"trace_id"`
		Optional *customUUID `query:"optional"`
	}

	tests := []struct {
		name     string
		values   url.Values
		wantErr  bool
		validate func(t *testing.T, params Request)
	}{
		{
			name: "valid custom type",
			values: func() url.Values {
				v := url.Values{}
				v.Set("id", "550e8400-e29b-41d4-a716-446655440000")
				v.Set("trace_id", "660e8400-e29b-41d4-a716-446655440001")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params Request) {
				expectedID := "550e8400-e29b-41d4-a716-446655440000"
				expectedTraceID := "660e8400-e29b-41d4-a716-446655440001"
				assert.Equal(t, expectedID, string(params.ID))
				assert.Equal(t, expectedTraceID, string(params.TraceID))
			},
		},
		{
			name: "invalid custom type",
			values: func() url.Values {
				v := url.Values{}
				v.Set("id", "invalid-uuid")

				return v
			}(),
			wantErr:  true,
			validate: func(t *testing.T, params Request) {},
		},
		{
			name: "pointer to custom type",
			values: func() url.Values {
				v := url.Values{}
				v.Set("id", "550e8400-e29b-41d4-a716-446655440000")
				v.Set("optional", "770e8400-e29b-41d4-a716-446655440002")

				return v
			}(),
			wantErr: false,
			validate: func(t *testing.T, params Request) {
				require.NotNil(t, params.Optional, "Optional should not be nil")
				expected := "770e8400-e29b-41d4-a716-446655440002"
				assert.Equal(t, expected, string(*params.Optional))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := &Request{}
			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				assert.ErrorContains(t, err, "invalid UUID format")
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
				tt.validate(t, *params)
			}
		})
	}
}

// TestBind_QueryEmbeddedStruct tests embedded struct support
func TestBind_QueryEmbeddedStruct(t *testing.T) {
	t.Parallel()

	type Pagination struct {
		Page     int `query:"page"`
		PageSize int `query:"page_size"`
	}

	type SearchRequest struct {
		Pagination        // Embedded struct
		Query      string `query:"q"`
		Sort       string `query:"sort"`
	}

	type AdvancedSearch struct {
		*Pagination
		Query string `query:"q"`
	}

	t.Run("embedded fields", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Set("q", "golang")
		values.Set("page", "2")
		values.Set("page_size", "20")
		values.Set("sort", "name")

		getter := NewQueryGetter(values)
		var params SearchRequest
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		assert.Equal(t, "golang", params.Query)
		assert.Equal(t, 2, params.Page, "Page from embedded struct")
		assert.Equal(t, 20, params.PageSize, "PageSize from embedded struct")
	})

	t.Run("pointer to embedded struct", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Set("q", "test")
		values.Set("page", "3")
		values.Set("page_size", "30")

		getter := NewQueryGetter(values)
		params := AdvancedSearch{
			Pagination: &Pagination{}, // Must initialize pointer
		}
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		assert.Equal(t, "test", params.Query)
		assert.Equal(t, 3, params.Page)
	})
}

// TestBind_QueryMapJSONFallback tests the JSON string parsing fallback for map fields.
// This tests the code path where no dot/bracket notation is found, so it falls back
// to parsing a JSON string value for the map prefix.
func TestBind_QueryMapJSONFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   url.Values
		params   any
		wantErr  bool
		validate func(t *testing.T, params any, err error)
	}{
		{
			name: "string map from JSON string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)

				return v
			}(),
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata, "Metadata map should not be nil")
				assert.Equal(t, "John", p.Metadata["name"])
				assert.Equal(t, "30", p.Metadata["age"])
				assert.Equal(t, "NYC", p.Metadata["city"])
				require.Len(t, p.Metadata, 3)
			},
		},
		{
			name: "int map from JSON string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores", `{"math":95,"science":88,"history":92}`)

				return v
			}(),
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Scores map[string]int `query:"scores"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Scores, "Scores map should not be nil")
				assert.Equal(t, 95, p.Scores["math"])
				assert.Equal(t, 88, p.Scores["science"])
				assert.Equal(t, 92, p.Scores["history"])
			},
		},
		{
			name: "float64 map from JSON string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("rates", `{"usd":1.0,"eur":0.85,"gbp":0.77}`)

				return v
			}(),
			params: &struct {
				Rates map[string]float64 `query:"rates"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Rates map[string]float64 `query:"rates"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Rates, "Rates map should not be nil")
				assert.InDelta(t, 1.0, p.Rates["usd"], 0.01)
				assert.InDelta(t, 0.85, p.Rates["eur"], 0.01)
			},
		},
		{
			name: "bool map from JSON string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("flags", `{"debug":true,"verbose":false,"trace":true}`)

				return v
			}(),
			params: &struct {
				Flags map[string]bool `query:"flags"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Flags map[string]bool `query:"flags"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Flags, "Flags map should not be nil")
				assert.True(t, p.Flags["debug"])
				assert.False(t, p.Flags["verbose"])
				assert.True(t, p.Flags["trace"])
			},
		},
		{
			name: "interface{} map from JSON string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("settings", `{"debug":true,"port":8080,"name":"server"}`)

				return v
			}(),
			params: &struct {
				Settings map[string]any `query:"settings"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Settings map[string]any `query:"settings"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Settings, "Settings map should not be nil")
				assert.Equal(t, "true", p.Settings["debug"])
				assert.Equal(t, "8080", p.Settings["port"])
				assert.Equal(t, "server", p.Settings["name"])
			},
		},
		{
			name: "empty JSON object",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", `{}`)

				return v
			}(),
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata, "Metadata map should not be nil")
				assert.Empty(t, p.Metadata, "Expected empty map")
			},
		},
		{
			name: "empty JSON string - should not error",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", "")

				return v
			}(),
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				// Should not error, just skip JSON parsing
			},
		},
		{
			name: "invalid JSON - should silently fail without error",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", `{invalid json}`)

				return v
			}(),
			params: &struct {
				Metadata map[string]string `query:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Metadata map[string]string `query:"metadata"`
				})
				require.True(t, ok)
				// Map should remain nil or empty since JSON parsing failed
				assert.Empty(t, p.Metadata, "Metadata should be empty when JSON is invalid")
			},
		},
		{
			name: "type conversion error - should return error",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores", `{"math":"not-a-number"}`)

				return v
			}(),
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				// Error should mention the key
				assert.ErrorContains(t, err, "math", "Error should mention the key 'math'")
			},
		},
		{
			name: "JSON string with numeric keys",
			values: func() url.Values {
				v := url.Values{}
				v.Set("data", `{"123":"value1","456":"value2"}`)

				return v
			}(),
			params: &struct {
				Data map[string]string `query:"data"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				p, ok := params.(*struct {
					Data map[string]string `query:"data"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Data, "Data map should not be nil")
				assert.Equal(t, "value1", p.Data["123"])
				assert.Equal(t, "value2", p.Data["456"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
			}
			tt.validate(t, tt.params, err)
		})
	}
}

// TestBind_QueryPointerMap tests pointer to map types
func TestBind_QueryPointerMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   url.Values
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "pointer to map[string]string",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata.name", "John")
				v.Set("metadata.age", "30")

				return v
			}(),
			params: &struct {
				Metadata *map[string]string `query:"metadata"`
			}{},
			validate: func(t *testing.T, params any) {
				p, ok := params.(*struct {
					Metadata *map[string]string `query:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata, "Metadata map pointer should not be nil")
				assert.Equal(t, "John", (*p.Metadata)["name"])
				assert.Equal(t, "30", (*p.Metadata)["age"])
			},
		},
		{
			name: "pointer to map[string]int",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores.math", "95")
				v.Set("scores.science", "88")

				return v
			}(),
			params: &struct {
				Scores *map[string]int `query:"scores"`
			}{},
			validate: func(t *testing.T, params any) {
				p, ok := params.(*struct {
					Scores *map[string]int `query:"scores"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Scores, "Scores map pointer should not be nil")
				assert.Equal(t, 95, (*p.Scores)["math"])
				assert.Equal(t, 88, (*p.Scores)["science"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, tt.params)

			require.NoError(t, err)
			tt.validate(t, tt.params)
		})
	}
}

// TestBind_QueryMapTypeConversionError tests error path when type conversion fails
func TestBind_QueryMapTypeConversionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   url.Values
		params   any
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "dot notation - invalid int conversion",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores.math", "not-a-number")
				v.Set("scores.science", "88")

				return v
			}(),
			params: &struct {
				Scores map[string]int `query:"scores"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "math", "Error should mention key 'math'")
			},
		},
		{
			name: "bracket notation - invalid float conversion",
			values: func() url.Values {
				v := url.Values{}
				v.Set("rates[usd]", "invalid-float")

				return v
			}(),
			params: &struct {
				Rates map[string]float64 `query:"rates"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "usd", "Error should mention key 'usd'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				tt.validate(t, err)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
			}
		})
	}
}

// TestBind_QueryAllComplexTypes tests combined complex features
func TestBind_QueryAllComplexTypes(t *testing.T) {
	t.Parallel()

	type ComplexParams struct {
		// Basic types
		Name string `query:"name"`
		Age  int    `query:"age"`

		// Time types
		StartDate time.Time     `query:"start"`
		Timeout   time.Duration `query:"timeout"`

		// Network types
		AllowedIP   net.IP    `query:"allowed_ip"`
		Subnet      net.IPNet `query:"subnet"`
		CallbackURL url.URL   `query:"callback"`

		// Regex
		Pattern regexp.Regexp `query:"pattern"`

		// Maps
		Metadata map[string]string `query:"metadata"`
		Settings map[string]any    `query:"settings"`

		// Nested struct
		Address struct {
			Street string `query:"street"`
			City   string `query:"city"`
		} `query:"address"`

		// Enum validation
		Status string `query:"status" enum:"active,inactive"`

		// Slices of complex types
		Dates []time.Time `query:"dates"`
		IPs   []net.IP    `query:"ips"`
	}

	tests := []struct {
		name     string
		values   url.Values
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "all complex types",
			values: func() url.Values {
				v := url.Values{}
				// Basic
				v.Set("name", "John")
				v.Set("age", "30")
				// Time
				v.Set("start", "2024-01-15T10:00:00Z")
				v.Set("timeout", "30s")
				// Network
				v.Set("allowed_ip", "192.168.1.1")
				v.Set("subnet", "10.0.0.0/8")
				v.Set("callback", "https://example.com/hook")
				// Regex
				v.Set("pattern", `^\w+$`)
				// Maps
				v.Set("metadata.key1", "value1")
				v.Set("metadata.key2", "value2")
				v.Set("settings.debug", "true")
				// Nested struct
				v.Set("address.street", "Main St")
				v.Set("address.city", "NYC")
				// Enum
				v.Set("status", "active")
				// Slices
				v.Add("dates", "2024-01-15")
				v.Add("dates", "2024-01-16")
				v.Add("ips", "192.168.1.1")
				v.Add("ips", "10.0.0.1")

				return v
			}(),
			params: &ComplexParams{},
			validate: func(t *testing.T, params any) {
				p, ok := params.(*ComplexParams)
				require.True(t, ok)
				assert.Equal(t, "John", p.Name, "Name should match")
				assert.Equal(t, 30*time.Second, p.Timeout, "Timeout should be 30s")
				assert.Equal(t, "value1", p.Metadata["key1"], "metadata.key1 should match")
				assert.Equal(t, "Main St", p.Address.Street, "address.street should match")
				assert.Equal(t, "active", p.Status, "Status should match")
				require.Len(t, p.Dates, 2, "Dates should have 2 elements")
				require.Len(t, p.IPs, 2, "IPs should have 2 elements")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewQueryGetter(tt.values)
			err := Raw(getter, TagQuery, tt.params)

			require.NoError(t, err)
			tt.validate(t, tt.params)
		})
	}
}
