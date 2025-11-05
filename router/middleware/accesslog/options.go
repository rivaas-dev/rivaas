package accesslog

import "time"

// Option defines functional options for access log middleware.
type Option func(*config)

// config holds access log configuration.
type config struct {
	// excludePaths are exact paths to skip
	excludePaths map[string]bool

	// excludePrefixes are path prefixes to skip (e.g., "/metrics")
	excludePrefixes []string

	// sampleRate samples access logs (1.0 = all, 0.1 = 10%)
	sampleRate float64

	// logErrorsOnly only logs requests with status >= 400
	logErrorsOnly bool

	// slowThreshold logs slow requests separately (forced logging)
	slowThreshold time.Duration
}

func defaultConfig() *config {
	return &config{
		excludePaths:  make(map[string]bool),
		sampleRate:    1.0, // Log everything by default
		logErrorsOnly: false,
	}
}

// WithExcludePaths skips logging for exact path matches.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithExcludePaths("/health", "/metrics"),
//	)
func WithExcludePaths(paths ...string) Option {
	return func(c *config) {
		for _, path := range paths {
			c.excludePaths[path] = true
		}
	}
}

// WithExcludePrefixes skips logging for paths with given prefixes.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithExcludePrefixes("/metrics", "/debug"),
//	)
func WithExcludePrefixes(prefixes ...string) Option {
	return func(c *config) {
		c.excludePrefixes = append(c.excludePrefixes, prefixes...)
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0).
// A rate of 1.0 logs all requests, 0.1 logs 10% of requests.
// Sampling is deterministic based on request ID hash.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithSampleRate(0.1), // Log 10% of requests
//	)
func WithSampleRate(rate float64) Option {
	return func(c *config) {
		if rate < 0 {
			rate = 0
		}
		if rate > 1 {
			rate = 1
		}
		c.sampleRate = rate
	}
}

// WithErrorsOnly only logs requests with errors (status >= 400).
// This is useful for reducing log volume in production while still
// capturing all errors for debugging.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithErrorsOnly(),
//	)
func WithErrorsOnly() Option {
	return func(c *config) {
		c.logErrorsOnly = true
	}
}

// WithSlowThreshold logs slow requests separately (forced, ignores sampling).
// Requests that exceed the threshold will always be logged, even if
// sampling would normally skip them.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithSlowThreshold(500 * time.Millisecond),
//	)
func WithSlowThreshold(threshold time.Duration) Option {
	return func(c *config) {
		c.slowThreshold = threshold
	}
}
