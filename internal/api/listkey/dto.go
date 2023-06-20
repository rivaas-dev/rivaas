// Package listkey returns list of keys.
package listkey

import (
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

// input represents the request body.
type input struct {
	ActorID     string `form:"actor_id"`
	Description string `form:"description"`
}

// output represents the response body.
type output struct {
	Hash         string     `json:"hash"`
	ActorID      string     `json:"actor_id"`
	QuotaEndDate *date.Date `json:"quota_end_date"`
	Description  *string    `json:"description"`
	CreationAt   time.Time  `json:"creation_date"`
}
