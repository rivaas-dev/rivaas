package customers

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
)

// SubscriptionService handles subscription-related operations
type SubscriptionService struct {
	keysRepository db.DatabaseExecer
	pricingPlans   map[string]PricingPlan
}

// Subscription represents a customer's subscription information
type Subscription struct {
	PricingPlanID string   `json:"-"`
	APIKeys       *APIKeys `json:"apiKeys" jsonapi:"attr,apiKeys,omitempty"`
}

// APIKeys represents API key limits for different environments
type APIKeys struct {
	Production *APIKey `json:"production" jsonapi:"attr,production,omitempty"`
	Sandbox    *APIKey `json:"sandbox" jsonapi:"attr,sandbox,omitempty"`
}

// APIKey represents API key limits and current usage
type APIKey struct {
	MaxCount     int `json:"maxCount" jsonapi:"attr,maxCount,omitempty"`
	CurrentCount int `json:"currentCount" jsonapi:"attr,currentCount,omitempty"`
}

// PricingPlan represents a pricing plan configuration
type PricingPlan struct {
	QuotaPolicyID             string `conflex:"quota" mapstructure:"quota"`
	NumberOfAPIProductionKeys int    `mapstructure:"number_of_api_production_keys"`
	NumberOfAPISandboxKeys    int    `mapstructure:"number_of_api_sandbox_keys"`
}

// NewSubscriptionService creates a new SubscriptionService
func NewSubscriptionService(keysRepository db.DatabaseExecer, pricingPlans map[string]PricingPlan) *SubscriptionService {
	// Debug logging: Log all loaded pricing plans at initialization
	log.Info().
		Int("totalPlans", len(pricingPlans)).
		Msg("initializing SubscriptionService with pricing plans")

	for planID, plan := range pricingPlans {
		log.Debug().
			Str("planID", planID).
			Str("quotaPolicyName", plan.QuotaPolicyID).
			Int("numberOfAPIProductionKeys", plan.NumberOfAPIProductionKeys).
			Int("numberOfAPISandboxKeys", plan.NumberOfAPISandboxKeys).
			Msg("loaded pricing plan from config")
	}

	return &SubscriptionService{
		keysRepository: keysRepository,
		pricingPlans:   pricingPlans,
	}
}

