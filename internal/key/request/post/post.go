package post

import "gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"

//Post request
type Post struct {
	Policies     []string      `json:"policies"`
	ActorID      string        `json:"actor_id" binding:"required"`
	QuotaEndDate *date.YmdDate `json:"quota_end_date"`
	Quota        *int64        `json:"quota"`
	Description  *string       `json:"description"`
}
