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

// Package dumper provides configuration dumper implementations.
//
// The dumper package implements the [Dumper] interface defined in the parent
// config package. Dumpers write configuration data to various destinations
// such as files or remote services.
//
// # Available Dumpers
//
//   - File: Write configuration to files with various formats
//
// # Example
//
// Creating a file dumper:
//
//	encoder, _ := codec.GetEncoder(codec.TypeYAML)
//	fileDumper := dumper.NewFile("output.yaml", encoder)
//	err := fileDumper.Dump(context.Background(), &configMap)
//
// Creating a file dumper with custom permissions:
//
//	fileDumper := dumper.NewFileWithPermissions("output.yaml", encoder, 0600)
package dumper
