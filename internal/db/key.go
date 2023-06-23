package db

import (
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

type contact struct {
	Emails []string
	Users  []uint
}

// Key table structure.
type Key struct {
	Hash         string     `gorm:"column:key_hash;primaryKey"`
	ActorID      string     `gorm:"column:actor_id;index"`
	QuotaEndDate *date.Date `gorm:"column:quota_end_date"`
	CreatedAt    time.Time  `gorm:"column:creation_date;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:last_modified;autoUpdateTime"`
	Description  *string    `gorm:"column:description"`
	DeletedAt    *time.Time `gorm:"column:deleted_at"`
	Contact      contact    `gorm:"column:contacts;type:json;serializer:json"`
	Active       bool       `gorm:"column:active;default:true"`
}

// TableName overrides the table name used by User to `profiles`
func (Key) TableName() string {
	return "api_keys"
}
