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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CasterCodecTestSuite struct {
	suite.Suite
}

func TestCasterCodecTestSuite(t *testing.T) {
	suite.Run(t, new(CasterCodecTestSuite))
}

func (s *CasterCodecTestSuite) TestDecode_Bool() {
	codec := NewCaster(CastTypeBool)
	var v any
	err := codec.Decode([]byte("true"), &v)
	s.NoError(err)
	s.Equal(true, v)
}

func (s *CasterCodecTestSuite) TestDecode_Int() {
	codec := NewCaster(CastTypeInt)
	var v any
	err := codec.Decode([]byte("42"), &v)
	s.NoError(err)
	s.Equal(42, v)
}

func (s *CasterCodecTestSuite) TestDecode_Int64() {
	codec := NewCaster(CastTypeInt64)
	var v any
	err := codec.Decode([]byte("1234567890"), &v)
	s.NoError(err)
	s.Equal(int64(1234567890), v)
}

func (s *CasterCodecTestSuite) TestDecode_Float64() {
	codec := NewCaster(CastTypeFloat64)
	var v any
	err := codec.Decode([]byte("3.14"), &v)
	s.NoError(err)
	s.Equal(3.14, v)
}

func (s *CasterCodecTestSuite) TestDecode_String() {
	codec := NewCaster(CastTypeString)
	var v any
	err := codec.Decode([]byte("hello"), &v)
	s.NoError(err)
	s.Equal("hello", v)
}

func (s *CasterCodecTestSuite) TestDecode_Time() {
	codec := NewCaster(CastTypeTime)
	var v any
	t := time.Now().UTC().Truncate(time.Second)
	err := codec.Decode([]byte(t.Format(time.RFC3339)), &v)
	s.NoError(err)
	s.Equal(t, v)
}

func (s *CasterCodecTestSuite) TestDecode_Duration() {
	codec := NewCaster(CastTypeDuration)
	var v any
	err := codec.Decode([]byte("1h2m3s"), &v)
	s.NoError(err)
	s.Equal(1*time.Hour+2*time.Minute+3*time.Second, v)
}

func (s *CasterCodecTestSuite) TestDecode_Error_InvalidType() {
	codec := NewCaster(CastTypeInt)
	var v int // not *any
	err := codec.Decode([]byte("42"), &v)
	s.Error(err)
}

func (s *CasterCodecTestSuite) TestDecode_Error_InvalidValue() {
	codec := NewCaster(CastTypeInt)
	var v any
	err := codec.Decode([]byte("notanint"), &v)
	s.Error(err)
}

func (s *CasterCodecTestSuite) TestDecode_AllTypes() {
	types := []struct {
		castType CastType
		input    string
		want     any
	}{
		{CastTypeBool, "true", true},
		{CastTypeInt, "42", 42},
		{CastTypeInt64, "42", int64(42)},
		{CastTypeInt32, "42", int32(42)},
		{CastTypeInt16, "42", int16(42)},
		{CastTypeInt8, "42", int8(42)},
		{CastTypeUint, "42", uint(42)},
		{CastTypeUint64, "42", uint64(42)},
		{CastTypeUint32, "42", uint32(42)},
		{CastTypeUint16, "42", uint16(42)},
		{CastTypeUint8, "42", uint8(42)},
		{CastTypeFloat64, "42.5", 42.5},
		{CastTypeFloat32, "42.5", float32(42.5)},
		{CastTypeString, "foo", "foo"},
	}
	for _, tt := range types {
		codec := NewCaster(tt.castType)
		var v any
		err := codec.Decode([]byte(tt.input), &v)
		s.NoError(err, "type %v", tt.castType)
		// For float32, compare as string to avoid precision issues
		if f32, ok := tt.want.(float32); ok {
			s.Equal(strconv.FormatFloat(float64(f32), 'f', 1, 32), strconv.FormatFloat(float64(v.(float32)), 'f', 1, 32))
		} else {
			s.Equal(tt.want, v, "type %v", tt.castType)
		}
	}
}
