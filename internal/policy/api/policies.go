package api

import (
	"context"
	"sort"
)

func ListPolicies(ctx context.Context, clientInterface ClientInterface) ([]string, error) {
	policies, _, err := clientInterface.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(policies))
	for _, p := range policies {
		res = append(res, p.Id)
	}

	// we want our API response to be consistent
	sort.Strings(res)
	return res, err
}
