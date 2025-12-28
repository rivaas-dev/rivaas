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
package codec

import (
	"fmt"

	"github.com/spf13/cast"
)

// CastType is a string type that represents the type of a value that can be cast.
type CastType string

// revive:disable:exported
const (
	CastTypeBool       CastType = "bool"
	TypeCasterBool     Type     = "caster-bool"
	CastTypeTime       CastType = "time"
	TypeCasterTime     Type     = "caster-time"
	CastTypeDuration   CastType = "duration"
	TypeCasterDuration Type     = "caster-duration"
	CastTypeFloat64    CastType = "float64"
	TypeCasterFloat64  Type     = "caster-float64"
	CastTypeFloat32    CastType = "float32"
	TypeCasterFloat32  Type     = "caster-float32"
	CastTypeInt64      CastType = "int64"
	TypeCasterInt64    Type     = "caster-int64"
	CastTypeInt32      CastType = "int32"
	TypeCasterInt32    Type     = "caster-int32"
	CastTypeInt16      CastType = "int16"
	TypeCasterInt16    Type     = "caster-int16"
	CastTypeInt8       CastType = "int8"
	TypeCasterInt8     Type     = "caster-int8"
	CastTypeInt        CastType = "int"
	TypeCasterInt      Type     = "caster-int"
	CastTypeUint       CastType = "uint"
	TypeCasterUint     Type     = "caster-uint"
	CastTypeUint64     CastType = "uint64"
	TypeCasterUint64   Type     = "caster-uint64"
	CastTypeUint32     CastType = "uint32"
	TypeCasterUint32   Type     = "caster-uint32"
	CastTypeUint16     CastType = "uint16"
	TypeCasterUint16   Type     = "caster-uint16"
	CastTypeUint8      CastType = "uint8"
	TypeCasterUint8    Type     = "caster-uint8"
	CastTypeString     CastType = "string"
	TypeCasterString   Type     = "caster-string"
)

// init registers the various type casters with the codec package.
func init() {
	RegisterDecoder(TypeCasterBool, NewCaster(CastTypeBool))
	RegisterDecoder(TypeCasterTime, NewCaster(CastTypeTime))
	RegisterDecoder(TypeCasterDuration, NewCaster(CastTypeDuration))
	RegisterDecoder(TypeCasterFloat64, NewCaster(CastTypeFloat64))
	RegisterDecoder(TypeCasterFloat32, NewCaster(CastTypeFloat32))
	RegisterDecoder(TypeCasterInt64, NewCaster(CastTypeInt64))
	RegisterDecoder(TypeCasterInt32, NewCaster(CastTypeInt32))
	RegisterDecoder(TypeCasterInt16, NewCaster(CastTypeInt16))
	RegisterDecoder(TypeCasterInt8, NewCaster(CastTypeInt8))
	RegisterDecoder(TypeCasterInt, NewCaster(CastTypeInt))
	RegisterDecoder(TypeCasterUint, NewCaster(CastTypeUint))
	RegisterDecoder(TypeCasterUint64, NewCaster(CastTypeUint64))
	RegisterDecoder(TypeCasterUint32, NewCaster(CastTypeUint32))
	RegisterDecoder(TypeCasterUint16, NewCaster(CastTypeUint16))
	RegisterDecoder(TypeCasterUint8, NewCaster(CastTypeUint8))
	RegisterDecoder(TypeCasterString, NewCaster(CastTypeString))
}

// CasterCodec is a codec that casts the input data to a specific type.
// The castType field determines the type to which the data will be cast.
type CasterCodec struct {
	castType CastType
}

// NewCaster creates a new CasterCodec instance with the specified castType.
// The CasterCodec is used to cast input data to a specific type during decoding.
func NewCaster(castType CastType) *CasterCodec {
	return &CasterCodec{
		castType: castType,
	}
}

// Decode implements the Decoder interface for the CasterCodec. It takes the input data
// and casts it to the type specified by the castType field of the CasterCodec. The
// result is stored in the value pointed to by the v parameter.
func (c *CasterCodec) Decode(data []byte, v any) error {
	m, ok := v.(*any)
	if !ok {
		return fmt.Errorf("invalid type assertion")
	}
	value := string(data)

	var err error
	switch c.castType {
	case CastTypeBool:
		*m, err = cast.ToBoolE(value)
	case CastTypeTime:
		*m, err = cast.ToTimeE(value)
	case CastTypeDuration:
		*m, err = cast.ToDurationE(value)
	case CastTypeFloat64:
		*m, err = cast.ToFloat64E(value)
	case CastTypeFloat32:
		*m, err = cast.ToFloat32E(value)
	case CastTypeInt64:
		*m, err = cast.ToInt64E(value)
	case CastTypeInt32:
		*m, err = cast.ToInt32E(value)
	case CastTypeInt16:
		*m, err = cast.ToInt16E(value)
	case CastTypeInt8:
		*m, err = cast.ToInt8E(value)
	case CastTypeInt:
		*m, err = cast.ToIntE(value)
	case CastTypeUint:
		*m, err = cast.ToUintE(value)
	case CastTypeUint64:
		*m, err = cast.ToUint64E(value)
	case CastTypeUint32:
		*m, err = cast.ToUint32E(value)
	case CastTypeUint16:
		*m, err = cast.ToUint16E(value)
	case CastTypeUint8:
		*m, err = cast.ToUint8E(value)
	case CastTypeString:
		*m, err = cast.ToStringE(value)
	}

	return err
}
