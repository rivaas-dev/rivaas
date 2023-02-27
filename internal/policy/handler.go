package policy

import (
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/policy/api"
)

// Handler handles policy requests
type Handler struct {
	policiesClient api.ClientInterface
}

// NewHandler constructor
func NewHandler(client api.ClientInterface) *Handler {
	return &Handler{policiesClient: client}
}
