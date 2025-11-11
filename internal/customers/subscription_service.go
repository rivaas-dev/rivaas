package customers

import (
	"context"
	"fmt"
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
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
	QuotaPolicyName           string `mapstructure:"quota"`
	NumberOfAPIProductionKeys int    `mapstructure:"number_of_api_production_keys"`
	NumberOfAPISandboxKeys    int    `mapstructure:"number_of_api_sandbox_keys"`
}

// NewSubscriptionService creates a new SubscriptionService
func NewSubscriptionService(keysRepository db.DatabaseExecer, pricingPlans map[string]PricingPlan) *SubscriptionService {
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

// GetPricingPlanQuotaPolicyName retrieves the quota policy name from a pricing plan.
func (s *SubscriptionService) GetPricingPlanQuotaPolicyName(pricingPlanID string) (string, error) {
	if pricingPlanID == "" {
		return "", fmt.Errorf("%w: pricing plan ID cannot be empty", ErrPricingPlanNotFound)
	}

	pricingPlan, exists := s.pricingPlans[pricingPlanID]
	if !exists {
		return "", fmt.Errorf("%w: pricing plan '%s' not found", ErrPricingPlanNotFound, pricingPlanID)
	}

	if pricingPlan.QuotaPolicyName == "" {
		return "", fmt.Errorf("%w: quota policy name is not configured for pricing plan '%s'", ErrPricingPlanNotFound, pricingPlanID)
	}

	return pricingPlan.QuotaPolicyName, nil
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
		return 0, 0, fmt.Errorf("%w: pricing plan '%s' not found", ErrPricingPlanNotFound, pricingPlanID)
	}

	return pricingPlan.NumberOfAPIProductionKeys, pricingPlan.NumberOfAPISandboxKeys, nil
}
