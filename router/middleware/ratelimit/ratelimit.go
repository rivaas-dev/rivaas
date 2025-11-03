package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"rivaas.dev/router"
)

// Option defines functional options for ratelimit middleware configuration.
type Option func(*config)

// config holds the configuration for the ratelimit middleware.
type config struct {
	// requestsPerSecond is the number of requests allowed per second
	requestsPerSecond int

	// burst is the maximum burst size (number of requests that can be made instantly)
	burst int

	// keyFunc extracts the rate limit key from the request (e.g., IP address, user ID)
	keyFunc func(*router.Context) string

	// onLimitExceeded is called when rate limit is exceeded
	onLimitExceeded func(*router.Context)

	// cleanupInterval is how often to clean up expired limiters
	cleanupInterval time.Duration

	// limiterTTL is how long to keep a limiter before cleaning it up
	limiterTTL time.Duration
}

// rateLimiter implements a token bucket algorithm for rate limiting.
// This is a per-key limiter that tracks tokens and last refill time.
type rateLimiter struct {
	tokens         float64   // Current number of tokens
	lastRefillTime time.Time // Last time tokens were refilled
	mu             sync.Mutex
}

// rateLimiterStore manages multiple rate limiters keyed by client identifier.
type rateLimiterStore struct {
	limiters map[string]*rateLimiter
	mu       sync.RWMutex
	config   *config
	stopChan chan struct{}
}

// defaultConfig returns the default configuration for ratelimit middleware.
func defaultConfig() *config {
	return &config{
		requestsPerSecond: 100,                 // 100 requests per second
		burst:             20,                  // Allow bursts up to 20 requests
		keyFunc:           defaultKeyFunc,      // Use client IP as key
		onLimitExceeded:   defaultLimitHandler, // Default 429 response
		cleanupInterval:   time.Minute,         // Clean up every minute
		limiterTTL:        5 * time.Minute,     // Remove inactive limiters after 5 minutes
	}
}

// defaultKeyFunc extracts the client IP address as the rate limit key.
func defaultKeyFunc(c *router.Context) string {
	return c.ClientIP()
}

// defaultLimitHandler sends a 429 Too Many Requests response.
func defaultLimitHandler(c *router.Context) {
	c.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "rate limit exceeded",
	})
}

// newRateLimiterStore creates a new rate limiter store and starts cleanup goroutine.
func newRateLimiterStore(cfg *config) *rateLimiterStore {
	store := &rateLimiterStore{
		limiters: make(map[string]*rateLimiter),
		config:   cfg,
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go store.cleanupLoop()

	return store
}

// cleanupLoop periodically removes expired limiters to prevent memory leaks.
func (s *rateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(s.config.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopChan:
			return
		}
	}
}

// cleanup removes limiters that haven't been used recently.
func (s *rateLimiterStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, limiter := range s.limiters {
		limiter.mu.Lock()
		inactive := now.Sub(limiter.lastRefillTime) > s.config.limiterTTL
		limiter.mu.Unlock()

		if inactive {
			delete(s.limiters, key)
		}
	}
}

// stop stops the cleanup goroutine.
func (s *rateLimiterStore) stop() {
	close(s.stopChan)
}

// getLimiter retrieves or creates a limiter for the given key.
func (s *rateLimiterStore) getLimiter(key string) *rateLimiter {
	// Fast path: read lock for existing limiter
	s.mu.RLock()
	limiter, exists := s.limiters[key]
	s.mu.RUnlock()

	if exists {
		return limiter
	}

	// Slow path: create new limiter with write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	limiter, exists = s.limiters[key]
	if exists {
		return limiter
	}

	// Create new limiter with full bucket
	limiter = &rateLimiter{
		tokens:         float64(s.config.burst),
		lastRefillTime: time.Now(),
	}
	s.limiters[key] = limiter

	return limiter
}

// allow checks if a request should be allowed based on the token bucket algorithm.
func (l *rateLimiter) allow(rate int, burst int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefillTime).Seconds()

	// Refill tokens based on time elapsed
	// Formula: tokens = min(burst, tokens + rate * elapsed)
	l.tokens += float64(rate) * elapsed
	if l.tokens > float64(burst) {
		l.tokens = float64(burst)
	}

	l.lastRefillTime = now

	// Check if we have enough tokens
	if l.tokens >= 1.0 {
		l.tokens -= 1.0
		return true
	}

	return false
}

// New returns a middleware that limits request rate using the token bucket algorithm.
//
// Algorithm: Token Bucket
//   - Each client has a bucket that holds tokens (burst capacity)
//   - Tokens are refilled at a constant rate (requestsPerSecond)
//   - Each request consumes 1 token
//   - Requests are rejected when bucket is empty
//
// Features:
//   - Per-client rate limiting (default: by IP address)
//   - Configurable burst support for traffic spikes
//   - Automatic cleanup of inactive limiters
//   - Custom key extraction (IP, user ID, API key)
//   - Custom limit exceeded handler
//
// Basic usage:
//
//	r := router.New()
//	r.Use(ratelimit.New())
//
// Custom configuration:
//
//	r.Use(ratelimit.New(
//	    ratelimit.WithRequestsPerSecond(50),   // 50 req/s per client
//	    ratelimit.WithBurst(10),               // Allow bursts of 10
//	    ratelimit.WithKeyFunc(func(c *router.Context) string {
//	        return c.Request.Header.Get("X-API-Key") // Rate limit by API key
//	    }),
//	))
//
// Per-user rate limiting:
//
//	r.Use(ratelimit.New(
//	    ratelimit.WithKeyFunc(func(c *router.Context) string {
//	        userID := c.Request.Header.Get("X-User-ID")
//	        if userID == "" {
//	            return c.ClientIP() // Fall back to IP
//	        }
//	        return "user:" + userID
//	    }),
//	))
//
// Performance: ~200-500ns overhead per request (negligible)
// Memory: ~200 bytes per unique client (cleaned up after 5 minutes of inactivity)
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Create rate limiter store
	store := newRateLimiterStore(cfg)

	return func(c *router.Context) {
		// Extract rate limit key
		key := cfg.keyFunc(c)
		if key == "" {
			// If key extraction fails, allow the request
			// This prevents blocking all requests due to misconfiguration
			c.Next()
			return
		}

		// Get limiter for this key
		limiter := store.getLimiter(key)

		// Check if request is allowed
		if !limiter.allow(cfg.requestsPerSecond, cfg.burst) {
			// Rate limit exceeded
			cfg.onLimitExceeded(c)
			c.Abort()
			return
		}

		// Request allowed, continue
		c.Next()
	}
}
