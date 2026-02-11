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

//go:build !integration

package dumper

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FileDumperTestSuite struct {
	suite.Suite
	tmpFile string
}

func (s *FileDumperTestSuite) SetupTest() {
	f, err := os.CreateTemp(s.T().TempDir(), "filedumper_test_*.json")
	s.Require().NoError(err)
	s.tmpFile = f.Name()
	s.Require().NoError(f.Close())
}

func (s *FileDumperTestSuite) TestDump_Success() {
	encoder := &mockEncoder{}
	fileDumper := NewFile(s.tmpFile, encoder)
	values := &map[string]any{"foo": "bar"}

	err := fileDumper.Dump(context.Background(), values)
	s.NoError(err)

	// Check file contents
	data, err := os.ReadFile(s.tmpFile)
	s.NoError(err)
	s.Equal("encoded", string(data))
}

func (s *FileDumperTestSuite) TestDump_EncodeError() {
	encoder := &mockEncoder{err: errors.New("encode error")}
	fileDumper := NewFile(s.tmpFile, encoder)
	values := &map[string]any{"foo": "bar"}

	err := fileDumper.Dump(context.Background(), values)
	s.Error(err)
	s.Contains(err.Error(), "encode error")
}

func (s *FileDumperTestSuite) TestDump_FileWriteError() {
	encoder := &mockEncoder{}
	// Use an invalid path to force a write error
	fileDumper := NewFile("/invalid/path/shouldfail.json", encoder)
	values := &map[string]any{"foo": "bar"}

	err := fileDumper.Dump(context.Background(), values)
	s.Error(err)
	s.Contains(err.Error(), "failed to write file")
}

func (s *FileDumperTestSuite) TestNewFileWithPermissions_DumpWritesWithCustomPermissions() {
	encoder := &mockEncoder{}
	fileDumper := NewFileWithPermissions(s.tmpFile, encoder, 0o600)
	values := &map[string]any{"foo": "bar"}

	err := fileDumper.Dump(context.Background(), values)
	s.Require().NoError(err)

	info, err := os.Stat(s.tmpFile)
	s.Require().NoError(err)
	s.Equal(os.FileMode(0o600), info.Mode().Perm(), "file should have custom permissions 0600")
}

// mockEncoder implements codec.Encoder for testing
// Always returns "encoded" as bytes unless err is set

type mockEncoder struct {
	err error
}

func (m *mockEncoder) Encode(_ any) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte("encoded"), nil
}

func TestFileDumperTestSuite(t *testing.T) {
	suite.Run(t, new(FileDumperTestSuite))
}
