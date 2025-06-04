package validation

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v1/policies"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// ValidatePolicies validates requested policies.
func ValidatePolicies(ctx *goskell.Context, tykClient *tyk.APIClient, requestedPolicies []string) bool {
	policies, err := policies.GetPolicies(ctx, tykClient)
	if err != nil {
		return false
	}

	policymap := make(map[string]bool)
	for _, policy := range policies {
		policymap[policy.ID] = true
	}

	for _, requestedPolicy := range requestedPolicies {
		if !policymap[requestedPolicy] {
			return false
		}

	}
	return true
}
