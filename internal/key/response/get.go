package response

import (
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/date"
	"time"
)

//KeyDetails object used for the list operation (strictly served from database)
type KeyDetails struct {
	KeyHash        string        `json:"hash"`
	ActorID        string        `json:"actor_id"`
	ExpirationDate *date.YmdDate `json:"expiration_date,omitempty"`
	Description    *string       `json:"description,omitempty"`
	CreationDate   time.Time     `json:"creation_date"`
}

//NewKeyDetails constructor
func NewKeyDetails(keyHash string, actorID string, expirationDate *time.Time, description *string,
	creationDate time.Time) *KeyDetails {
	return &KeyDetails{KeyHash: keyHash, ActorID: actorID, ExpirationDate: date.CreateYmdFromTimePtr(expirationDate), Description: description, CreationDate: creationDate}
}

//KeyDetailsWithGatewayContext object used for the /key/{key_hash} operation, is served from both tyk and DB
type KeyDetailsWithGatewayContext struct {
	ActorID        string        `json:"actor_id"`
	ExpirationDate *date.YmdDate `json:"expiration_date,omitempty"`
	Quota          *int64        `json:"quota,omitempty"`
	Description    *string       `json:"description,omitempty"`
	Policies       []string      `json:"policies,omitempty"`
	CreationDate   time.Time     `json:"creation_date"`
}

//NewKeyDetailsWithGatewayContext constructor
func NewKeyDetailsWithGatewayContext(actorID string, expirationDate *time.Time, quota *int64, description *string, policies []string, creationDate time.Time) *KeyDetailsWithGatewayContext {
	return &KeyDetailsWithGatewayContext{ActorID: actorID, ExpirationDate: date.CreateYmdFromTimePtr(expirationDate), Quota: quota,
		Description: description,
		Policies:    policies, CreationDate: creationDate}
}
