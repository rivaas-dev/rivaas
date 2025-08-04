package customers

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient  keycloak.Client
	keycloakConfig  config.KeyCloakConfig
	omaClient       *oma.Client
	customerService *customers.Service

	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(KeyCloakClient keycloak.Client, KeyCloakConfig config.KeyCloakConfig,
	omaClient *oma.Client,
	customerService *customers.Service,
	pagination config.Pagination,
) *Handler {
	return &Handler{
		keycloakClient:  KeyCloakClient,
		keycloakConfig:  KeyCloakConfig,
		omaClient:       omaClient,
		customerService: customerService,
		defaultPageSize: pagination.DefaultPageSize,
		maxPageSize:     pagination.MaxPageSize,
	}
}
