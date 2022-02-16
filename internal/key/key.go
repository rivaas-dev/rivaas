package key

import "time"

//Key object
type Key struct {
	Hash           string
	ActorID        string
	ExpirationDate *time.Time
	Quota          *int64
	Description    *string
}

//New key
func New(hash string, actorID string, expirationDate *time.Time, quota *int64, description *string) *Key {
	return &Key{Hash: hash, ActorID: actorID, ExpirationDate: expirationDate, Quota: quota, Description: description}
}
