package validation

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/policies"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// ValidatePolicies validates requested policies.
func ValidatePolicies(ctx *goskell.Context, tykClient *tyk.APIClient, requestedPolicies []string) bool {
	policies, err := policies.GetPolicies(ctx, tykClient)
	if err != nil {
		return false
	}
	for _, requestedPolicy := range requestedPolicies {
		for _, policy := range policies {
			if policy == requestedPolicy {
				return true
			}
		}
	}
	return false
}
