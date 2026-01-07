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

package dumper

import (
	"context"
	"fmt"
	"os"

	"rivaas.dev/config/codec"
)

// File represents a configuration dumper that writes data to a file.
// It supports customizable file permissions and uses encoders to
// convert configuration data to the appropriate format.
type File struct {
	path        string
	encoder     codec.Encoder
	permissions os.FileMode
}

const (
	// DefaultFilePermissions represents the default file permissions for dumped configuration files.
	// Files are created with read/write permissions for the owner and read permissions for group and others (0644).
	DefaultFilePermissions = 0o644
)

// NewFile creates a new File dumper that writes configuration to the specified file path.
// It uses default file permissions of 0644.
// The encoder parameter determines how the configuration data is formatted.
func NewFile(path string, encoder codec.Encoder) *File {
	return &File{
		path:        path,
		encoder:     encoder,
		permissions: DefaultFilePermissions,
	}
}

// NewFileWithPermissions creates a new File dumper with custom file permissions.
// This allows control over the file security and access rights.
// Use this when you need more restrictive permissions (e.g., 0600 for sensitive configuration).
func NewFileWithPermissions(path string, encoder codec.Encoder, permissions os.FileMode) *File {
	return &File{
		path:        path,
		encoder:     encoder,
		permissions: permissions,
	}
}

// Dump writes the provided configuration values to the file.
// It encodes the values using the configured encoder and writes them atomically.
//
// Errors:
//   - Returns error if encoding fails
//   - Returns error if writing to the file fails
func (f *File) Dump(_ context.Context, values *map[string]any) error {
	data, err := f.encoder.Encode(values)
	if err != nil {
		return fmt.Errorf("failed to encode values: %w", err)
	}

	if err = os.WriteFile(f.path, data, f.permissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
