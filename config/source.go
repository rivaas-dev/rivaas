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

package config

import "context"

// Source defines the interface for configuration sources.
// Implementations load configuration data from various locations
// such as files, environment variables, or remote services.
//
// Load must be safe to call concurrently.
type Source interface {
	// Load loads configuration data from the source.
	// It returns a map containing the configuration key-value pairs.
	// Keys are normalized to lowercase for case-insensitive access.
	Load(ctx context.Context) (map[string]any, error)
}

// Watcher defines the interface for watching configuration changes.
// Implementations monitor configuration sources for changes and
// notify when updates occur.
type Watcher interface {
	// Watch starts watching for changes to configuration data.
	// It blocks until the context is cancelled or an error occurs.
	Watch(ctx context.Context) error
}
