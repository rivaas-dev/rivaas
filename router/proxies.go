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
	"fmt"
	"net"
	"strings"
)

// RealIPHeader represents a header name used for real client IP detection.
type RealIPHeader string

const (
	// HeaderXFF is the X-Forwarded-For header.
	HeaderXFF RealIPHeader = "X-Forwarded-For"

	// HeaderXRealIP is the X-Real-IP header.
	HeaderXRealIP RealIPHeader = "X-Real-IP"

	// HeaderCFConnecting is the CF-Connecting-IP header (Cloudflare).
	HeaderCFConnecting RealIPHeader = "CF-Connecting-IP"
)

// TrustedProxyOption configures trusted proxy detection for real client IP.
type TrustedProxyOption func(*trustedProxyConfig)

// trustedProxyConfig holds the configuration for trusted proxies.
type trustedProxyConfig struct {
	proxies []string
	headers []RealIPHeader
	maxHops int
}

// realIPConfig holds the compiled trusted proxy configuration.
type realIPConfig struct {
	cidrs   []*net.IPNet
	headers []RealIPHeader
	maxHops int
}

// WithProxies sets the list of trusted proxy CIDR ranges.
// Only requests from these IPs will have their X-Forwarded-For headers trusted.
//
// Example:
//
//	router.WithProxies("10.0.0.0/8", "192.168.0.0/16", "127.0.0.1/32")
func WithProxies(cidrs ...string) TrustedProxyOption {
	return func(cfg *trustedProxyConfig) {
		cfg.proxies = cidrs
	}
}

// WithProxyHeaders sets which headers to consult, in order of preference.
// Defaults to [HeaderXFF, HeaderXRealIP] if not specified.
//
// You can use predefined headers (HeaderXFF, HeaderXRealIP, HeaderCFConnecting)
// or create custom headers for any CDN/proxy service by casting a string:
//
//	customHeader := router.RealIPHeader("Fastly-Client-IP")
//	router.WithProxyHeaders(router.HeaderXFF, customHeader)
//
// Common CDN headers:
//   - Cloudflare: "CF-Connecting-IP" (or use HeaderCFConnecting)
//   - Fastly: "Fastly-Client-IP"
//   - Akamai: "True-Client-IP"
//   - AWS CloudFront: "CloudFront-Viewer-Address" (contains IP:port, needs parsing)
//
// Example:
//
//	router.WithProxyHeaders(HeaderXFF, HeaderXRealIP, HeaderCFConnecting)
//
//	// With custom header:
//	router.WithProxyHeaders(
//	    router.HeaderXFF,
//	    router.RealIPHeader("Fastly-Client-IP"),
//	)
func WithProxyHeaders(headers ...RealIPHeader) TrustedProxyOption {
	return func(cfg *trustedProxyConfig) {
		cfg.headers = headers
	}
}

// WithProxyMaxHops sets the maximum number of trusted proxies to walk in X-Forwarded-For.
// Defaults to 1 if not specified.
// This prevents walking too far back in the chain and using an attacker's IP.
//
// Example:
//
//	router.WithProxyMaxHops(3)
func WithProxyMaxHops(maxHops int) TrustedProxyOption {
	return func(cfg *trustedProxyConfig) {
		cfg.maxHops = maxHops
	}
}

// compileProxies compiles the trusted proxy configuration, parsing CIDRs and setting defaults.
func compileProxies(opts *trustedProxyConfig) (*realIPConfig, error) {
	cfg := &realIPConfig{
		headers: opts.headers,
		maxHops: opts.maxHops,
	}

	// Set defaults
	if len(cfg.headers) == 0 {
		cfg.headers = []RealIPHeader{HeaderXFF, HeaderXRealIP}
	}
	if cfg.maxHops <= 0 {
		cfg.maxHops = 1
	}

	// Parse CIDRs once at startup
	cfg.cidrs = make([]*net.IPNet, 0, len(opts.proxies))
	for _, cidr := range opts.proxies {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		cfg.cidrs = append(cfg.cidrs, ipnet)
	}

	return cfg, nil
}

// isTrusted checks if an IP address is in the trusted proxy list.
func (cfg *realIPConfig) isTrusted(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	for _, ipnet := range cfg.cidrs {
		if ipnet.Contains(parsed) {
			return true
		}
	}
	return false
}

// WithTrustedProxies returns a RouterOption that configures trusted proxy detection.
//
// Security: Only IPs in the Proxies CIDR list will have their forwarding headers
// trusted. This prevents IP spoofing attacks.
//
// Example:
//
//	r := router.MustNew(
//	    router.WithTrustedProxies(
//	        router.WithProxies("10.0.0.0/8", "192.168.0.0/16"),
//	    ),
//	)
//
//	// With custom headers and max hops:
//	r := router.MustNew(
//	    router.WithTrustedProxies(
//	        router.WithProxies("10.0.0.0/8", "192.168.0.0/16"),
//	        router.WithProxyHeaders(router.HeaderXFF, router.HeaderXRealIP),
//	        router.WithProxyMaxHops(1),
//	    ),
//	)
func WithTrustedProxies(opts ...TrustedProxyOption) Option {
	return func(r *Router) {
		cfg := &trustedProxyConfig{}
		for _, opt := range opts {
			opt(cfg)
		}

		compiled, err := compileProxies(cfg)
		if err != nil {
			// Fail fast on invalid configuration
			panic(fmt.Sprintf("invalid trusted proxy configuration: %v", err))
		}
		r.realip = compiled
	}
}

