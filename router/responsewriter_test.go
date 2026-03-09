// Copyright 2026 The Rivaas Authors
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

package router

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponseWriterWrapper(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)
	require.NotNil(t, rw)
	assert.Same(t, w, rw.ResponseWriter)
	assert.False(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode()) // getter returns 200 when not yet set
	assert.Equal(t, int64(0), rw.Size())
}

func TestResponseWriterWrapper_WriteHeader_SetsStatusAndWritten(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	rw.WriteHeader(http.StatusCreated)

	assert.True(t, rw.Written())
	assert.Equal(t, http.StatusCreated, rw.StatusCode())
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestResponseWriterWrapper_WriteHeader_NoDuplicateCall(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	rw.WriteHeader(http.StatusOK)
	rw.WriteHeader(http.StatusCreated) // second call should be ignored

	assert.Equal(t, http.StatusOK, rw.StatusCode())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponseWriterWrapper_Write_WithoutWriteHeader_Implicit200(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	n, err := rw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	assert.True(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode())
	assert.Equal(t, int64(5), rw.Size())
	assert.Equal(t, "hello", w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponseWriterWrapper_Write_TracksSize(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	rw.WriteHeader(http.StatusOK)
	n1, err := rw.Write([]byte("ab"))
	require.NoError(t, err)
	n2, err := rw.Write([]byte("c"))
	require.NoError(t, err)

	assert.Equal(t, 2, n1)
	assert.Equal(t, 1, n2)
	assert.Equal(t, int64(3), rw.Size())
}

func TestResponseWriterWrapper_StatusCode_Size_Written_Getters(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	assert.False(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode()) // zero value -> 200
	assert.Equal(t, int64(0), rw.Size())

	_, err := rw.Write([]byte("x"))
	require.NoError(t, err)
	assert.True(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode())
	assert.Equal(t, int64(1), rw.Size())

	rw.WriteHeader(http.StatusNotFound) // too late, already written
	assert.Equal(t, http.StatusOK, rw.StatusCode())
}

func TestResponseWriterWrapper_ImplementsResponseInfoAndWrittenChecker(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	var _ ResponseInfo = rw
	var _ WrittenChecker = rw

	// Use via interfaces
	var ri ResponseInfo = rw
	var wc WrittenChecker = rw
	assert.Equal(t, http.StatusOK, ri.StatusCode())
	assert.Equal(t, int64(0), ri.Size())
	assert.False(t, wc.Written())
}

func TestResponseWriterWrapper_Hijack_WhenUnderlyingNotHijacker(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	conn, rwBuf, err := rw.Hijack()

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, rwBuf)
	assert.ErrorIs(t, err, ErrResponseWriterNotHijacker)
}

// mockHijackerResponseWriter implements http.ResponseWriter and http.Hijacker for testing.
type mockHijackerResponseWriter struct {
	http.ResponseWriter
	conn      net.Conn
	rw        *bufio.ReadWriter
	hijackErr error
}

func (m *mockHijackerResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if m.hijackErr != nil {
		return nil, nil, m.hijackErr
	}
	return m.conn, m.rw, nil
}

func TestResponseWriterWrapper_Hijack_WhenUnderlyingIsHijacker(t *testing.T) {
	t.Parallel()
	server, client := net.Pipe()
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		server.Close()
		//nolint:errcheck // Test cleanup
		_ = client.Close()
	})

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	underlying := &mockHijackerResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		conn:           server,
		rw:             mockRW,
	}

	rw := NewResponseWriterWrapper(underlying)

	conn, rwBuf, err := rw.Hijack()

	require.NoError(t, err)
	assert.Same(t, server, conn)
	assert.Same(t, mockRW, rwBuf)
}

func TestResponseWriterWrapper_Flush_WhenUnderlyingNotFlusher(t *testing.T) {
	t.Parallel()
	type noFlusher struct{ http.ResponseWriter }
	recorder := httptest.NewRecorder()
	w := &noFlusher{ResponseWriter: recorder}
	rw := NewResponseWriterWrapper(w)

	rw.Flush() // No-op when underlying is not http.Flusher; must not panic

	// Writer remains usable after Flush
	_, err := rw.Write([]byte("ok"))
	require.NoError(t, err)
	assert.Equal(t, "ok", recorder.Body.String())
}

func TestResponseWriterWrapper_Flush_WhenUnderlyingIsFlusher(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	rw.Flush() // Delegates to underlying; must not panic

	// Writer remains usable after Flush
	_, err := rw.Write([]byte("ok"))
	require.NoError(t, err)
	assert.Equal(t, "ok", w.Body.String())
}

func TestResponseWriterWrapper_AddSize(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	assert.Equal(t, int64(0), rw.Size())
	rw.AddSize(10)
	assert.Equal(t, int64(10), rw.Size())
	rw.AddSize(5)
	assert.Equal(t, int64(15), rw.Size())
}

func TestResponseWriterWrapper_MarkWritten(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	assert.False(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode()) // getter returns 200 when unset

	rw.MarkWritten()

	assert.True(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode())
}

func TestResponseWriterWrapper_MarkWritten_Idempotent(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := NewResponseWriterWrapper(w)

	rw.WriteHeader(http.StatusCreated)
	rw.MarkWritten() // no-op when already written

	assert.Equal(t, http.StatusCreated, rw.StatusCode())
}