// GetSubscription retrieves subscription information for a customer including API key limits,
// current usage, and quota information.
func (s *SubscriptionService) GetSubscription(ctx context.Context, customerID, accountID, pricingPlanID string) (Subscription, error) {
	if customerID == "" {
		return Subscription{}, ErrInvalidCustomerID
	}
	if accountID == "" {
		return Subscription{}, ErrInvalidAccountID
	}
	if pricingPlanID == "" {
		return Subscription{}, fmt.Errorf("%w: pricing plan ID cannot be empty", ErrPricingPlanNotFound)
	}

	productionMaxCount, sandboxMaxCount, err := s.getMaxAPIKeyCount(pricingPlanID)
	if err != nil {
		return Subscription{}, fmt.Errorf("failed to get API key limits: %w", err)
	}

	// Get current API key counts
	productionCurrentCount, sandboxCurrentCount, err := s.GetCurrentAPIKeyCount(customerID, accountID)
	if err != nil {
		return Subscription{}, fmt.Errorf("failed to get current API key counts: %w", err)
	}

	subscription := Subscription{
		PricingPlanID: pricingPlanID,
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

// GetCurrentAPIKeyCount retrieves the current count of active API keys for a customer
// separated by environment (production and sandbox).
func (s *SubscriptionService) GetCurrentAPIKeyCount(customerID, accountID string) (production int, sandbox int, err error) {
	if customerID == "" {
		return 0, 0, ErrInvalidCustomerID
	}
	if accountID == "" {
		return 0, 0, ErrInvalidAccountID
	}

	params := db.SearchParams{
		FilterParams: db.FilterParams{
			Active:       db.Pointer(true),
			ExpiresAfter: db.Pointer(time.Now()),
			Deleted:      db.Pointer(false),
		},
		MatchParams: db.MatchParams{
			CustomerID: customerID,
			AccountID:  accountID,
		},
	}

	counts, err := s.keysRepository.GetKeysCountPerEnvironment(params)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get API key counts from database: %w", err)
	}

	production = int(counts[db.ProdEnv])
	sandbox = int(counts[db.SandboxEnv])
	return production, sandbox, nil
}

// GetPricingPlanQuotaPolicy retrieves the quota policy from a pricing plan.
func (s *SubscriptionService) GetPricingPlanQuotaPolicy(pricingPlanID string) (policies.Policy, error) {
	if pricingPlanID == "" {
		return policies.Policy{}, fmt.Errorf("%w: pricing plan ID cannot be empty", ErrPricingPlanNotFound)
	}

	// Debug logging: Show what we're looking for and what's available
	log.Debug().
		Str("requestedPlanID", pricingPlanID).
		Int("availablePlans", len(s.pricingPlans)).
		Msg("attempting to retrieve pricing plan quota policy name")

	pricingPlan, exists := s.pricingPlans[pricingPlanID]
	if !exists {
		// Only error out - no automatic fallback to "custom"
		// The fallback to "custom" should happen in accounts.go/account_service.go
		// when Keycloak doesn't provide a pricing plan ID at all
		if pricingPlanID == "custom" {
			log.Error().
				Str("pricingPlanID", pricingPlanID).
				Msg("'custom' pricing plan not found in Consul configuration")
			return policies.Policy{}, fmt.Errorf("%w: 'custom' pricing plan not available in configuration", ErrPricingPlanNotFound)
		}

		log.Error().
			Str("pricingPlanID", pricingPlanID).
			Msg("pricing plan from Keycloak not found in Consul configuration")
		return policies.Policy{}, fmt.Errorf("%w: pricing plan '%s' is not configured in Consul", ErrPricingPlanNotFound, pricingPlanID)
	}

	// Debug logging: Show the exact structure of the found pricing plan
	log.Debug().
		Str("pricingPlanID", pricingPlanID).
		Str("quotaPolicyName", pricingPlan.QuotaPolicyID).
		Int("numberOfAPIProductionKeys", pricingPlan.NumberOfAPIProductionKeys).
		Int("numberOfAPISandboxKeys", pricingPlan.NumberOfAPISandboxKeys).
		Bool("quotaIsEmpty", pricingPlan.QuotaPolicyID == "").
		Msg("retrieved pricing plan from map - checking quota policy name")

	if pricingPlan.QuotaPolicyID == "" {
		log.Error().
			Str("pricingPlanID", pricingPlanID).
			Str("quotaPolicyName", pricingPlan.QuotaPolicyID).
			Int("numberOfAPIProductionKeys", pricingPlan.NumberOfAPIProductionKeys).
			Int("numberOfAPISandboxKeys", pricingPlan.NumberOfAPISandboxKeys).
			Msg("pricing plan has empty quota policy name - check Consul configuration")
		return policies.Policy{}, fmt.Errorf("%w: quota policy name is not configured for pricing plan '%s'", ErrPricingPlanNotFound, pricingPlanID)
	}

	return policies.Policy{
		ID: pricingPlan.QuotaPolicyID,
	}, nil
}

// getMaxAPIKeyCount retrieves the maximum allowed API key counts for production and sandbox
// environments from the customer's pricing plan.
func (s *SubscriptionService) getMaxAPIKeyCount(pricingPlanID string) (production int, sandbox int, err error) {
	if s.pricingPlans == nil {
		return 0, 0, fmt.Errorf("%w: pricing plans configuration is not available", ErrPricingPlanNotFound)
	}
	if pricingPlanID == "" {
		return 0, 0, fmt.Errorf("%w: pricing plan ID cannot be empty", ErrPricingPlanNotFound)
	}

	pricingPlan, ok := s.pricingPlans[pricingPlanID]
	if !ok {
		// Only error out - no automatic fallback to "custom"
		// The fallback to "custom" should happen in accounts.go/account_service.go
		// when Keycloak doesn't provide a pricing plan ID at all
		if pricingPlanID == "custom" {
			log.Error().
				Str("pricingPlanID", pricingPlanID).
				Msg("'custom' pricing plan not found in Consul configuration")
			return 0, 0, fmt.Errorf("%w: 'custom' pricing plan not available in configuration", ErrPricingPlanNotFound)
		}

		log.Error().
			Str("pricingPlanID", pricingPlanID).
			Msg("pricing plan from Keycloak not found in Consul configuration")
		return 0, 0, fmt.Errorf("%w: pricing plan '%s' is not configured in Consul", ErrPricingPlanNotFound, pricingPlanID)
	}

	return pricingPlan.NumberOfAPIProductionKeys, pricingPlan.NumberOfAPISandboxKeys, nil
}
