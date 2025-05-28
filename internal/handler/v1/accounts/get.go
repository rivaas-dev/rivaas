package accounts

import (
	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
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

// KeycloakAccount is used to fetch all groups from Keycloak and later
// parsed to Account / Customer json api structs for output
type KeycloakAccount struct {
	KeycloakAccountID            *string
	KeycloakAccountSalesforceId  string
	KeycloakCustomerID           *string
	KeycloakCustomerSalesforceId string
	CustomerName                 string
	AccountName                  string
	CustomerContactDetails       []Contact //customer level contacts
	AccountContactDetails        []Contact //account level contacts
}

// Account is the base data element
type Account struct {
	ID                    string    `jsonapi:"primary,accounts"`
	Customers             *Customer `jsonapi:"relation,customer"`
	Name                  string    `jsonapi:"attr,name"`
	AccountContactDetails []Contact `jsonapi:"attr,contactDetails"`
}

// Customer is in relation to an api account
type Customer struct {
	ID                     string    `jsonapi:"primary,customer"`
	Name                   string    `jsonapi:"attr,name"`
	SalesforceID           string    `jsonapi:"attr,salesforceID"`
	CustomerContactDetails []Contact `jsonapi:"attr,contactDetails"`
}

type Contact struct {
	ID string `jsonapi:"id" json:"id" binding:"required"`
	keycloak.Contact
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {

	// Fetch
	_, span := goot.Span(ctx.Request.Context(), "get_from_keycloak")
	groups, err := h.keycloakClient.GetGroupsPaginated(ctx, nil, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
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
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups := parseGroups(groups)
	// Sort in descending alphabetic order
	sort.Slice(parsedKeyCloakGroups, func(i, j int) bool {
		return parsedKeyCloakGroups[i].CustomerName < parsedKeyCloakGroups[j].CustomerName
	})
	// Build response from groups
	response, err := jsonapi.Marshal(buildResponse(parsedKeyCloakGroups))
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

// parseGroups looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func parseGroups(group []*keycloak.Group) []*KeycloakAccount {
	// Return
	var accounts []*KeycloakAccount
	// iterate the main groups
	for i := 0; i < len(group); i++ {
		// Set main group variables used for the subgroups
		customerName := group[i].Name
		customerID := group[i].ID
		customerSalesforceId := getSalesforceId(*group[i])
		customerContacts := getCustomerContactDetails(*group[i])
		// Iterate subgroup
		sub := *group[i].SubGroups
		for s := 0; s < len(sub); s++ {
			if sub[s] == nil {
				continue
			}

			// validate
			if isApiAccount(*sub[s]) {
				accounts = append(accounts, &KeycloakAccount{
					CustomerName:                 *customerName,
					KeycloakCustomerSalesforceId: customerSalesforceId,
					AccountName:                  *sub[s].Name,
					KeycloakAccountSalesforceId:  getSalesforceId(*sub[s]),
					KeycloakAccountID:            sub[s].ID,
					KeycloakCustomerID:           customerID,
					CustomerContactDetails:       customerContacts,
					AccountContactDetails:        getCustomerContactDetails(*sub[s]),
				})
			}
		}
	}
	// Return
	return accounts
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
		account.AccountContactDetails = group.AccountContactDetails
		// Add relationship to api account -> customer
		customer := Customer{}
		customer.ID = *group.KeycloakCustomerID
		customer.Name = group.CustomerName
		customer.SalesforceID = group.KeycloakCustomerSalesforceId
		customer.CustomerContactDetails = group.CustomerContactDetails
		// Add customer
		account.Customers = &customer
		accounts = append(accounts, &account)
	}
	// Return
	return accounts
}

// isApiAccount determines if the keycloak group is api account
func isApiAccount(group keycloak.Group) bool {
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

func getSalesforceId(group keycloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	return attr.SalesforceID
}

func getCustomerContactDetails(group keycloak.Group) []Contact {

	var contacts []Contact
	customer, _ := keycloak.ToSubGroupAttributes(*group.Attributes)
	for contactID, contact := range customer.ContactDetails {
		contacts = append(contacts, Contact{
			ID:      contactID,
			Contact: contact,
		})
	}
	return contacts
}
