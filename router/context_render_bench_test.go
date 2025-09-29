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

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

// Benchmark data for consistent comparison
var benchData = map[string]any{
	"id":      123,
	"name":    "John Doe",
	"email":   "john@example.com",
	"active":  true,
	"score":   95.5,
	"tags":    []string{"admin", "user", "verified"},
	"profile": map[string]string{"bio": "Developer", "location": "NYC"},
}

// BenchmarkJSON_Baseline establishes baseline performance
func BenchmarkJSON_Baseline(b *testing.B) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.JSON(200, benchData)
	}
}

// BenchmarkIndentedJSON tests formatted JSON rendering.
func BenchmarkIndentedJSON(b *testing.B) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.IndentedJSON(200, benchData)
	}
}

// BenchmarkPureJSON tests unescaped HTML JSON rendering.
func BenchmarkPureJSON(b *testing.B) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.PureJSON(200, benchData)
	}
}

// BenchmarkSecureJSON tests prefixed JSON rendering.
func BenchmarkSecureJSON(b *testing.B) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.SecureJSON(200, benchData)
	}
}

// BenchmarkASCIIJSON tests ASCII-escaped JSON rendering.
func BenchmarkASCIIJSON(b *testing.B) {
	// Data with Unicode for meaningful benchmark
	unicodeData := map[string]string{
		"message": "Hello 世界 🌍",
		"name":    "José García",
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.ASCIIJSON(200, unicodeData)
	}
}

// BenchmarkYAML tests YAML rendering.
func BenchmarkYAML(b *testing.B) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.YAML(200, benchData)
	}
}

// BenchmarkDataFromReader tests streaming.
func BenchmarkDataFromReader(b *testing.B) {
	// 10KB data for realistic streaming test
	data := bytes.Repeat([]byte("A"), 10*1024)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		reader := bytes.NewReader(data)
		_ = c.DataFromReader(200, int64(len(data)), "application/octet-stream", reader, nil)
	}
}

// BenchmarkDataFromReader_Large tests large file streaming
func BenchmarkDataFromReader_Large(b *testing.B) {
	// 1MB data for large file test
	data := bytes.Repeat([]byte("B"), 1024*1024)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		reader := bytes.NewReader(data)
		_ = c.DataFromReader(200, int64(len(data)), "application/octet-stream", reader, nil)
	}
}

// BenchmarkData tests raw data sending.
func BenchmarkData(b *testing.B) {
	data := []byte("Hello, World! This is some test data.")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.Data(200, "text/plain", data)
	}
}

// BenchmarkData_Binary tests binary data sending.
func BenchmarkData_Binary(b *testing.B) {
	// 1KB binary data
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		c.Data(200, "application/octet-stream", data)
	}
}

// BenchmarkJSON_vs_IndentedJSON_Comparison compares JSON and IndentedJSON.
func BenchmarkJSON_vs_IndentedJSON_Comparison(b *testing.B) {
	b.Run("JSON", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.JSON(200, benchData)
		}
	})

	b.Run("IndentedJSON", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.IndentedJSON(200, benchData)
		}
	})
}

// BenchmarkJSON_vs_PureJSON_Comparison compares HTML escaping overhead
func BenchmarkJSON_vs_PureJSON_Comparison(b *testing.B) {
	htmlData := map[string]string{
		"html": "<h1>Title</h1><p>Content</p>",
		"url":  "https://example.com?a=1&b=2&c=3",
		"text": "Some <script>alert('test')</script> content",
	}

	b.Run("JSON_with_HTML_escaping", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.JSON(200, htmlData)
		}
	})

	b.Run("PureJSON_no_escaping", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.PureJSON(200, htmlData)
		}
	})
}

// BenchmarkJSON_vs_SecureJSON_Comparison compares prefix overhead
func BenchmarkJSON_vs_SecureJSON_Comparison(b *testing.B) {
	b.Run("JSON", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.JSON(200, benchData)
		}
	})

	b.Run("SecureJSON", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.SecureJSON(200, benchData)
		}
	})
}

// BenchmarkJSON_vs_YAML_Comparison compares JSON and YAML rendering.
func BenchmarkJSON_vs_YAML_Comparison(b *testing.B) {
	b.Run("JSON", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.JSON(200, benchData)
		}
	})

	b.Run("YAML", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.YAML(200, benchData)
		}
	})
}

// BenchmarkDataFromReader_Sizes tests streaming at different sizes
func BenchmarkDataFromReader_Sizes(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			data := bytes.Repeat([]byte("X"), sz.size)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(sz.size))

			for i := 0; i < b.N; i++ {
				w.Body.Reset()
				reader := bytes.NewReader(data)
				_ = c.DataFromReader(200, int64(sz.size), "application/octet-stream", reader, nil)
			}
		})
	}
}

