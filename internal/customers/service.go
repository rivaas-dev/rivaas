package customers

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

type Service struct {
	keysRepository db.DatabaseExecer
	keycloakClient keycloak.Client
	pricingPlans   map[string]PricingPlan
}

// New constructs a new Service.
func New(
	keysRepository db.DatabaseExecer,
	keycloakClient keycloak.Client,
	pricingPlans map[string]PricingPlan,
) *Service {
	return &Service{
		keysRepository: keysRepository,
		keycloakClient: keycloakClient,
		pricingPlans:   pricingPlans,
	}
}
