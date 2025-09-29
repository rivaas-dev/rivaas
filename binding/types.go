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
	"encoding"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"time"
)

// Metadata tracks binding state for framework integration.
// It is used by router.Context to cache body reads and presence maps.
type Metadata struct {
	BodyRead bool   // Whether the request body has been read
	RawBody  []byte // Cached raw body bytes
}

// fieldInfo stores cached information about a struct field.
// It contains parsed tag information, type metadata, and validation rules
// that are computed once and reused across binding operations.
type fieldInfo struct {
	index           []int        // Field index path (supports nested structs)
	name            string       // Struct field name
	tagName         string       // Primary tag value (e.g., "user_id" from `query:"user_id"`)
	aliases         []string     // Additional lookup names (e.g., ["id"] from `query:"user_id,id"`)
	kind            reflect.Kind // Field type
	fieldType       reflect.Type // Full type information
	isPtr           bool         // Whether field is a pointer type
	isSlice         bool         // Whether field is a slice type
	isMap           bool         // Whether field is a map type
	isStruct        bool         // Whether field is a nested struct
	elemKind        reflect.Kind // Element type for slices
	enumValues      string       // Comma-separated enum values from tag
	defaultValue    string       // Raw default value from tag
	typedDefault    any          // Converted default value (nil if invalid or not set)
	hasTypedDefault bool         // Whether typedDefault is valid
}

// structInfo holds cached parsing information for a struct type.
// It contains the list of fields with their binding metadata for a given tag type.
type structInfo struct {
	fields []fieldInfo
}

// Type references for special type handling.
var (
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	timeType            = reflect.TypeFor[time.Time]()
	durationType        = reflect.TypeFor[time.Duration]()
	urlType             = reflect.TypeFor[url.URL]()
	ipType              = reflect.TypeFor[net.IP]()
	ipNetType           = reflect.TypeFor[net.IPNet]()
	regexpType          = reflect.TypeFor[regexp.Regexp]()
)
