package db

import (
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

type contact struct {
	Emails []string
	Users  []uint
}

type Environment = string

const (
	ProdEnv    Environment = "production"
	SandboxEnv Environment = "sandbox"
)

// Key table structure.
type Key struct {
	Hash        string         `gorm:"column:key_hash;primaryKey"`
	ActorID     string         `gorm:"column:actor_id;index"`
	ExpiresAt   *date.Date     `gorm:"column:expires_at"`
	CreatedAt   time.Time      `gorm:"column:creation_date;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:last_modified;autoUpdateTime"`
	Description *string        `gorm:"column:description"`
	DeletedAt   *time.Time     `gorm:"column:deleted_at"`
	Contact     contact        `gorm:"column:contacts;type:json;serializer:json"`
	Active      bool           `gorm:"column:active;default:true"`
	Metadata    map[string]any `gorm:"column:metadata;type:json;serializer:json"`
	Environment Environment    `gorm:"column:environment"`
}

// TableName overrides the table name used by User to `profiles`
func (Key) TableName() string {
	return "api_keys"
}
