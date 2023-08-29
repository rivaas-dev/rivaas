// Package keys defines all methods of the API key.
package keys

// Contact represents contact points information.
type Contact struct {
	Emails []string `json:"emails"`
	Users  []uint   `json:"users"`
}

// RateLimit represents rate limit information.
type RateLimit struct {
	Rate uint `json:"rate"`
	Per  uint `json:"per"`
}

// KeyID represents the request key hash.
type KeyID struct {
	ID string `uri:"id" binding:"required"` // Key ID.
}

// ApikeyEnvironment represents the environment the key is used
type ApikeyEnvironment = string

const (
	ProdEnv    ApikeyEnvironment = "production"
	SandboxEnv ApikeyEnvironment = "sandbox"
)
