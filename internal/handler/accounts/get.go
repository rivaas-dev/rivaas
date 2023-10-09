package accounts

import (
	"github.com/Nerzal/gocloak/v13"
	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
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

// KeycloakAccount is used to fetch all groups from Keycloak
type KeycloakAccount struct {
	KeycloakAccountID            *string
	KeycloakAccountSalesforceId  string
	KeycloakCustomerID           *string
	KeycloakCustomerSalesforceId string
	CustomerName                 string
	AccountName                  string
}

// Account is the base data element
type Account struct {
	ID           string    `jsonapi:"primary,accounts"`
	Customers    *Customer `jsonapi:"relation,customer"`
	Name         string    `jsonapi:"attr,name"`
	SalesforceID string    `jsonapi:"attr,salesforceID"`
}

// Customer is in relation to an api account
type Customer struct {
	ID           string `jsonapi:"primary,customer"`
	Name         string `jsonapi:"attr,name"`
	SalesforceID string `jsonapi:"attr,salesforceID"`
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {

	// Fetch
	keycloakGroups, err := h.keycloakClient.GetGroups(ctx)
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
	groups := parseGroups(keycloakGroups)
	// Sort in descending alphabetic order
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].CustomerName < groups[j].CustomerName
	})
	// Build response from groups
	response, err := jsonapi.Marshal(buildResponse(groups))
	if err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusInternalServerError),
				Status: http.StatusInternalServerError,
				Detail: err.Error(),
			},
		)
		return
	}
	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}

// buildResponse Returns a list of Accounts based on the keycloak groups
func buildResponse(groups []*KeycloakAccount) []*Account {
	var accounts []*Account
	// Iterate all keycloak groups
	for _, group := range groups {
		// Add the main level, api account
		account := Account{}
		account.ID = *group.KeycloakAccountID
		account.Name = group.AccountName
		account.SalesforceID = group.KeycloakAccountSalesforceId
		// Add relationship to api account -> customer
		customer := Customer{}
		customer.ID = *group.KeycloakCustomerID
		customer.Name = group.CustomerName
		customer.SalesforceID = group.KeycloakCustomerSalesforceId
		// Add customer
		account.Customers = &customer
		accounts = append(accounts, &account)
	}
	// Return
	return accounts
}

// parseGroups looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func parseGroups(group []*gocloak.Group) []*KeycloakAccount {
	// Return
	var groups []*KeycloakAccount
	// iterate the main groups
	for i := 0; i < len(group); i++ {
		// Set main group variables used for the subgroups
		customerName := group[i].Name
		customerID := group[i].ID
		customerSalesforceId := getSalesforceId(group[i])
		// Iterate subgroup
		sub := *group[i].SubGroups
		for s := 0; s < len(sub); s++ {
			// validate
			if isApiAccount(sub[s]) {
				groups = append(groups, &KeycloakAccount{
					CustomerName:                 *customerName,
					KeycloakCustomerSalesforceId: customerSalesforceId,
					AccountName:                  *sub[s].Name,
					KeycloakAccountSalesforceId:  getSalesforceId(&sub[s]),
					KeycloakAccountID:            sub[s].ID,
					KeycloakCustomerID:           customerID,
				})
			}
		}
	}
	// Return
	return groups
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

func getSalesforceId(group *gocloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}

	return attr.SalesforceID
}
