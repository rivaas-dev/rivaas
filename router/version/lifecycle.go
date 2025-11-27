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

package version

import "time"

// LifecycleOption configures a specific version's lifecycle.
// These options are passed to r.Version("v1", opts...).
type LifecycleOption func(*LifecycleConfig)

// Deprecated marks this version as deprecated.
// The deprecation date is set to now.
//
// Example:
//
//	v1 := r.Version("v1", version.Deprecated())
func Deprecated() LifecycleOption {
	return func(lc *LifecycleConfig) {
		lc.Deprecated = true
		if lc.DeprecatedSince.IsZero() {
			lc.DeprecatedSince = time.Now()
		}
	}
}

// DeprecatedSince marks this version as deprecated since a specific date.
// Use this when the deprecation was announced in the past.
//
// Example:
//
//	v1 := r.Version("v1",
//	    version.DeprecatedSince(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
//	)
func DeprecatedSince(date time.Time) LifecycleOption {
	return func(lc *LifecycleConfig) {
		lc.Deprecated = true
		lc.DeprecatedSince = date
	}
}

// Sunset sets when this version will be removed.
// After this date, requests will receive 410 Gone (if EnforceSunset is enabled).
//
// Example:
//
//	v1 := r.Version("v1",
//	    version.Deprecated(),
//	    version.Sunset(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)),
//	)
func Sunset(date time.Time) LifecycleOption {
	return func(lc *LifecycleConfig) {
		lc.SunsetDate = date
	}
}

// MigrationDocs sets the URL for migration documentation.
// This URL is included in Link headers with rel=deprecation and rel=sunset.
//
// Example:
//
//	v1 := r.Version("v1",
//	    version.Deprecated(),
//	    version.MigrationDocs("https://docs.example.com/migrate/v1-to-v2"),
//	)
func MigrationDocs(url string) LifecycleOption {
	return func(lc *LifecycleConfig) {
		lc.MigrationURL = url
	}
}

// SuccessorVersion indicates which version users should migrate to.
// This is informational and included in deprecation headers.
//
// Example:
//
//	v1 := r.Version("v1",
//	    version.Deprecated(),
//	    version.SuccessorVersion("v2"),
//	)
func SuccessorVersion(v string) LifecycleOption {
	return func(lc *LifecycleConfig) {
		lc.Successor = v
	}
}

// ApplyLifecycleOptions applies lifecycle options to a LifecycleConfig.
func ApplyLifecycleOptions(opts ...LifecycleOption) *LifecycleConfig {
	lc := &LifecycleConfig{}
	for _, opt := range opts {
		opt(lc)
	}
	return lc
}
