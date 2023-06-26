// Package getkey returns a key details.
package getkey

import (
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

type contact struct {
	Emails []string `json:"emails"`
	Users  []uint   `json:"users"`
}

type rateLimit struct {
	Rate uint `json:"rate"`
	Per  uint `json:"per"`
}

// input represents request body.
type input struct {
	Hash string `uri:"id" binding:"required"` // Key ID.
}

// output represents response body.
type output struct {
	ActorID      string    `json:"actor_id"`
	QuotaEndDate date.Date `json:"quota_end_date"`
	Quota        int64     `json:"quota"`
	Description  string    `json:"description"`
	Policies     []string  `json:"policies"`
	CreationDate time.Time `json:"creation_date"`
	Contact      contact   `json:"contacts,omitempty"`
	Active       bool      `json:"active"`
	RateLimit    rateLimit `json:"rate_limit"`
}
