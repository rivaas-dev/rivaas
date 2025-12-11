package accounts

import (
	"net/http"
	"sort"

	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
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
	customer.Contact
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {

	// Fetch
	_, span := goot.Span(ctx.Request.Context(), "get_from_keycloak")
	customers, err := h.customerService.ListCustomers(ctx, customer.ListParams{})
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
	responseAccounts := buildResponse(customers)
	// Sort in descending alphabetic order
	sort.Slice(responseAccounts, func(i, j int) bool {
		if responseAccounts[i].Customers == nil || responseAccounts[j].Customers == nil {
			return false // just prevent a panic and keep backward compatibility
		}
		return responseAccounts[i].Customers.Name < responseAccounts[j].Customers.Name
	})
	// Build response from groups
	response, err := jsonapi.Marshal(responseAccounts)
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
func buildResponse(customers []customer.Customer) []*Account {
	var accounts []*Account
	// Iterate all keycloak groups
	for _, c := range customers {
		for _, a := range c.Accounts {
			// skip non-api accounts
			if a.Type != checkTypeValue {
				continue
			}

			// Add the main level, api account
			account := Account{}
			account.ID = a.ID
			account.Name = a.Name
			account.AccountContactDetails = getCustomerContactDetails(a.SalesforceContactDetails)
			// Add relationship to api account -> customer
			accountCustomer := Customer{}
			accountCustomer.ID = c.ID
			accountCustomer.Name = c.Name
			accountCustomer.SalesforceID = c.SalesforceID
			accountCustomer.CustomerContactDetails = getCustomerContactDetails(c.Contacts)
			// Add customer
			account.Customers = &accountCustomer
			accounts = append(accounts, &account)
		}
	}
	// Return
	return accounts
}

func getCustomerContactDetails(contacts map[string]customer.Contact) []Contact {
	var out []Contact
	for contactID, contact := range contacts {
		out = append(out, Contact{
			ID:      contactID,
			Contact: contact,
		})
	}
	return out
}
