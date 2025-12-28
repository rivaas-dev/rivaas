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

package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"rivaas.dev/config/codec"
)

// extensionFormats maps file extensions to codec types for automatic format detection.
var extensionFormats = map[string]codec.Type{
	".yaml": codec.TypeYAML,
	".yml":  codec.TypeYAML,
	".json": codec.TypeJSON,
	".toml": codec.TypeTOML,
}

// detectFormat automatically detects the codec type based on the file extension.
// It returns an error if the format cannot be determined from the extension.
func detectFormat(path string) (codec.Type, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if format, ok := extensionFormats[ext]; ok {
		return format, nil
	}
	return "", fmt.Errorf("cannot detect format from extension %q; use WithFileAs() to specify format explicitly", ext)
}
