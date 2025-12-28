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

package codec

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// TOMLCodecTestSuite is a test suite for TOMLCodec.
type TOMLCodecTestSuite struct {
	suite.Suite
	codec TOMLCodec
}

// SetupTest sets up the test suite.
func (s *TOMLCodecTestSuite) SetupTest() {
	s.codec = TOMLCodec{}
}

// TestTOMLCodecTestSuite runs the TOMLCodecTestSuite.
func TestTOMLCodecTestSuite(t *testing.T) {
	suite.Run(t, new(TOMLCodecTestSuite))
}

func (s *TOMLCodecTestSuite) TestEncode() {
	data := map[string]any{"foo": "bar", "num": 42}
	b, err := s.codec.Encode(data)
	s.NoError(err)
	s.Contains(string(b), "foo")
	s.Contains(string(b), "bar")
	s.Contains(string(b), "num")
}

func (s *TOMLCodecTestSuite) TestEncode_Empty() {
	b, err := s.codec.Encode(map[string]any{})
	s.NoError(err)
	s.Equal("", string(b))
}

func (s *TOMLCodecTestSuite) TestEncode_Error() {
	ch := make(chan int) // not serializable
	_, err := s.codec.Encode(ch)
	s.Error(err)
}

func (s *TOMLCodecTestSuite) TestDecode() {
	var v map[string]any
	tomlStr := `foo = "bar"
num = 42`
	err := s.codec.Decode([]byte(tomlStr), &v)
	s.NoError(err)
	s.Equal("bar", v["foo"])
	s.EqualValues(42, v["num"])
}

func (s *TOMLCodecTestSuite) TestDecode_Empty() {
	var v map[string]any
	err := s.codec.Decode([]byte(``), &v)
	s.NoError(err)
	s.Empty(v)
}

func (s *TOMLCodecTestSuite) TestDecode_Error() {
	var v map[string]any
	err := s.codec.Decode([]byte(`foo = [`), &v) // invalid TOML
	s.Error(err)
}
