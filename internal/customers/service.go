package customers

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
)

type Service struct {
	pricingPlans   map[string]PricingPlan
	keysRepository db.DatabaseExecer
}

// New constructs a new Service.
func New(
	keysRepository db.DatabaseExecer,
	pricingPlans map[string]PricingPlan,
) *Service {
	return &Service{
		keysRepository: keysRepository,
		pricingPlans:   pricingPlans,
	}
}
