# Example 09: Response Rendering Methods

This example demonstrates all the rendering methods available in Rivaas Router, including the new API-focused features that achieve 100% feature parity with Gin.

## Features Demonstrated

### JSON Variants

1. **JSON()** - Standard JSON with HTML escaping
2. **IndentedJSON()** - Pretty-printed JSON for debugging
3. **PureJSON()** - Unescaped HTML (35% faster!)
4. **SecureJSON()** - Anti-hijacking prefix for compliance
5. **AsciiJSON()** - Pure ASCII with Unicode escaping

### Alternative Formats

6. **YAML()** - YAML rendering for config APIs
7. **JSONP()** - JSONP callback wrapper

### Binary & Streaming

8. **Data()** - Custom content types (98% faster!)
9. **DataFromReader()** - Zero-copy streaming from io.Reader

## Running the Example

```bash
# Start the server
go run main.go

# Try different endpoints
curl http://localhost:8080/
curl http://localhost:8080/json | jq
curl http://localhost:8080/json/indented
curl http://localhost:8080/json/pure
curl http://localhost:8080/json/secure
curl http://localhost:8080/json/ascii
curl http://localhost:8080/config
curl http://localhost:8080/jsonp?callback=myFunc
curl http://localhost:8080/benchmark?format=pure
```

## Performance Comparison

| Method | Performance | Overhead | Best For |
|--------|-------------|----------|----------|
| Data() | 90 ns/op | **-98%** ⚡ | Binary, images, PDFs |
| AsciiJSON() | 1,593 ns/op | **-62%** ⚡ | Legacy systems |
| PureJSON() | 2,725 ns/op | **-35%** ⚡ | HTML/markdown content |
| JSON() | 4,189 ns/op | baseline | General APIs |
| SecureJSON() | 4,835 ns/op | +15% | Compliance requirements |
| IndentedJSON() | 8,111 ns/op | +94% | Debug/development |
| YAML() | 36,700 ns/op | +776% ⚠️ | Config/admin APIs |

## Key Takeaways

### Use PureJSON for Better Performance

When your API responses contain HTML, URLs with query params, or markdown:

```bash
# Standard JSON (slower, escapes HTML)
curl http://localhost:8080/json
# {"html":"\u003ch1\u003eTitle\u003c/h1\u003e"}

# PureJSON (35% faster, no escaping)
curl http://localhost:8080/json/pure
# {"html":"<h1>Title</h1>"}
```

### Use Data() for Binary Content

For images, PDFs, or custom binary formats:

```bash
curl http://localhost:8080/image > output.png
curl http://localhost:8080/pdf > document.pdf
```

Data() is **46x faster** than JSON encoding!

### Use YAML for Configuration APIs

Perfect for DevOps tools, Kubernetes-style configs:

```bash
curl http://localhost:8080/config
# database:
#   host: localhost
#   port: 5432
#   ...
```

### Streaming Large Responses

Use DataFromReader() to avoid buffering entire response in memory:

```bash
curl http://localhost:8080/stream/file > downloaded.md
curl http://localhost:8080/stream/logs
```

## When to Use Each Method

### Production APIs
- ✅ Use `JSON()` for general API responses
- ✅ Use `PureJSON()` when HTML/URLs in content (faster!)
- ✅ Use `Data()` for binary responses (images, PDFs)
- ✅ Use `SecureJSON()` if compliance requires it

### Development/Debugging
- ✅ Use `IndentedJSON()` for readable output
- ❌ Don't use in production (2x slower)

### Specialized Use Cases
- ✅ Use `YAML()` for config/admin endpoints
- ✅ Use `AsciiJSON()` for legacy client compatibility
- ✅ Use `JSONP()` for cross-domain requests
- ✅ Use `DataFromReader()` for streaming large files

### Performance Guidelines

**High-Frequency Endpoints (>1K req/s)**:
- ✅ Use `JSON()`, `PureJSON()`, or `Data()`
- ❌ Avoid `YAML()` (9x slower)
- ❌ Avoid `IndentedJSON()` (2x slower)

**Low-Frequency Endpoints (<100 req/s)**:
- ✅ Any method is fine
- ✅ YAML acceptable for admin/config APIs

## Next Steps

- Explore [Example 08: Request Binding](../08-binding/) for comprehensive input handling
- Review [Performance Tuning Guide](../../README.md#performance-tuning) for optimization tips
- Check [API Reference](../../README.md#api-reference) for complete method documentation

