package response

import "time"

//Post response
type Post struct {
	ID           string     `json:"key"`
	Hash         string     `json:"hash"`
	QuotaEndDate *time.Time `json:"quota_end_date,omitempty"`
}
