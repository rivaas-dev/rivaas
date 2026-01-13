package policies

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
)

var quotaPolicyPrefix = "quota-"

type Policy struct {
	ID   string `json:"id" jsonapi:"primary,policies"`
	Name string `json:"name" jsonapi:"attr,name,omitempty"`
}

func (p Policy) String() string {
	return p.ID
}

func ToStringSlice[T fmt.Stringer](slice []T) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		out = append(out, s.String())
	}
	return out
}

func FilterString(policies []string) []string {
	res := make([]string, 0, len(policies))
	for _, p := range policies {
		if strings.HasPrefix(p, quotaPolicyPrefix) {
			continue // we want to skip quota policies, only leave API access policies in
		}

		res = append(res, p)
	}

	sort.Strings(res)
	return res
}

// buildPolicies converts Tyk policies to our internal representation with optional ID filtering
func buildPolicies(policies []tyk.Policy, idFilter map[string]struct{}) []*Policy {
	res := make([]*Policy, 0, len(policies))
	for _, p := range policies {
		// Skip quota policies
		if strings.HasPrefix(p.Id, quotaPolicyPrefix) {
			continue
		}
		// If filter provided, only include requested IDs
		if idFilter != nil {
			if _, ok := idFilter[p.Id]; !ok {
				continue
			}
		}
		res = append(res, &Policy{ID: p.Id, Name: p.Name})
	}
	// Ensure stable order
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res
}

// TykToPolicies converts Tyk policies to our internal representation
func TykToPolicies(policies []tyk.Policy) []*Policy {
	return buildPolicies(policies, nil)
}

// GetPolicies fetch list of policies from Tyk server.
func GetPolicies(ctx context.Context, tykClient *tyk.APIClient) ([]*Policy, error) {
	policies, _, err := tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	return TykToPolicies(policies), nil
}

// GetPoliciesByIDs fetch list of policy full information in the arguments from Tyk server.
func GetPoliciesByIDs(ctx context.Context, tykClient *tyk.APIClient, ids []string) ([]*Policy, error) {
	policies, _, err := tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	// filter out quota IDs and build a set for fast lookup
	filtered := FilterString(ids)
	idSet := make(map[string]struct{}, len(filtered))
	for _, id := range filtered {
		idSet[id] = struct{}{}
	}

	res := buildPolicies(policies, idSet)
	return res, nil
}
