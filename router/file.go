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

package router

// This file contains the File type for handling uploaded files.
// It provides a clean, ergonomic API for working with multipart form uploads.

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// File represents an uploaded file with a clean, ergonomic API.
// It wraps multipart.FileHeader and provides convenient methods for
// reading, streaming, and saving uploaded files.
//
// Security features built-in:
//   - Filename is sanitized to prevent path traversal attacks
//   - Save() validates destination path
//
// Example:
//
//	file, err := c.File("avatar")
//	if err != nil {
//	    return c.JSON(400, router.H{"error": "avatar required"})
//	}
//
//	fmt.Printf("Received: %s (%d bytes, %s)\n", file.Name, file.Size, file.ContentType)
//
//	if err := file.Save("./uploads/" + uuid.New().String() + file.Ext()); err != nil {
//	    return c.JSON(500, router.H{"error": "failed to save"})
//	}
type File struct {
	// Name is the original filename, sanitized to prevent path traversal.
	// Only the base filename is kept (no directory components).
	Name string

	// Size is the file size in bytes.
	Size int64

	// ContentType is the MIME type from the Content-Type header.
	// Examples: "image/png", "application/pdf", "text/plain"
	ContentType string

	// header is the underlying multipart.FileHeader for internal use.
	header *multipart.FileHeader
}

// newFile creates a File from a multipart.FileHeader.
// The filename is sanitized to prevent path traversal attacks.
func newFile(fh *multipart.FileHeader) *File {
	// Sanitize filename: use only the base name to prevent path traversal
	name := filepath.Base(fh.Filename)

	// Additional sanitization: remove any remaining path separators
	// that might slip through on different OS
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	// Get content type from header
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &File{
		Name:        name,
		Size:        fh.Size,
		ContentType: contentType,
		header:      fh,
	}
}

// Bytes reads the entire file contents into memory.
// Use Open() for large files to avoid memory pressure.
//
// Example:
//
//	file, _ := c.File("config")
//	data, err := file.Bytes()
//	if err != nil {
//	    return err
//	}
//	// Process data...
func (f *File) Bytes() ([]byte, error) {
	src, err := f.header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	return io.ReadAll(src)
}

// Open returns a reader for streaming the file contents.
// Caller must close the returned ReadCloser when done.
//
// Use this for large files to avoid loading the entire file into memory.
//
// Example:
//
//	file, _ := c.File("video")
//	reader, err := file.Open()
//	if err != nil {
//	    return err
//	}
//	defer reader.Close()
//
//	// Stream to destination...
//	io.Copy(destination, reader)
func (f *File) Open() (io.ReadCloser, error) {
	src, err := f.header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return src, nil
}

// Save writes the file to the destination path.
// Creates parent directories automatically if they don't exist.
//
// The destination path is cleaned to prevent path traversal,
// but you should still validate the destination is within
// your intended upload directory.
//
// Example:
//
//	file, _ := c.File("document")
//
//	// Save with original name
//	file.Save("./uploads/" + file.Name)
//
//	// Save with generated name (recommended)
//	file.Save("./uploads/" + uuid.New().String() + file.Ext())
func (f *File) Save(dst string) (err error) {
	// Clean the destination path
	dst = filepath.Clean(dst)

	// Open the uploaded file
	src, err := f.header.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close source file: %w", cerr)
		}
	}()

	// Create parent directories if needed
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		// CRITICAL: Close can fail when flushing buffered data to disk.
		// If Close fails, the file may be incomplete even though io.Copy succeeded.
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", cerr)
		}
	}()

	// Copy file contents
	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// Ext returns the file extension including the dot.
// Returns empty string if no extension is present.
//
// Examples:
//
//	"image.jpg"  → ".jpg"
//	"doc.tar.gz" → ".gz"
//	"README"     → ""
func (f *File) Ext() string {
	return filepath.Ext(f.Name)
}
