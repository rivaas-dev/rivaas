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

// Package source provides configuration source implementations.
//
// The source package implements the [Source] interface defined in the parent
// config package. Sources load configuration data from various locations such
// as files, environment variables, and remote services.
//
// # Available Sources
//
//   - File: Load configuration from files with various formats
//   - OSEnvVar: Load configuration from environment variables
//   - Consul: Load configuration from Consul key-value store
//
// # Example
//
// Creating a file source:
//
//	decoder, _ := codec.GetDecoder(codec.TypeYAML)
//	fileSource := source.NewFile("config.yaml", decoder)
//	config, err := fileSource.Load(context.Background())
//
// Creating an environment variable source:
//
//	envSource := source.NewOSEnvVar("APP_")
//	config, err := envSource.Load(context.Background())
package source
