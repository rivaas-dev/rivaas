package internal

import (
	"github.com/Nerzal/gocloak/v13"
	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/ci-api/cigourn/salesforce"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// WriteJSONAPIResponse writes a json api response.
func WriteJSONAPIResponse(ctx *goskell.Context, body any, statusCode int, headers map[string]string) {
	for key, value := range headers {
		ctx.Header(key, value)
	}
	ctx.Header("Content-Type", jsonapi.MediaType)
	ctx.JSON(statusCode, body)
}

// GetCustomerName compares the keycloak group id with the database key actor id.
func GetCustomerName(keycloakGroups []*gocloak.Group, key db.Key) string {
	// Set default as unknown
	customerName := "UNKNOWN"
	// Parse the URN
	parsedURN, err := cigourn.Parse(key.ActorID)
	if err != nil {
		return customerName
	}
	// Check if the parsed urn is of account type api key or salesforce
	_, TypeApi := parsedURN.(*api.Key)
	_, TypeSalesForce := parsedURN.(*salesforce.Account)
	// Validate
	if !TypeApi && !TypeSalesForce {
		return customerName
	}
	// Check in the keycloak groups for a matching customerId on api.key & salesforce.account
	for _, gr := range keycloakGroups {
		// For api account
		if TypeApi && *gr.ID == parsedURN.(*api.Key).CustomerID {
			return *gr.Name
		}
		// For salesforce account
		if TypeSalesForce && *gr.ID == parsedURN.(*salesforce.Account).AccountID {
			return *gr.Name
		}
	}

	return customerName
}
