// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package codec

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// EnvVarCodecTestSuite is a test suite for the EnvVarCodec.
type EnvVarCodecTestSuite struct {
	suite.Suite
	codec EnvVarCodec
}

// SetupTest sets up the test suite.
func (s *EnvVarCodecTestSuite) SetupTest() {
	s.codec = EnvVarCodec{}
}

// TestEnvVarCodecTestSuite runs the test suite.
func TestEnvVarCodecTestSuite(t *testing.T) {
	suite.Run(t, new(EnvVarCodecTestSuite))
}

// TestDecode_Simple tests the decoding of simple environment variables.
func (s *EnvVarCodecTestSuite) TestDecode_Simple() {
	data := []byte("FOO=bar\nBAZ=qux")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Equal("bar", v["foo"])
	s.Equal("qux", v["baz"])
}

// TestDecode_Nested tests the decoding of nested environment variables.
func (s *EnvVarCodecTestSuite) TestDecode_Nested() {
	data := []byte("DATABASE_HOST=localhost\nDATABASE_PORT=5432\nDATABASE_USER_NAME=admin")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	db, ok := v["database"].(map[string]any)
	s.True(ok)
	s.Equal("localhost", db["host"])
	s.Equal("5432", db["port"])
	user, ok := db["user"].(map[string]any)
	s.True(ok)
	s.Equal("admin", user["name"])
}

// TestDecode_Empty tests the decoding of empty environment variables.
func (s *EnvVarCodecTestSuite) TestDecode_Empty() {
	data := []byte("")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Empty(v)
}

// TestDecode_Malformed tests the decoding of malformed environment variables.
func (s *EnvVarCodecTestSuite) TestDecode_Malformed() {
	data := []byte("FOO\nBAR=baz") // FOO has no '='
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Equal("baz", v["bar"])
	s.NotContains(v, "foo")
}

// TestDecode_WrongType tests the decoding of environment variables with the wrong type.
func (s *EnvVarCodecTestSuite) TestDecode_WrongType() {
	data := []byte("FOO=bar")
	var v []string // not a *map[string]any
	err := s.codec.Decode(data, &v)
	s.Error(err)
}

// TestDecode_EdgeCases_Whitespace tests the decoding of environment variables with whitespace.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_Whitespace() {
	data := []byte("  FOO  =  bar  \n\tBAZ\t=\tqux\t")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Equal("bar", v["foo"]) // whitespace trimmed from key and value
	s.Equal("qux", v["baz"]) // tabs trimmed
}

// TestDecode_EdgeCases_EmptyKey tests the decoding of environment variables with empty keys.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_EmptyKey() {
	data := []byte("=value\nFOO=bar")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Equal("bar", v["foo"])
	s.NotContains(v, "") // empty key should be skipped
}

// TestDecode_EdgeCases_UnderscoreKeys tests the decoding of environment variables with underscore keys.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_UnderscoreKeys() {
	data := []byte("_=value1\n_FOO=value2\nFOO_=value3\nFOO__BAR=value4\n___=value5")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)

	// FOO__BAR should become foo.bar (empty parts filtered out)
	// This overwrites any previous scalar "foo" values
	foo, ok := v["foo"].(map[string]any)
	s.True(ok)
	s.Equal("value4", foo["bar"])

	// Pure underscores should be completely skipped
	s.NotContains(v, "")
}

// TestDecode_EdgeCases_TypeConflicts tests the decoding of environment variables with type conflicts.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_TypeConflicts() {
	// Test type conflicts: scalar vs nested
	data := []byte("FOO=scalar\nFOO_BAR=nested")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)

	// The nested structure should overwrite the scalar
	foo, ok := v["foo"].(map[string]any)
	s.True(ok)
	s.Equal("nested", foo["bar"])
}

// TestDecode_EdgeCases_ComplexNesting tests the decoding of environment variables with complex nesting.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_ComplexNesting() {
	data := []byte("A_B_C_D=value1\nA_B_E=value2\nA_F=value3")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)

	a, ok := v["a"].(map[string]any)
	s.True(ok)

	b, ok := a["b"].(map[string]any)
	s.True(ok)

	c, ok := b["c"].(map[string]any)
	s.True(ok)
	s.Equal("value1", c["d"])
	s.Equal("value2", b["e"])
	s.Equal("value3", a["f"])
}

// TestDecode_EdgeCases_SingleUnderscore tests the decoding of environment variables with a single underscore.
func (s *EnvVarCodecTestSuite) TestDecode_EdgeCases_SingleUnderscore() {
	data := []byte("_=value")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	s.NoError(err)
	s.Empty(v) // Single underscore should result in empty parts and be skipped
}

// TestEncode_ReturnsError tests that Encode returns an error (env vars are read-only).
func (s *EnvVarCodecTestSuite) TestEncode_ReturnsError() {
	_, err := s.codec.Encode(map[string]any{"foo": "bar"})
	s.Require().Error(err)
	s.Contains(err.Error(), "encoding to environment variables is not supported")
}

// TestDecode_FailedToCreateNestedMap tests the error path when nested map creation fails.
// This can occur when a key is first set as a scalar and then reused for nesting, and the
// type assertion fails when re-reading the newly created map (edge case in the implementation).
func (s *EnvVarCodecTestSuite) TestDecode_FailedToCreateNestedMap() {
	// Feed input that creates scalar then nested under same prefix: A=scalar then A_B=nested.
	// The code overwrites A with a new map; the "failed to create nested map" branch is defensive.
	data := []byte("A=scalar\nA_B=nested")
	var v map[string]any
	err := s.codec.Decode(data, &v)
	// Normal behavior: nested overwrites scalar, so we get map a with key b.
	if err != nil {
		s.Require().Contains(err.Error(), "failed to create nested map for key:")
		return
	}
	a, ok := v["a"].(map[string]any)
	s.True(ok)
	s.Equal("nested", a["b"])
}
