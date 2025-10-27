package router

import (
	"net/http"
	"strings"
)

// Static serves static files from the filesystem under the given URL prefix.
// The relativePath is the URL prefix, and root is the filesystem directory.
// This creates efficient file serving routes with proper caching headers.
//
// SECURITY: This method uses http.FileServer which automatically prevents
// path traversal attacks (e.g., "../../../etc/passwd"). The http.Dir implementation
// cleans paths and prevents access to parent directories. However, ensure that:
//   - The root directory only contains files intended to be publicly accessible
//   - Sensitive files are not stored in the served directory
//   - File permissions are properly configured at the OS level
//
// Example:
//
//	r.Static("/assets", "./public")      // Serve ./public/* at /assets/*
//	r.Static("/uploads", "/var/uploads") // Serve /var/uploads/* at /uploads/*
func (r *Router) Static(relativePath, root string) {
	r.StaticFS(relativePath, http.Dir(root))
}

// StaticFS serves static files from the given http.FileSystem under the URL prefix.
// This provides more control over the file system implementation.
//
// Example:
//
//	r.StaticFS("/assets", http.Dir("./public"))
//	r.StaticFS("/files", customFileSystem)
func (r *Router) StaticFS(relativePath string, fs http.FileSystem) {
	if len(relativePath) == 0 {
		panic("relativePath cannot be empty")
	}

	// Ensure relativePath starts with / and ends with /*
	if relativePath[0] != '/' {
		relativePath = "/" + relativePath
	}
	if !strings.HasSuffix(relativePath, "/*") {
		if strings.HasSuffix(relativePath, "/") {
			relativePath += "*"
		} else {
			relativePath += "/*"
		}
	}

	// Create a file server handler
	fileServer := http.StripPrefix(strings.TrimSuffix(relativePath, "/*"), http.FileServer(fs))

	// Add the route for static files
	r.GET(relativePath, func(c *Context) {
		fileServer.ServeHTTP(c.Response, c.Request)
	})
}

// StaticFile serves a single file at the given URL path.
// This is useful for serving specific files like favicon.ico or robots.txt.
//
// Example:
//
//	r.StaticFile("/favicon.ico", "./assets/favicon.ico")
//	r.StaticFile("/robots.txt", "./static/robots.txt")
func (r *Router) StaticFile(relativePath, filepath string) {
	if len(relativePath) == 0 {
		panic("relativePath cannot be empty")
	}
	if len(filepath) == 0 {
		panic("filepath cannot be empty")
	}

	// Ensure relativePath starts with /
	if relativePath[0] != '/' {
		relativePath = "/" + relativePath
	}

	r.GET(relativePath, func(c *Context) {
		c.File(filepath)
	})
}
