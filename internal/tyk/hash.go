// Package tyk provides utilities for Tyk Gateway integration.
package tyk

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/spaolacci/murmur3"
)

// Hash algorithm constants matching Tyk's implementation.
const (
	HashSha256    = "sha256"
	HashMurmur32  = "murmur32"
	HashMurmur64  = "murmur64"
	HashMurmur128 = "murmur128"

	// B64JSONPrefix is the base64 encoding of "{" - indicates a JSON token.
	B64JSONPrefix = "eyJ"
)

// hashFunction returns the appropriate hash.Hash for the given algorithm.
// Defaults to murmur32 if algorithm is empty or unknown.
func hashFunction(algorithm string) hash.Hash {
	switch algorithm {
	case HashSha256:
		return sha256.New()
	case HashMurmur64:
		return murmur3.New64()
	case HashMurmur128:
		return murmur3.New128()
	case "", HashMurmur32:
		return murmur3.New32()
	default:
		// Unknown algorithm, fall back to murmur32
		return murmur3.New32()
	}
}

// tokenHashAlgo extracts the hash algorithm from a base64-encoded JSON token.
// Returns empty string for legacy tokens (which use murmur32 by default).
func tokenHashAlgo(token string) string {
	if !strings.HasPrefix(token, B64JSONPrefix) {
		// Legacy token format - uses default algorithm
		return ""
	}

	jsonToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		// Failed to decode - treat as legacy token
		return ""
	}

	hashAlgo, err := jsonparser.GetString(jsonToken, "h")
	if err != nil {
		// No hash algorithm field - use default
		return ""
	}

	return hashAlgo
}

// HashKey hashes a raw API key using the same algorithm Tyk uses.
// It automatically detects the token format (legacy vs base64 JSON)
// and uses the appropriate hashing algorithm.
func HashKey(rawKey string) string {
	algo := tokenHashAlgo(rawKey)
	h := hashFunction(algo)
	h.Write([]byte(rawKey))
	return hex.EncodeToString(h.Sum(nil))
}

// HashKeyWithAlgorithm hashes a raw API key with a specific algorithm.
// Use this when you know the algorithm ahead of time.
func HashKeyWithAlgorithm(rawKey, algorithm string) string {
	h := hashFunction(algorithm)
	h.Write([]byte(rawKey))
	return hex.EncodeToString(h.Sum(nil))
}
