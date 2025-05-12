package accounts

import (
	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"net/http"
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
	AccountContactDetails []Contact `jsonapi:"attr,contactDetails,omitempty"`
}

// Customer is in relation to an api account
type Customer struct {
	ID                     string    `jsonapi:"primary,customer"`
	Name                   string    `jsonapi:"attr,name"`
	SalesforceID           string    `jsonapi:"attr,salesforceID,omitempty"`
	CustomerContactDetails []Contact `jsonapi:"attr,contactDetails,omitempty"`
}

type Contact struct {
	ID string `jsonapi:"id" json:"id" binding:"required"`
	keycloak.Contact
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {

	// Fetch
	_, span := goot.Span(ctx.Request.Context(), "get_from_keycloak")
	groups, err := h.keycloakClient.GetGroups(ctx, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups := parseGroups(groups)

	// Build response from groups
	response, err := jsonapi.Marshal(parsedKeyCloakGroups)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
