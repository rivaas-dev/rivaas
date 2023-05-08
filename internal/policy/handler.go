package policy

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy/api"
)

// Handler handles policy requests
type Handler struct {
	policiesClient api.ClientInterface
}

// NewHandler constructor
func NewHandler(client api.ClientInterface) *Handler {
	return &Handler{policiesClient: client}
}
