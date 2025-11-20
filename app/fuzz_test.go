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

package app

import (
	"errors"
	"math"
	"testing"
	"time"
)

func FuzzConfigValidation(f *testing.F) {
	// Seed corpus
	f.Add("my-service", "1.0.0", "development")
	f.Add("", "v2.3.4", "production")
	f.Add("app", "", "staging")

	f.Fuzz(func(t *testing.T, name, version, env string) {
		// Should never panic, even with invalid input
		_, err := New(
			WithServiceName(name),
			WithServiceVersion(version),
			WithEnvironment(env),
		)

		// Either succeeds or returns structured error
		if err != nil {
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("expected ValidationError, got %T: %v", err, err)
			}
		}
	})
}

func FuzzServerTimeouts(f *testing.F) {
	// Seed with interesting timeout values
	f.Add(int64(1000000000))    // 1 second
	f.Add(int64(-1))            // negative
	f.Add(int64(0))             // zero
	f.Add(int64(math.MaxInt64)) // max

	f.Fuzz(func(t *testing.T, nanos int64) {
		duration := time.Duration(nanos)

		_, err := New(
			WithServiceName("fuzz-test"),
			WithServiceVersion("1.0.0"),
			WithServerConfig(
				WithReadTimeout(duration),
				WithWriteTimeout(duration*2),
			),
		)

		// Should handle any duration gracefully
		if err != nil {
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("expected ValidationError, got %T: %v", err, err)
			}
		}
	})
}

func FuzzServiceName(f *testing.F) {
	// Seed with various inputs
	f.Add("valid-service")
	f.Add("")
	f.Add("service-with-123-numbers")
	f.Add("service_with_underscores")

	f.Fuzz(func(t *testing.T, name string) {
		_, err := New(
			WithServiceName(name),
			WithServiceVersion("1.0.0"),
		)

		// Should either succeed or return ValidationError
		if err != nil {
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("expected ValidationError, got %T: %v", err, err)
			}
		}
	})
}