// ClientIP returns the real client IP address, respecting trusted proxy headers.
//
// Algorithm:
//  1. Extract peer IP from RemoteAddr
//  2. If peer is not trusted → return peer IP (ignore headers)
//  3. If peer is trusted → consult headers in order:
//     - X-Forwarded-For: walk backwards to find last untrusted IP (respecting MaxHops)
//     - X-Real-IP: use if present
//     - CF-Connecting-IP: use if present (Cloudflare)
//  4. Return found IP or fallback to peer IP
//
// Security: Only trusts headers when the immediate peer is in the trusted CIDR list.
// This prevents attackers from spoofing their IP by sending forged headers.
//
// Example:
//
//	func handler(c *router.Ctx) error {
//	    clientIP := c.ClientIP()
//	    // Use clientIP for rate limiting, logging, etc.
//	    return c.JSON(http.StatusOK, map[string]string{"ip": clientIP})
//	}
func (c *Context) ClientIP() string {
	remote := clientIPFromRemoteAddr(c.Request.RemoteAddr)

	// Fast path: no router or no proxy config
	if c.router == nil || c.router.realip == nil {
		return remote
	}

	cfg := c.router.realip

	// Security: peer must be trusted to consult headers
	if !cfg.isTrusted(remote) {
		return remote
	}

	// Peer is trusted → consult headers in order
	for _, h := range cfg.headers {
		switch h {
		case HeaderXFF:
			if ip := lastUntrustedXFF(c.Request.Header.Get("X-Forwarded-For"), cfg); ip != "" {
				// Report suspicious long chains
				xff := c.Request.Header.Get("X-Forwarded-For")
				if strings.Count(xff, ",") > 10 {
					c.router.emit(DiagXFFSuspicious, "suspicious X-Forwarded-For chain detected", map[string]any{
						"remote":    remote,
						"xff_count": strings.Count(xff, ",") + 1,
						"xff":       xff,
					})
				}
				return ip
			}
		case HeaderXRealIP:
			if ip := parseOneIP(c.Request.Header.Get("X-Real-IP")); ip != "" {
				return ip
			}
		case HeaderCFConnecting:
			if ip := parseOneIP(c.Request.Header.Get("Cf-Connecting-Ip")); ip != "" {
				return ip
			}
		default:
			// Support custom header names (e.g., Fastly-Client-IP, True-Client-IP, etc.)
			// RealIPHeader is a string type, so any header name can be used
			if ip := parseOneIP(c.Request.Header.Get(string(h))); ip != "" {
				return ip
			}
		}
	}

	return remote
}

// clientIPFromRemoteAddr extracts the IP from RemoteAddr (format: "ip:port").
func clientIPFromRemoteAddr(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	// Remove port if present
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port, return as-is
		return remoteAddr
	}
	return host
}

// lastUntrustedXFF finds the last untrusted IP in the X-Forwarded-For chain.
// Walks from right to left, stopping when crossing from trusted to untrusted.
func lastUntrustedXFF(xff string, cfg *realIPConfig) string {
	if xff == "" {
		return ""
	}

	// Split XFF header (comma-separated list)
	parts := splitAndTrim(xff, ',')
	if len(parts) == 0 {
		return ""
	}

	// Walk from right to left (most recent proxy first)
	// Track the leftmost untrusted IP we've seen
	hops := 0
	leftmostUntrusted := ""

	for i := len(parts) - 1; i >= 0; i-- {
		ip := parseOneIP(parts[i])
		if ip == "" {
			continue
		}

		if cfg.isTrusted(ip) {
			hops++
			if cfg.maxHops > 0 && hops > cfg.maxHops {
				break // Max hops exceeded
			}
			continue
		}

		// Found an untrusted IP - track it (rightmost wins, but we'll use leftmost if all are untrusted)
		leftmostUntrusted = ip
		// Continue to check if there are more untrusted IPs before this one
	}

	// If we found untrusted IPs, return the leftmost one from the original chain
	if leftmostUntrusted != "" {
		// Find leftmost untrusted IP in original chain
		for i := range parts {
			if ip := parseOneIP(parts[i]); ip != "" && !cfg.isTrusted(ip) {
				return ip
			}
		}
		// Fallback to the one we found
		return leftmostUntrusted
	}

	// All IPs were trusted, return leftmost (original client)
	if len(parts) > 0 {
		if ip := parseOneIP(parts[0]); ip != "" {
			return ip
		}
	}

	return ""
}

// parseOneIP parses a single IP address, trimming whitespace.
func parseOneIP(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	ip := net.ParseIP(s)
	if ip == nil {
		return ""
	}

	return ip.String()
}

// splitAndTrim splits a string by separator and trims each element.
func splitAndTrim(s string, sep byte) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, string(sep))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
