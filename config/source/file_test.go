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

package source

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FileSourceTestSuite struct {
	suite.Suite
	tmpFile string
}

func (s *FileSourceTestSuite) SetupTest() {
	f, err := os.CreateTemp("", "filesource_test_*.json")
	s.Require().NoError(err)

	s.tmpFile = f.Name()
	_, err = f.WriteString(`{"foo": "bar"}`)
	s.Require().NoError(err)
	s.Require().NoError(f.Close())
}

func (s *FileSourceTestSuite) TearDownTest() {
	if s.tmpFile != "" {
		s.Require().NoError(os.Remove(s.tmpFile))
	}
}

func TestFileSourceTestSuite(t *testing.T) {
	suite.Run(t, new(FileSourceTestSuite))
}

func (s *FileSourceTestSuite) TestLoad_ValidFile() {
	decoder := &mockDecoderFile{decodeMap: map[string]any{"foo": "bar"}}
	file := NewFile(s.tmpFile, decoder)
	conf, err := file.Load(context.TODO())
	s.NoError(err)
	s.Equal(map[string]any{"foo": "bar"}, conf)
}

func (s *FileSourceTestSuite) TestLoad_EmptyFile() {
	f, err := os.CreateTemp("", "filesource_empty_*.json")
	s.Require().NoError(err)
	tmp := f.Name()
	s.Require().NoError(f.Close())
	defer func() {
		s.Require().NoError(os.Remove(tmp))
	}()
	decoder := &mockDecoderFile{decodeMap: map[string]any{}}
	file := NewFile(tmp, decoder)
	conf, err := file.Load(context.TODO())
	s.NoError(err)
	s.Empty(conf)
}

func (s *FileSourceTestSuite) TestLoad_InvalidFile() {
	file := NewFile("/invalid/path/shouldfail.json", &mockDecoderFile{})
	_, err := file.Load(context.TODO())
	s.Error(err)
}

func (s *FileSourceTestSuite) TestLoad_Content() {
	decoder := &mockDecoderFile{decodeMap: map[string]any{"foo": "bar"}}
	file := NewFileContent([]byte(`{"foo": "bar"}`), decoder)
	conf, err := file.Load(context.TODO())
	s.NoError(err)
	s.Equal(map[string]any{"foo": "bar"}, conf)
}

func (s *FileSourceTestSuite) TestLoad_DecodeError() {
	decoder := &mockDecoderFile{err: true}
	file := NewFile(s.tmpFile, decoder)
	_, err := file.Load(context.TODO())
	s.Error(err)
}

// mockDecoderFile implements codec.Decoder for testing

type mockDecoderFile struct {
	decodeMap map[string]any
	err       bool
}

func (m *mockDecoderFile) Decode(_ []byte, v any) error {
	if m.err {
		return os.ErrInvalid
	}
	if ptr, ok := v.(*map[string]any); ok {
		*ptr = m.decodeMap
		return nil
	}
	return os.ErrInvalid
}
