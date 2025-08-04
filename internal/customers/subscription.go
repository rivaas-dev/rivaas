package customers

import (
	"errors"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"time"
)

type Subscription struct {
	PricingPlanID string   `json:"-"`
	APIKeys       *APIKeys `json:"apiKeys" jsonapi:"attr,apiKeys,omitempty"`
}

type APIKeys struct {
	Production *APIKey `json:"production" jsonapi:"attr,production,omitempty"`
	Sandbox    *APIKey `json:"sandbox" jsonapi:"attr,sandbox,omitempty"`
}

type APIKey struct {
	MaxCount     int `json:"maxCount" jsonapi:"attr,maxCount,omitempty"`
	CurrentCount int `json:"currentCount" jsonapi:"attr,currentCount,omitempty"`
}

type PricingPlan struct {
	Quota                     string
	NumberOfAPIProductionKeys int `mapstructure:"number_of_api_production_keys"`
	NumberOfAPISandboxKeys    int `mapstructure:"number_of_api_sandbox_keys"`
}

func isGroupValid(group *keycloak.Group) bool {
	return group != nil && group.Attributes != nil && len(*group.Attributes) != 0
}

func (s *Service) GetSubscription(group, subGroup *keycloak.Group, customerPricingPlanID string) (Subscription, error) {
	productionMaxCount, sandboxMaxCount, err := getMaxAPIKeyCount(s.pricingPlans, customerPricingPlanID)
	if err != nil {
		return Subscription{}, err
	}

	productionCurrentCount, sandboxCurrentCount, err := s.GetCurrentAPIKeyCount(*group.ID, *subGroup.ID)
	if err != nil {
		return Subscription{}, err
	}

	subscription := Subscription{
		PricingPlanID: customerPricingPlanID,
		APIKeys: &APIKeys{
			Production: &APIKey{
				MaxCount:     productionMaxCount,
				CurrentCount: productionCurrentCount,
			},
			Sandbox: &APIKey{
				MaxCount:     sandboxMaxCount,
				CurrentCount: sandboxCurrentCount,
			},
		},
	}
	return subscription, nil
}

func getMaxAPIKeyCount(pricingPlans map[string]PricingPlan, customerPricingPlanID string) (production int, sandbox int, err error) {
	pricingPlan, ok := pricingPlans[customerPricingPlanID]
	if !ok {
		return 0, 0, errors.New("customer pricing plan not found")
	}

	return pricingPlan.NumberOfAPIProductionKeys, pricingPlan.NumberOfAPISandboxKeys, nil
}

func (s *Service) GetCurrentAPIKeyCount(customerID, accountID string) (production int, sandbox int, err error) {
	params := db.SearchParams{
		FilterParams: db.FilterParams{
			Active:       db.Pointer(true),
			ExpiresAfter: &date.Date{Time: time.Now()},
		},
		MatchParams: db.MatchParams{
			CustomerID: customerID,
			AccountID:  accountID,
		},
	}
	counts, err := s.keysRepository.GetKeysCountPerEnvironment(params)
	if err != nil {
		return 0, 0, err
	}

	production = int(counts[db.ProdEnv])
	sandbox = int(counts[db.SandboxEnv])
	return production, sandbox, nil
}
