// Package listkey returns list of keys.
package listkey

import (
	"time"
)

// input represents the request body.
type input struct {
	ActorID     string `form:"actor_id"`
	Description string `form:"description"`
}

type contact struct {
	Emails []string `json:"emails"`
	Users  []uint   `json:"users"`
}

type rateLimit struct {
	Rate uint `json:"rate"`
	Per  uint `json:"per"`
}

// output represents the response body.
type output struct {
	Hash        string    `json:"hash"`
	ActorID     string    `json:"actor_id"`
	ExpiresAt   string    `json:"expires_at"`
	Description *string   `json:"description"`
	CreationAt  time.Time `json:"creation_date"`
	Contact     contact   `json:"contacts"`
	Active      bool      `json:"active"`
	RateLimit   rateLimit `json:"rate_limit"`
}
