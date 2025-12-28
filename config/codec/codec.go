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

// Type represents a codec type identifier.
type Type string

// Encoder converts Go values into encoded byte representations.
// Implementations must be safe for concurrent use.
type Encoder interface {
	// Encode converts the value v into an encoded byte slice.
	// It returns an error if encoding fails.
	Encode(v any) ([]byte, error)
}

// Decoder converts encoded byte representations into Go values.
// Implementations must be safe for concurrent use.
type Decoder interface {
	// Decode converts the encoded data into the value pointed to by v.
	// It returns an error if decoding fails or if v is not a valid target.
	Decode(data []byte, v any) error
}
