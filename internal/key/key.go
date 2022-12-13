package key

import "time"

const (
	apiKeyTable = "api_keys"
)

// Key object
type Key struct {
	Hash         string     `gorm:"column:key_hash;primaryKey"`
	ActorID      string     `gorm:"column:actor_id;index"`
	QuotaEndDate *time.Time `gorm:"column:quota_end_date"`
	CreatedAt    time.Time  `gorm:"column:creation_date;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:last_modified;autoUpdateTime"`
	Description  *string    `gorm:"column:description"`
}

// TableName overrides the table name used by User to `profiles`
func (Key) TableName() string {
	return apiKeyTable
}

// New key
func New(hash string, actorID string, quotaEndDate *time.Time, description *string) *Key {
	return &Key{Hash: hash, ActorID: actorID, QuotaEndDate: quotaEndDate, Description: description}
}
