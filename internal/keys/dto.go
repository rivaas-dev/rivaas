package keys

import "time"

//PostKeyRequest request
type PostKeyRequest struct {
	Policies []string `json:"policies"`
	ActorID  string   `json:"actor_id" binding:"required"`
}

//PostKeyResponse response
type PostKeyResponse struct {
	ID   string `json:"key"`
	Hash string `json:"hash"`
}

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
