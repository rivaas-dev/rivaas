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

// Package codec provides functionality for encoding and decoding data.
//go:build !integration

package codec

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// YAMLCodecTestSuite is a test suite for the YAMLCodec type.
type YAMLCodecTestSuite struct {
	suite.Suite
	codec YAMLCodec
}

// SetupTest sets up the test suite.
func (s *YAMLCodecTestSuite) SetupTest() {
	s.codec = YAMLCodec{}
}

// TestYAMLCodecTestSuite runs the YAMLCodecTestSuite.
func TestYAMLCodecTestSuite(t *testing.T) {
	suite.Run(t, new(YAMLCodecTestSuite))
}

// TestRegistration tests that the YAMLCodec is properly registered as both an
// encoder and decoder for the YAML data format.
func (s *YAMLCodecTestSuite) TestRegistration() {
	encoder, err := GetEncoder(TypeYAML)
	s.Require().NoError(err)
	s.Assert().IsType(YAMLCodec{}, encoder, "expected YAMLCodec, got %T", encoder)

	decoder, err := GetDecoder(TypeYAML)
	s.Require().NoError(err)
	s.Assert().IsType(YAMLCodec{}, decoder, "expected YAMLCodec, got %T", decoder)
}

func (s *YAMLCodecTestSuite) TestEncode() {
	data := map[string]any{"foo": "bar", "num": 42, "nested": map[string]any{"key": "value"}}
	b, err := s.codec.Encode(data)
	s.Require().NoError(err)
	// Basic check, YAML output can vary (e.g. order of keys)
	s.Assert().Contains(string(b), "foo: bar")
	s.Assert().Contains(string(b), "num: 42")
	s.Assert().Contains(string(b), "nested:")
	s.Assert().Contains(string(b), "  key: value")
}

func (s *YAMLCodecTestSuite) TestEncode_Empty() {
	b, err := s.codec.Encode(map[string]any{})
	s.Require().NoError(err)
	// gopkg.in/yaml.v3 marshals an empty map to "null\n" or "{}\n" depending on context
	// For consistency, we'll accept either, or just check for non-error and minimal output.
	// "{}\n" is a common representation. "null\n" can also occur.
	// Let's check if it's one of the expected empty representations.
	strOut := string(b)
	s.Assert().True(strOut == "{}\n" || strOut == "null\n" || strOut == "")
}

func (s *YAMLCodecTestSuite) TestEncode_Error() {
	// Channels are not directly serializable to YAML by default by gopkg.in/yaml.v3
	ch := make(chan int)
	_, err := s.codec.Encode(ch)
	s.Require().Error(err)
}

func (s *YAMLCodecTestSuite) TestDecode() {
	var v map[string]any
	yamlStr := `
foo: bar
num: 42
nested:
  key: value
`
	err := s.codec.Decode([]byte(yamlStr), &v)
	s.Require().NoError(err)
	s.Assert().Equal("bar", v["foo"])
	s.Assert().EqualValues(42, v["num"])
	s.Require().IsType(map[string]any{}, v["nested"])
	nestedMap, ok := v["nested"].(map[string]any)
	s.Require().True(ok, "nested should be map[string]any")
	s.Assert().Equal("value", nestedMap["key"])
}

func (s *YAMLCodecTestSuite) TestDecode_Empty() {
	var v map[string]any
	err := s.codec.Decode([]byte(`{}`), &v)
	s.Require().NoError(err)
	s.Assert().Empty(v)

	var v2 map[string]any
	err = s.codec.Decode([]byte(`null`), &v2)
	s.Assert().Error(err)
}

func (s *YAMLCodecTestSuite) TestDecode_Error() {
	var v map[string]any
	// Use a truly invalid YAML: unclosed quote
	err := s.codec.Decode([]byte("foo: \"bar"), &v)
	s.Assert().Error(err)
}

func (s *YAMLCodecTestSuite) TestDecode_IntoStruct() {
	type Config struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
	var cfg Config
	yamlStr := `
host: localhost
port: 8080
`
	err := s.codec.Decode([]byte(yamlStr), &cfg)
	s.Require().NoError(err)
	s.Assert().Equal("localhost", cfg.Host)
	s.Assert().Equal(8080, cfg.Port)
}

