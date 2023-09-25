package accounts

import (
	"fmt"
	"github.com/Nerzal/gocloak/v13"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"net/http"
	"sort"
	"strings"
)

const (
	checkType      string = "type"
	checkTypeValue string = "api"
)

type Response struct {
	Groups []Group
}
type Group struct {
	KeycloakAccountId  *string
	KeycloakCustomerId *string
	ActorId            string
	CustomerName       string
}

// GetGroup looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func GetGroup(group []*gocloak.Group) []*Group {
	// Return
	var groups []*Group
	// iterate the main groups
	for i := 0; i < len(group); i++ {
		// Set main group variables used for the subgroups
		customerName := group[i].Name
		customerID := group[i].ID
		// Iterate subgroup
		sub := *group[i].SubGroups
		for s := 0; s < len(sub); s++ {

			// validate
			if isApiAccount(sub[s]) {
				groups = append(groups, &Group{
					CustomerName:       fmt.Sprintf("%s - %s", *customerName, *sub[s].Name),
					KeycloakAccountId:  sub[s].ID,
					KeycloakCustomerId: customerID,
					ActorId:            getActorId(&sub[s]),
				})
			}
		}
	}
	// Return
	return groups
}

func getActorId(group *gocloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}

	return attr.ActorID
}

// isApiAccount determines if the keycloak group is api account
func isApiAccount(group gocloak.Group) bool {
	// Check if there are attributes
	if len(*group.Attributes) > 0 {
		attrMap := *group.Attributes
		for key, value := range attrMap {
			if strings.ToLower(key) == checkType && strings.ToLower(value[0]) == checkTypeValue {
				return true
			}
		}
	}
	return false
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {
	// Fetch
	resp, err := h.keycloakClient.GetGroups(ctx)
	if err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			},
		)
		return
	}

	// Fill the response with the data
	groups := GetGroup(resp)
	// Sort
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].CustomerName < groups[j].CustomerName
	})
	// return
	ctx.JSON(http.StatusOK, groups)
}
