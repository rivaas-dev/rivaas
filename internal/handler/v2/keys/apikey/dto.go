package apikey

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

// Contact represents contact points information.
type Contact struct {
	Emails []string `jsonapi:"attr,emails"`
	Users  []uint   `jsonapi:"attr,users"`
}

// RateLimit represents rate limit information.
type RateLimit struct {
	Rate float64 `jsonapi:"attr,rate"`
	Per  float64 `jsonapi:"attr,per"`
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

type APIKey struct {
	ID             string            `jsonapi:"primary,keys"` // The key identifier. This is a unique identifier to each API key.
	Name           string            `jsonapi:"attr,name"`    // name of the API Key
	CustomerName   string            `jsonapi:"attr,customerName"`
	Hash           string            `jsonapi:"attr,hash,omitempty"`
	CreationDate   string            `jsonapi:"attr,creationDate"`
	Environment    ApikeyEnvironment `jsonapi:"attr,environment"`
	ActorID        string            `jsonapi:"attr,actorID"`
	CreatorID      string            `jsonapi:"attr,creatorID"`
	Policies       []string          `jsonapi:"attr,policies"`
	ExpiresAt      *date.Date        `jsonapi:"attr,expiresAt"`
	Quota          int64             `jsonapi:"attr,quota"`
	QuotaRemaining int64             `jsonapi:"attr,quotaRemaining"`
	Description    string            `jsonapi:"attr,description"`
	CreatedDate    string            `jsonapi:"attr,createdDate"`
	Contact        Contact           `jsonapi:"attr,contacts"`
	Active         bool              `jsonapi:"attr,active"`
	RateLimit      RateLimit         `jsonapi:"attr,rateLimit"`
	Labels         map[string]string `jsonapi:"attr,labels"`
}

func String(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}
