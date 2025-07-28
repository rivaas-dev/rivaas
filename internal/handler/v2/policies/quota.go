package policies

import (
	"context"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"sort"
	"strings"
)

var quotaPolicyPrefix = "quota-"

func FilterString(policies []string) []string {
	res := make([]string, 0, len(policies))
	for _, p := range policies {
		if strings.HasPrefix(p, quotaPolicyPrefix) {
			continue // we want to skip quota policies, only leave API access policies in
		}

		res = append(res, p)
	}

	return res
}

// GetPolicies fetch list of policies from Tyk server.
func GetPolicies(ctx context.Context, tykClient *tyk.APIClient) ([]*Policy, error) {
	policies, _, err := tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]*Policy, 0, len(policies))
	for _, p := range policies {
		if strings.HasPrefix(p.Id, quotaPolicyPrefix) {
			continue // we want to skip quota policies, only leave API access policies in
		}

		res = append(res, &Policy{
			ID:   p.Id,
			Name: p.Name,
		})
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res, err
}
