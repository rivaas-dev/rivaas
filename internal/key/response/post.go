package response

import "time"

//Post response
type Post struct {
	ID             string     `json:"key"`
	Hash           string     `json:"hash"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
}
