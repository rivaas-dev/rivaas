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
	"bytes"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindJSON_BasicTypes tests binding basic JSON data
func TestBindJSON_BasicTypes(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	body := []byte(`{"name":"John","email":"john@example.com","age":30}`)

	var user User
	err := JSONTo(body, &user)

	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

// TestBindJSON_NestedStructs tests binding nested JSON structures
func TestBindJSON_NestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type User struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	body := []byte(`{
		"name":"Alice",
		"address":{"street":"123 Main St","city":"NYC"}
	}`)

	var user User
	err := JSONTo(body, &user)

	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "123 Main St", user.Address.Street)
	assert.Equal(t, "NYC", user.Address.City)
}

// TestBindJSON_Arrays tests binding JSON arrays
func TestBindJSON_Arrays(t *testing.T) {
	t.Parallel()

	type Data struct {
		Tags []string `json:"tags"`
		IDs  []int    `json:"ids"`
	}

	body := []byte(`{"tags":["go","rust","python"],"ids":[1,2,3]}`)

	var data Data
	err := JSONTo(body, &data)

	require.NoError(t, err)
	assert.Equal(t, []string{"go", "rust", "python"}, data.Tags)
	assert.Equal(t, []int{1, 2, 3}, data.IDs)
}

// TestBindJSON_ErrorCases tests JSON binding error scenarios
func TestBindJSON_ErrorCases(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		body    []byte
		wantErr bool
	}{
		{
			name:    "malformed JSON",
			body:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    []byte(``),
			wantErr: true,
		},
		{
			name:    "type mismatch",
			body:    []byte(`{"name":"John","age":"not-a-number"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var user User
			err := JSONTo(tt.body, &user)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBindJSONStrict_UnknownFields tests strict JSON binding
func TestBindJSONStrict_UnknownFields(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name    string
		body    []byte
		wantErr bool
	}{
		{
			name:    "known fields only",
			body:    []byte(`{"name":"John","email":"john@example.com"}`),
			wantErr: false,
		},
		{
			name:    "unknown field present",
			body:    []byte(`{"name":"John","email":"john@example.com","unknown":"field"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var user User
			err := JSONTo(tt.body, &user, WithUnknownFields(UnknownError))

			if tt.wantErr {
				require.Error(t, err)
				var unknownErr *UnknownFieldError
				assert.ErrorAs(t, err, &unknownErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBindJSONInto_Generic tests generic JSON binding helper
func TestBindJSONInto_Generic(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := []byte(`{"name":"Jane","age":25}`)

	user, err := JSON[User](body)

	require.NoError(t, err)
	assert.Equal(t, "Jane", user.Name)
	assert.Equal(t, 25, user.Age)
}

// TestJSONReader tests binding JSON from io.Reader.
func TestJSONReader(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := []byte(`{"name":"reader-user","age":33}`)
	result, err := JSONReader[User](bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "reader-user", result.Name)
	assert.Equal(t, 33, result.Age)
}

// TestJSONReaderTo tests binding JSON from io.Reader into out.
func TestJSONReaderTo(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := []byte(`{"name":"reader-to-user","age":44}`)
	var out User
	err := JSONReaderTo(bytes.NewReader(body), &out)
	require.NoError(t, err)
	assert.Equal(t, "reader-to-user", out.Name)
	assert.Equal(t, 44, out.Age)
}

// TestJSONReader_ErrorCase tests error path for JSON reader binding.
func TestJSONReader_ErrorCase(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
	}

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		_, err := JSONReader[User](bytes.NewReader([]byte("invalid")))
		require.Error(t, err)
	})

	t.Run("empty reader returns error", func(t *testing.T) {
		t.Parallel()

		var out User
		err := JSONReaderTo(bytes.NewReader(nil), &out)
		require.Error(t, err)
	})
}

// TestBindJSON_UnknownWarn tests UnknownWarn policy: unknown fields are collected in Result.Unknown but binding succeeds.
func TestBindJSON_UnknownWarn(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	body := []byte(`{"name":"John","email":"john@example.com","unknown":"x"}`)
	var result Result
	var out User
	err := JSONTo(body, &out,
		WithUnknownFields(UnknownWarn),
		WithResult(&result))
	require.NoError(t, err)
	assert.Equal(t, "John", out.Name)
	assert.Equal(t, "john@example.com", out.Email)
	require.Len(t, result.Unknown, 1)
	assert.Equal(t, "unknown", result.Unknown[0])
}

// TestBindJSON_UnknownWarn_Nested tests UnknownWarn with nested struct and unknown field at nested level.
func TestBindJSON_UnknownWarn_Nested(t *testing.T) {
	t.Parallel()

	type Address struct {
		City string `json:"city"`
	}
	type User struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	body := []byte(`{"name":"Alice","address":{"city":"NYC","unknown_nested":"y"}}`)
	var result Result
	var out User
	err := JSONTo(body, &out,
		WithUnknownFields(UnknownWarn),
		WithResult(&result))
	require.NoError(t, err)
	assert.Equal(t, "Alice", out.Name)
	assert.Equal(t, "NYC", out.Address.City)
	require.Len(t, result.Unknown, 1)
	assert.Equal(t, "address.unknown_nested", result.Unknown[0])
}

// ---------------------------------------------------------------------------
// JSON default:"..." battle tests
// ---------------------------------------------------------------------------

// TestBindJSON_Defaults_Scalars tests that default values are applied to scalar
// fields when they are absent from the JSON payload.
func TestBindJSON_Defaults_Scalars(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Name string `json:"name" default:"hello"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, "hello", p.Name)
	})

	t.Run("int", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Count int `json:"count" default:"42"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, 42, p.Count)
	})

	t.Run("int8", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V int8 `json:"v" default:"127"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, int8(127), p.V)
	})

	t.Run("int16", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V int16 `json:"v" default:"-100"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, int16(-100), p.V)
	})

	t.Run("int32", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V int32 `json:"v" default:"100000"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, int32(100000), p.V)
	})

	t.Run("int64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V int64 `json:"v" default:"9999999"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, int64(9999999), p.V)
	})

	t.Run("uint", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V uint `json:"v" default:"10"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, uint(10), p.V)
	})

	t.Run("uint8", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V uint8 `json:"v" default:"255"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, uint8(255), p.V)
	})

	t.Run("uint16", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V uint16 `json:"v" default:"1000"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, uint16(1000), p.V)
	})

	t.Run("uint32", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V uint32 `json:"v" default:"70000"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, uint32(70000), p.V)
	})

	t.Run("uint64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V uint64 `json:"v" default:"123456789"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, uint64(123456789), p.V)
	})

	t.Run("float32", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V float32 `json:"v" default:"3.14"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.InDelta(t, float32(3.14), p.V, 0.001)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			V float64 `json:"v" default:"2.718"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.InDelta(t, 2.718, p.V, 0.0001)
	})

	t.Run("bool true", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Active bool `json:"active" default:"true"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.True(t, p.Active)
	})

	t.Run("bool false", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Active bool `json:"active" default:"false"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.False(t, p.Active)
	})

	t.Run("time.Time", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Created time.Time `json:"created" default:"2024-01-01T00:00:00Z"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		assert.True(t, expected.Equal(p.Created), "expected %v, got %v", expected, p.Created)
	})

	t.Run("time.Duration", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Timeout time.Duration `json:"timeout" default:"5s"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, p.Timeout)
	})

	t.Run("time.Duration complex", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Timeout time.Duration `json:"timeout" default:"1h30m"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, 90*time.Minute, p.Timeout)
	})

	t.Run("net.IP", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Host net.IP `json:"host" default:"192.168.1.1"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, net.ParseIP("192.168.1.1"), p.Host)
	})

	t.Run("url.URL", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Endpoint url.URL `json:"endpoint" default:"https://example.com"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, "https://example.com", p.Endpoint.String())
	})
}

// TestBindJSON_Defaults_Pointers tests defaults for pointer fields.
func TestBindJSON_Defaults_Pointers(t *testing.T) {
	t.Parallel()

	t.Run("*string", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Name *string `json:"name" default:"hello"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Name)
		assert.Equal(t, "hello", *p.Name)
	})

	t.Run("*int", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Age *int `json:"age" default:"25"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Age)
		assert.Equal(t, 25, *p.Age)
	})

	t.Run("*int64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Count *int64 `json:"count" default:"999"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Count)
		assert.Equal(t, int64(999), *p.Count)
	})

	t.Run("*uint", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Limit *uint `json:"limit" default:"7"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Limit)
		assert.Equal(t, uint(7), *p.Limit)
	})

	t.Run("*float64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Price *float64 `json:"price" default:"3.14"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Price)
		assert.Equal(t, 3.14, *p.Price) //nolint:testifylint // exact decimal comparison
	})

	t.Run("*bool true", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Active *bool `json:"active" default:"true"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Active)
		assert.True(t, *p.Active)
	})

	t.Run("*bool false distinguishes from nil", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Active *bool `json:"active" default:"false"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.Active, "*bool with default:\"false\" must not be nil")
		assert.False(t, *p.Active)
	})

	t.Run("*time.Duration", func(t *testing.T) {
		t.Parallel()
		type P struct {
			TTL *time.Duration `json:"ttl" default:"10m"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		require.NotNil(t, p.TTL)
		assert.Equal(t, 10*time.Minute, *p.TTL)
	})
}

// TestBindJSON_Defaults_Slices tests defaults for slice fields.
func TestBindJSON_Defaults_Slices(t *testing.T) {
	t.Parallel()

	t.Run("[]string", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Tags []string `json:"tags" default:"a,b,c"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, p.Tags)
	})

	t.Run("[]string single element", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Tags []string `json:"tags" default:"only"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"only"}, p.Tags)
	})

	t.Run("[]string with spaces trimmed", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Tags []string `json:"tags" default:" a , b , c "`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, p.Tags)
	})

	t.Run("[]int", func(t *testing.T) {
		t.Parallel()
		type P struct {
			IDs []int `json:"ids" default:"1,2,3"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, p.IDs)
	})

	t.Run("[]int64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			IDs []int64 `json:"ids" default:"100,200"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []int64{100, 200}, p.IDs)
	})

	t.Run("[]uint", func(t *testing.T) {
		t.Parallel()
		type P struct {
			IDs []uint `json:"ids" default:"1,2,3"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []uint{1, 2, 3}, p.IDs)
	})

	t.Run("[]float64", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Scores []float64 `json:"scores" default:"1.1,2.2,3.3"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []float64{1.1, 2.2, 3.3}, p.Scores) //nolint:testifylint // exact decimal comparison
	})

	t.Run("[]bool", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Flags []bool `json:"flags" default:"true,false,true"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []bool{true, false, true}, p.Flags)
	})

	t.Run("[]time.Duration", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Timeouts []time.Duration `json:"timeouts" default:"1s,2m,3h"`
		}
		p, err := JSON[P]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, []time.Duration{1 * time.Second, 2 * time.Minute, 3 * time.Hour}, p.Timeouts)
	})
}

// TestBindJSON_Defaults_ExplicitZeroOverride verifies that explicit zero values
// in the JSON payload are NOT overwritten by defaults.
func TestBindJSON_Defaults_ExplicitZeroOverride(t *testing.T) {
	t.Parallel()

	t.Run("bool false stays false", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Active bool `json:"active" default:"true"`
		}
		p, err := JSON[P]([]byte(`{"active":false}`))
		require.NoError(t, err)
		assert.False(t, p.Active)
	})

	t.Run("int 0 stays 0", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Count int `json:"count" default:"42"`
		}
		p, err := JSON[P]([]byte(`{"count":0}`))
		require.NoError(t, err)
		assert.Equal(t, 0, p.Count)
	})

	t.Run("string empty stays empty", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Name string `json:"name" default:"hello"`
		}
		p, err := JSON[P]([]byte(`{"name":""}`))
		require.NoError(t, err)
		assert.Equal(t, "", p.Name)
	})

	t.Run("float64 0 stays 0", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Score float64 `json:"score" default:"9.99"`
		}
		p, err := JSON[P]([]byte(`{"score":0.0}`))
		require.NoError(t, err)
		assert.Equal(t, 0.0, p.Score) //nolint:testifylint // exact comparison
	})

	t.Run("empty slice stays empty", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Tags []string `json:"tags" default:"a,b"`
		}
		p, err := JSON[P]([]byte(`{"tags":[]}`))
		require.NoError(t, err)
		assert.Empty(t, p.Tags)
	})

	t.Run("null pointer stays nil", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Age *int `json:"age" default:"25"`
		}
		p, err := JSON[P]([]byte(`{"age":null}`))
		require.NoError(t, err)
		assert.Nil(t, p.Age)
	})

	t.Run("provided value wins", func(t *testing.T) {
		t.Parallel()
		type P struct {
			Count int     `json:"count" default:"42"`
			Name  string  `json:"name" default:"hello"`
			Price float64 `json:"price" default:"9.99"`
		}
		p, err := JSON[P]([]byte(`{"count":7,"name":"world","price":1.5}`))
		require.NoError(t, err)
		assert.Equal(t, 7, p.Count)
		assert.Equal(t, "world", p.Name)
		assert.Equal(t, 1.5, p.Price) //nolint:testifylint // exact comparison
	})
}

// TestBindJSON_Defaults_Nested tests defaults for fields inside nested structs.
func TestBindJSON_Defaults_Nested(t *testing.T) {
	t.Parallel()

	t.Run("nested field gets default", func(t *testing.T) {
		t.Parallel()
		type Address struct {
			City    string `json:"city" default:"Unknown"`
			Country string `json:"country" default:"US"`
		}
		type User struct {
			Name    string  `json:"name" default:"anon"`
			Address Address `json:"address"`
		}
		p, err := JSON[User]([]byte(`{"address":{"city":"NYC"}}`))
		require.NoError(t, err)
		assert.Equal(t, "anon", p.Name)
		assert.Equal(t, "NYC", p.Address.City)
		assert.Equal(t, "US", p.Address.Country)
	})

	t.Run("nested struct absent entirely", func(t *testing.T) {
		t.Parallel()
		type Address struct {
			City string `json:"city" default:"Unknown"`
		}
		type User struct {
			Name    string  `json:"name" default:"anon"`
			Address Address `json:"address"`
		}
		p, err := JSON[User]([]byte(`{}`))
		require.NoError(t, err)
		assert.Equal(t, "anon", p.Name)
		// When the parent struct key is absent, encoding/json does not touch it,
		// and there is no raw map to recurse into, so nested defaults are not applied.
		assert.Equal(t, "", p.Address.City)
	})
}

// TestBindJSON_Defaults_Partial tests a mix of provided and defaulted fields.
func TestBindJSON_Defaults_Partial(t *testing.T) {
	t.Parallel()

	type Config struct {
		Host    string        `json:"host" default:"localhost"`
		Port    int           `json:"port" default:"8080"`
		Debug   bool          `json:"debug" default:"true"`
		Timeout time.Duration `json:"timeout" default:"30s"`
		Tags    []string      `json:"tags" default:"prod,main"`
	}

	p, err := JSON[Config]([]byte(`{"host":"api.example.com","debug":false}`))
	require.NoError(t, err)
	assert.Equal(t, "api.example.com", p.Host)
	assert.Equal(t, 8080, p.Port)
	assert.False(t, p.Debug)
	assert.Equal(t, 30*time.Second, p.Timeout)
	assert.Equal(t, []string{"prod", "main"}, p.Tags)
}

// TestBindJSON_Defaults_EmptyObject tests that an empty JSON {} triggers all defaults.
func TestBindJSON_Defaults_EmptyObject(t *testing.T) {
	t.Parallel()

	type AllDefaults struct {
		Name    string        `json:"name" default:"default_name"`
		Count   int           `json:"count" default:"99"`
		Active  bool          `json:"active" default:"true"`
		Rate    float64       `json:"rate" default:"0.5"`
		Timeout time.Duration `json:"timeout" default:"1m"`
		Tags    []string      `json:"tags" default:"x,y"`
	}

	p, err := JSON[AllDefaults]([]byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "default_name", p.Name)
	assert.Equal(t, 99, p.Count)
	assert.True(t, p.Active)
	assert.Equal(t, 0.5, p.Rate) //nolint:testifylint // exact comparison
	assert.Equal(t, 1*time.Minute, p.Timeout)
	assert.Equal(t, []string{"x", "y"}, p.Tags)
}

// TestBindJSON_Defaults_FullPayload tests that when all fields are present, no defaults are applied.
func TestBindJSON_Defaults_FullPayload(t *testing.T) {
	t.Parallel()

	type P struct {
		Name   string `json:"name" default:"fallback"`
		Count  int    `json:"count" default:"99"`
		Active bool   `json:"active" default:"true"`
	}

	body := []byte(`{"name":"Alice","count":1,"active":false}`)
	p, err := JSON[P](body)
	require.NoError(t, err)
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, 1, p.Count)
	assert.False(t, p.Active)
}

// TestBindJSON_Defaults_ReaderPath tests that defaults work via JSONReaderTo.
func TestBindJSON_Defaults_ReaderPath(t *testing.T) {
	t.Parallel()

	type P struct {
		Name    string   `json:"name" default:"anon"`
		Count   int      `json:"count" default:"42"`
		Tags    []string `json:"tags" default:"a,b"`
		Active  bool     `json:"active" default:"true"`
		Missing *int     `json:"missing" default:"7"`
	}

	body := bytes.NewReader([]byte(`{"name":"Bob"}`))
	var p P
	err := JSONReaderTo(body, &p)
	require.NoError(t, err)
	assert.Equal(t, "Bob", p.Name)
	assert.Equal(t, 42, p.Count)
	assert.Equal(t, []string{"a", "b"}, p.Tags)
	assert.True(t, p.Active)
	require.NotNil(t, p.Missing)
	assert.Equal(t, 7, *p.Missing)
}

// TestBindJSON_Defaults_WithUnknownError tests defaults work alongside strict mode.
func TestBindJSON_Defaults_WithUnknownError(t *testing.T) {
	t.Parallel()

	type P struct {
		Name  string `json:"name" default:"anon"`
		Count int    `json:"count" default:"42"`
	}

	p, err := JSON[P]([]byte(`{"name":"Alice"}`), WithUnknownFields(UnknownError))
	require.NoError(t, err)
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, 42, p.Count)
}

// TestBindJSON_Defaults_WithUnknownWarn tests defaults work alongside warn mode.
func TestBindJSON_Defaults_WithUnknownWarn(t *testing.T) {
	t.Parallel()

	type P struct {
		Name  string `json:"name" default:"anon"`
		Count int    `json:"count" default:"42"`
	}

	var result Result
	p, err := JSON[P]([]byte(`{"extra":"x"}`),
		WithUnknownFields(UnknownWarn),
		WithResult(&result))
	require.NoError(t, err)
	assert.Equal(t, "anon", p.Name)
	assert.Equal(t, 42, p.Count)
	assert.Contains(t, result.Unknown, "extra")
}

// TestBindJSON_Defaults_NoDefaultsStruct verifies no overhead for structs without defaults.
func TestBindJSON_Defaults_NoDefaultsStruct(t *testing.T) {
	t.Parallel()

	type Plain struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	p, err := JSON[Plain]([]byte(`{"name":"Alice","age":30}`))
	require.NoError(t, err)
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, 30, p.Age)
}

// TestBindJSON_Defaults_JSONTo tests defaults via the JSONTo function (pointer variant).
func TestBindJSON_Defaults_JSONTo(t *testing.T) {
	t.Parallel()

	type P struct {
		Host string `json:"host" default:"localhost"`
		Port int    `json:"port" default:"3000"`
	}

	var p P
	err := JSONTo([]byte(`{"host":"db.internal"}`), &p)
	require.NoError(t, err)
	assert.Equal(t, "db.internal", p.Host)
	assert.Equal(t, 3000, p.Port)
}
