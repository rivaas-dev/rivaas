package key

import "time"

//Key object
type Key struct {
	Hash           string
	ActorID        string
	ExpirationDate *time.Time
}

//New key
func New(hash string, actorID string, expirationDate *time.Time) *Key {
	return &Key{Hash: hash, ActorID: actorID, ExpirationDate: expirationDate}
}
