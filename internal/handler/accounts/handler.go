package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient keycloak.Client
	keycloakConfig config.KeyCloakConfig
}

// New constructs a new Handler.
func New(KeyCloakClient keycloak.Client, KeyCloakConfig config.KeyCloakConfig) *Handler {
	return &Handler{
		keycloakClient: KeyCloakClient,
		keycloakConfig: KeyCloakConfig,
	}
}
