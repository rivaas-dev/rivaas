package customers

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient  keycloak.Client
	keycloakConfig  config.KeyCloakConfig
	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(KeyCloakClient keycloak.Client, KeyCloakConfig config.KeyCloakConfig, pagination config.Pagination) *Handler {
	return &Handler{
		keycloakClient:  KeyCloakClient,
		keycloakConfig:  KeyCloakConfig,
		defaultPageSize: pagination.DefaultPageSize,
		maxPageSize:     pagination.MaxPageSize,
	}
}
