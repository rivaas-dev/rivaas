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

// Package methodoverride provides middleware for HTTP method override,
// allowing clients to use POST requests with a header or form field to
// specify the actual HTTP method (PUT, DELETE, etc.).
//
// This middleware enables RESTful APIs to work with clients that don't
// support all HTTP methods (e.g., HTML forms only support GET and POST).
// It's commonly used for PUT and DELETE operations from web forms.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/methodoverride"
//
//	r := router.MustNew()
//	r.Use(methodoverride.New())
//
// # Method Override Sources
//
// The middleware checks for method override in the following order:
//
//   - X-HTTP-Method-Override header (default)
//   - _method form field (for POST requests with form data)
//   - X-HTTP-Method header (alternative header name)
//
// # Configuration Options
//
//   - HeaderName: Custom header name for method override (default: X-HTTP-Method-Override)
//   - FormFieldName: Custom form field name (default: _method)
//   - AllowedMethods: Methods allowed to be overridden (default: PUT, PATCH, DELETE)
//
// # Example Usage
//
// Clients can override methods using headers:
//
//	POST /users/123 HTTP/1.1
//	X-HTTP-Method-Override: DELETE
//
// Or using form fields:
//
//	<form method="POST" action="/users/123">
//	    <input type="hidden" name="_method" value="DELETE">
//	    <button type="submit">Delete</button>
//	</form>
//
// # Security Considerations
//
// Method override should only be used when necessary (e.g., HTML form limitations).
// Consider CSRF protection when using form-based method override. The middleware
// can be checked via CSRFVerified(c) when using WithRequireCSRFToken.
package methodoverride
