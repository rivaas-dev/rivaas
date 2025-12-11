package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"go.companyinfo.dev/keycloak"
)

// Handler handles keys requests
type Handler struct {
	keycloakClient  *keycloak.Client
	keycloakConfig  config.KeyCloakConfig
	solvimonClient  *solvimon.Client
	omaClient       *oma.Client
	customerService *customers.Service

	pricingPlans    map[string]customers.PricingPlan
	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(keyCloakClient *keycloak.Client, keyCloakConfig config.KeyCloakConfig,
	solvimonClient *solvimon.Client,
	omaClient *oma.Client,
	customerService *customers.Service,
	pagination config.Pagination,
	pricingPlans map[string]customers.PricingPlan,
) *Handler {
	return &Handler{
		keycloakClient:  keyCloakClient,
		keycloakConfig:  keyCloakConfig,
		solvimonClient:  solvimonClient,
		omaClient:       omaClient,
		customerService: customerService,
		defaultPageSize: pagination.DefaultPageSize,
		maxPageSize:     pagination.MaxPageSize,
		pricingPlans:    pricingPlans,
	}
}
