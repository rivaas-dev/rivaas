package response

import (
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/date"
	"time"
)

//KeyDetailed object
type KeyDetailed struct {
	ActorID        string        `json:"actor_id"`
	ExpirationDate *date.YmdDate `json:"expiration_date,omitempty"`
	Quota          *int64        `json:"quota,omitempty"`
	Description    *string       `json:"description,omitempty"`
	Policies       []string      `json:"policies,omitempty"`
	CreationDate   time.Time     `json:"creation_date"`
}

//NewKeyDetailed constructor
func NewKeyDetailed(actorID string, expirationDate *time.Time, quota *int64, description *string, policies []string, creationDate time.Time) *KeyDetailed {
	return &KeyDetailed{ActorID: actorID, ExpirationDate: date.CreateYmdFromTimePtr(expirationDate), Quota: quota,
		Description: description,
		Policies:    policies, CreationDate: creationDate}
}
