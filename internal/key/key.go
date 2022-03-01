package key

import "time"

const (
	apiKeyTable = "api_keys"
)

//Key object
type Key struct {
	Hash           string     `gorm:"column:key_hash;primaryKey"`
	ActorID        string     `gorm:"column:actor_id;index"`
	ExpirationDate *time.Time `gorm:"column:expiration_date"`
	CreatedAt      time.Time  `gorm:"column:creation_date"`
	UpdatedAt      time.Time  `gorm:"column:last_modified"`
	Description    *string    `gorm:"column:description"`
}

// TableName overrides the table name used by User to `profiles`
func (Key) TableName() string {
	return apiKeyTable
}

//New key
func New(hash string, actorID string, expirationDate *time.Time, description *string) *Key {
	return &Key{Hash: hash, ActorID: actorID, ExpirationDate: expirationDate, Description: description}
}
