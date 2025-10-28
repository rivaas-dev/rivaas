# Content Negotiation Example

This example demonstrates HTTP content negotiation using the Rivaas router's `Accepts*()` methods.

## What is Content Negotiation?

Content negotiation is an HTTP mechanism that allows a server to serve different versions of the same resource based on what the client can handle. The client specifies its preferences using HTTP headers:

- **Accept**: Preferred media types (JSON, XML, HTML, etc.)
- **Accept-Language**: Preferred languages (en, fr, de, etc.)
- **Accept-Encoding**: Preferred compression (gzip, br, deflate)
- **Accept-Charset**: Preferred character sets (utf-8, iso-8859-1, etc.)

## Features Demonstrated

### 1. Media Type Negotiation (`Accepts`)

Serve different formats based on the `Accept` header:

```bash
# Get JSON response
curl -H "Accept: application/json" http://localhost:8080/api/user

# Get XML response
curl -H "Accept: application/xml" http://localhost:8080/api/user

# Get HTML response
curl -H "Accept: text/html" http://localhost:8080/api/user
```

### 2. Language Negotiation (`AcceptsLanguages`)

Serve content in the user's preferred language:

```bash
# English greeting
curl -H "Accept-Language: en" http://localhost:8080/api/greeting

# French greeting
curl -H "Accept-Language: fr" http://localhost:8080/api/greeting

# German greeting
curl -H "Accept-Language: de" http://localhost:8080/api/greeting

# Spanish greeting
curl -H "Accept-Language: es" http://localhost:8080/api/greeting
```

### 3. Encoding Negotiation (`AcceptsEncodings`)

Negotiate compression format:

```bash
# Request Brotli compression
curl -H "Accept-Encoding: br" http://localhost:8080/api/data

# Request Gzip compression
curl -H "Accept-Encoding: gzip" http://localhost:8080/api/data

# Request Deflate compression
curl -H "Accept-Encoding: deflate" http://localhost:8080/api/data
```

### 4. Charset Negotiation (`AcceptsCharsets`)

Negotiate character encoding:

```bash
# Request UTF-8
curl -H "Accept-Charset: utf-8" http://localhost:8080/api/text

# Request ISO-8859-1
curl -H "Accept-Charset: iso-8859-1" http://localhost:8080/api/text
```

### 5. Combined Negotiation

Use multiple headers together:

```bash
curl -H "Accept: application/json" \
     -H "Accept-Language: de" \
     -H "Accept-Encoding: gzip" \
     http://localhost:8080/api/flexible
```

## Running the Example

```bash
# Start the server
go run main.go

# The server will display suggested curl commands
```

## Quality Values

HTTP headers support quality values (q-values) to express preference:

```bash
# Prefer HTML, but accept JSON with lower quality
curl -H "Accept: text/html, application/json;q=0.8" http://localhost:8080/api/user

# Prefer Brotli, but accept gzip if br is not available
curl -H "Accept-Encoding: br;q=1.0, gzip;q=0.8" http://localhost:8080/api/data
```

## API Methods

### `Accepts(offers ...string) string`

Returns the best matching content type from the offers based on the `Accept` header.

```go
format := c.Accepts("json", "xml", "html")
switch format {
case "json":
    c.JSON(200, data)
case "xml":
    // send XML
case "html":
    // send HTML
}
```

### `AcceptsLanguages(offers ...string) string`

Returns the best matching language from the offers based on the `Accept-Language` header.

```go
lang := c.AcceptsLanguages("en", "fr", "de")
// Returns: "en", "fr", "de", or ""
```

### `AcceptsEncodings(offers ...string) string`

Returns the best matching encoding from the offers based on the `Accept-Encoding` header.

```go
encoding := c.AcceptsEncodings("gzip", "br", "deflate")
if encoding != "" {
    // Apply compression
}
```

### `AcceptsCharsets(offers ...string) string`

Returns the best matching charset from the offers based on the `Accept-Charset` header.

```go
charset := c.AcceptsCharsets("utf-8", "iso-8859-1")
c.Header("Content-Type", fmt.Sprintf("text/plain; charset=%s", charset))
```

## Best Practices

1. **Always provide a fallback** when negotiation returns empty string
2. **Return 406 Not Acceptable** if you can't satisfy the client's requirements
3. **Support common formats** like JSON for APIs
4. **Use wildcards carefully** - `*/*` means the client accepts anything
5. **Respect quality values** - the framework handles this automatically
6. **Test with real browser headers** - they're often complex

## Real-World Browser Headers

Modern browsers send complex Accept headers:

```http
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7
Accept-Language: en-US,en;q=0.9,fr;q=0.8
Accept-Encoding: gzip, deflate, br
```

The framework handles these automatically using RFC 7231 rules.

## See Also

- [Router README](../../README.md) - Main documentation
- [RFC 7231](https://tools.ietf.org/html/rfc7231#section-5.3) - HTTP content negotiation specification
- [MDN: Content negotiation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Content_negotiation)

