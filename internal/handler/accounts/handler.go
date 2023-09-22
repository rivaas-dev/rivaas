package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/keycloak"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient *keycloak.Client
}

// New constructs a new Handler.
func New(KeyCloakClient *keycloak.Client) *Handler {
	return &Handler{
		keycloakClient: KeyCloakClient,
	}
}
