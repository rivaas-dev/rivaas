package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient keycloak.Client
	keycloakConfig config.KeyCloakConfig
	solvimonClient *solvimon.Client
	omaClient      *oma.Client

	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(KeyCloakClient keycloak.Client, KeyCloakConfig config.KeyCloakConfig, SolvimonClient *solvimon.Client, omaClient *oma.Client, pagination config.Pagination) *Handler {
	return &Handler{
		keycloakClient:  KeyCloakClient,
		keycloakConfig:  KeyCloakConfig,
		solvimonClient:  SolvimonClient,
		omaClient:       omaClient,
		defaultPageSize: pagination.DefaultPageSize,
		maxPageSize:     pagination.MaxPageSize,
	}
}