// BenchmarkAllRenderingMethods compares all methods side-by-side
func BenchmarkAllRenderingMethods(b *testing.B) {
	methods := []struct {
		name string
		fn   func(*Context) error
	}{
		{"JSON", func(c *Context) error { return c.WriteJSON(200, benchData) }},
		{"IndentedJSON", func(c *Context) error { return c.WriteIndentedJSON(200, benchData) }},
		{"PureJSON", func(c *Context) error { return c.WritePureJSON(200, benchData) }},
		{"SecureJSON", func(c *Context) error { return c.WriteSecureJSON(200, benchData) }},
		{"ASCIIJSON", func(c *Context) error {
			return c.WriteASCIIJSON(200, map[string]string{"msg": "Hello 世界"})
		}},
		{"YAML", func(c *Context) error { return c.WriteYAML(200, benchData) }},
		{"Data", func(c *Context) error { return c.WriteData(200, "text/plain", []byte("test")) }},
	}

	for _, method := range methods {
		b.Run(method.name, func(b *testing.B) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w.Body.Reset()
				_ = method.fn(c)
			}
		})
	}
}

// BenchmarkDataFromReader_vs_Data compares streaming vs buffered
func BenchmarkDataFromReader_vs_Data(b *testing.B) {
	data := bytes.Repeat([]byte("Test data"), 1000) // ~9KB

	b.Run("DataFromReader_streaming", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := bytes.NewReader(data)
			_ = c.DataFromReader(200, int64(len(data)), "application/octet-stream", reader, nil)
		}
	})

	b.Run("Data_buffered", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			c.Data(200, "application/octet-stream", data)
		}
	})
}

// BenchmarkSecureJSON_PrefixSizes tests prefix length impact
func BenchmarkSecureJSON_PrefixSizes(b *testing.B) {
	prefixes := []struct {
		name   string
		prefix string
	}{
		{"short", "x;"},
		{"default", "while(1);"},
		{"medium", "for(;;);"},
		{"long", strings.Repeat("x", 100) + ";"},
	}

	for _, p := range prefixes {
		b.Run(p.name, func(b *testing.B) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w.Body.Reset()
				c.SecureJSON(200, benchData, p.prefix)
			}
		})
	}
}

// BenchmarkDataFromReader_WithExtraHeaders tests header overhead
func BenchmarkDataFromReader_WithExtraHeaders(b *testing.B) {
	data := []byte("Test data")

	headers := map[string]string{
		"Content-Disposition": `attachment; filename="test.txt"`,
		"Cache-Control":       "no-cache, no-store, must-revalidate",
		"X-Custom-Header":     "value",
	}

	b.Run("no_extra_headers", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := bytes.NewReader(data)
			_ = c.DataFromReader(200, int64(len(data)), "text/plain", reader, nil)
		}
	})

	b.Run("with_extra_headers", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := bytes.NewReader(data)
			_ = c.DataFromReader(200, int64(len(data)), "text/plain", reader, headers)
		}
	})
}

// BenchmarkJSON_Variants_Parallel tests concurrent rendering.
func BenchmarkJSON_Variants_Parallel(b *testing.B) {
	methods := []struct {
		name string
		fn   func(*Context) error
	}{
		{"JSON", func(c *Context) error { return c.WriteJSON(200, benchData) }},
		{"PureJSON", func(c *Context) error { return c.WritePureJSON(200, benchData) }},
		{"SecureJSON", func(c *Context) error { return c.WriteSecureJSON(200, benchData) }},
	}

	for _, method := range methods {
		b.Run(method.name, func(b *testing.B) {
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/", nil)
				c := NewContext(w, req)

				for pb.Next() {
					w.Body.Reset()
					_ = method.fn(c)
				}
			})
		})
	}
}

// BenchmarkASCIIJSON_UnicodeComplexity tests Unicode escaping at different complexities
func BenchmarkASCIIJSON_UnicodeComplexity(b *testing.B) {
	tests := []struct {
		name string
		data map[string]string
	}{
		{
			"pure_ASCII",
			map[string]string{"text": "Hello World 123"},
		},
		{
			"mixed_ASCII_Latin",
			map[string]string{"name": "José García", "city": "São Paulo"},
		},
		{
			"CJK_characters",
			map[string]string{"message": "你好世界", "greeting": "こんにちは"},
		},
		{
			"emoji_heavy",
			map[string]string{"text": "🌍🎉🚀⭐🔥💯"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w.Body.Reset()
				c.ASCIIJSON(200, tt.data)
			}
		})
	}
}

// BenchmarkDataFromReader_ReaderTypes tests different io.Reader implementations
func BenchmarkDataFromReader_ReaderTypes(b *testing.B) {
	data := bytes.Repeat([]byte("X"), 10*1024) // 10KB

	b.Run("bytes.Reader", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := bytes.NewReader(data)
			_ = c.DataFromReader(200, int64(len(data)), "application/octet-stream", reader, nil)
		}
	})

	b.Run("bytes.Buffer", func(b *testing.B) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := bytes.NewBuffer(data)
			_ = c.DataFromReader(200, int64(len(data)), "application/octet-stream", reader, nil)
		}
	})

	b.Run("strings.Reader", func(b *testing.B) {
		strData := string(data)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))

		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			reader := strings.NewReader(strData)
			_ = c.DataFromReader(200, int64(len(strData)), "text/plain", reader, nil)
		}
	})
}

// BenchmarkDataFromReader_ChunkedTransfer tests streaming without Content-Length
func BenchmarkDataFromReader_ChunkedTransfer(b *testing.B) {
	data := bytes.Repeat([]byte("Chunk "), 1000) // ~6KB

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		w.Body.Reset()
		reader := bytes.NewReader(data)
		// contentLength = -1 triggers chunked transfer
		_ = c.DataFromReader(200, -1, "application/octet-stream", reader, nil)
	}
}
