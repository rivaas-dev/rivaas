package keys

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
)

func (h *Handler) addQuotaPolicies(ctx context.Context, customerID, accountID string, requestPolicies []string) ([]string, error) {
	// Get pricing plan ID from account extended info
	accountExt, err := h.customerService.GetAccountExtended(ctx, customerID, accountID)
	if err != nil {
		return nil, err
	}

	quotaPolicyName, err := h.customerService.GetPricingPlanQuotaPolicyName(accountExt.Subscription.PricingPlanID)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("pricingPlanID", accountExt.Subscription.PricingPlanID).
		Str("quotaPolicyName", quotaPolicyName).
		Msg("retrieved quota policy name for API key creation")

	if quotaPolicyName != "" {
		requestPolicies = append(requestPolicies, quotaPolicyName)
	} else {
		log.Warn().
			Str("pricingPlanID", accountExt.Subscription.PricingPlanID).
			Msg("quota policy name is empty, not adding to policies")
	}

	return requestPolicies, nil
}

func (h *Handler) addQuotaPoliciesWithActorID(ctx context.Context, actorID string, requestPolicies []string) ([]string, error) {
	customerURN, err := cigourn.Parse(actorID)
	if err != nil {
		return nil, fmt.Errorf("invalid actor id: %w", err)
	}

	apiKeyURN, ok := customerURN.(*api.Key)
	if !ok {
		return nil, errors.New("invalid authorization format")
	}

	return h.addQuotaPolicies(ctx, apiKeyURN.CustomerID, apiKeyURN.AccountID, requestPolicies)
}
