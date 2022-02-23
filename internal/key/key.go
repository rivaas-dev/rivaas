package key

import "time"

//Key object
type Key struct {
	Hash           string
	ActorID        string
	ExpirationDate *time.Time
	CreationDate   *time.Time
	Description    *string
}

//New key
func New(hash string, actorID string, expirationDate *time.Time, description *string) *Key {
	return &Key{Hash: hash, ActorID: actorID, ExpirationDate: expirationDate, Description: description}
}
