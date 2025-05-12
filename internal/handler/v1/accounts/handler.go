package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient keycloak.Client
	keycloakConfig config.KeyCloakConfig
	solvimonClient *solvimon.Client
}

// New constructs a new Handler.
func New(KeyCloakClient keycloak.Client, KeyCloakConfig config.KeyCloakConfig, SolvimonClient *solvimon.Client) *Handler {
	return &Handler{
		keycloakClient: KeyCloakClient,
		keycloakConfig: KeyCloakConfig,
		solvimonClient: SolvimonClient,
	}
}