func (s *YAMLCodecTestSuite) TestDecode_NonPointer() {
	var v map[string]any
	err := s.codec.Decode([]byte(`foo: bar`), v) // not a pointer
	s.Assert().Error(err)
}

func (s *YAMLCodecTestSuite) TestDecode_NonMapOrStruct() {
	var v int
	err := s.codec.Decode([]byte(`42`), &v)
	s.Assert().NoError(err)
	s.Assert().Equal(42, v)

	var arr []string
	err = s.codec.Decode([]byte("- a\n- b"), &arr)
	s.Assert().NoError(err)
	s.Assert().Equal([]string{"a", "b"}, arr)
}

func (s *YAMLCodecTestSuite) TestEncodeDecode_SliceAndNestedStruct() {
	type Nested struct {
		Name string `yaml:"name"`
	}
	type Parent struct {
		Items []Nested `yaml:"items"`
	}
	in := Parent{Items: []Nested{{Name: "foo"}, {Name: "bar"}}}
	b, err := s.codec.Encode(in)
	s.Require().NoError(err)
	var out Parent
	err = s.codec.Decode(b, &out)
	s.Require().NoError(err)
	s.Assert().Equal(in, out)
}

func (s *YAMLCodecTestSuite) TestEncodeDecode_BoolFloatNull() {
	m := map[string]any{"b": true, "f": 3.14, "n": nil}
	b, err := s.codec.Encode(m)
	s.Require().NoError(err)
	var out map[string]any
	err = s.codec.Decode(b, &out)
	s.Require().NoError(err)
	s.Assert().Equal(true, out["b"])
	fVal, ok := out["f"].(float64)
	s.Require().True(ok, "f should be float64, got %T", out["f"])
	s.Assert().InDelta(3.14, fVal, 0.0001)
	s.Assert().Nil(out["n"])
}

func (s *YAMLCodecTestSuite) TestEncodeDecode_CustomType() {
	type Custom struct {
		Value string
	}
	in := Custom{Value: "custom"}
	b, err := s.codec.Encode(in)
	s.Require().NoError(err)
	var out Custom
	err = s.codec.Decode(b, &out)
	s.Require().NoError(err)
	s.Assert().Equal(in, out)
}

func (s *YAMLCodecTestSuite) TestDecode_ExtraFields() {
	type Target struct {
		Foo string `yaml:"foo"`
	}
	var t Target
	err := s.codec.Decode([]byte("foo: bar\nextra: ignored"), &t)
	s.Require().NoError(err)
	s.Assert().Equal("bar", t.Foo)
}

func (s *YAMLCodecTestSuite) TestDecode_TypeMismatch() {
	type Target struct {
		Num int `yaml:"num"`
	}
	var t Target
	err := s.codec.Decode([]byte("num: notanint"), &t)
	s.Assert().Error(err)
}

func (s *YAMLCodecTestSuite) TestEncodeDecode_UnicodeSpecialChars() {
	m := map[string]any{"emoji": "😀", "special": "\u2603\nnewline\n"}
	b, err := s.codec.Encode(m)
	s.Require().NoError(err)
	var out map[string]any
	err = s.codec.Decode(b, &out)
	s.Require().NoError(err)
	s.Assert().Equal(m["emoji"], out["emoji"])
	s.Assert().Equal(m["special"], out["special"])
}

func (s *YAMLCodecTestSuite) TestDecode_DeeplyNested() {
	yamlStr := "a:\n  b:\n    c:\n      d:\n        e: 1"
	var v map[string]any
	err := s.codec.Decode([]byte(yamlStr), &v)
	s.Require().NoError(err)
	// Walk down the nested structure
	a, okA := v["a"].(map[string]any)
	s.Require().True(okA, "a should be map[string]any")
	b, okB := a["b"].(map[string]any)
	s.Require().True(okB, "b should be map[string]any")
	c, okC := b["c"].(map[string]any)
	s.Require().True(okC, "c should be map[string]any")
	d, okD := c["d"].(map[string]any)
	s.Require().True(okD, "d should be map[string]any")
	s.Assert().EqualValues(1, d["e"])
}
