package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/ci-api/cigourn/salesforce"
)

// GetCustomerName compares the keycloak group id with the database key actor id.
func getCustomerName(keycloakGroups []*customers.CustomerResource, key db.Key) string {
	return getCustomerNameByActorID(keycloakGroups, key.ActorID)
}

// GetCustomerNameByActorID compares the keycloak group id with the database key actor id.
func getCustomerNameByActorID(keycloakGroups []*customers.CustomerResource, actorID string) string {
	// Set default as unknown
	customerName := "UNKNOWN"
	// Parse the URN
	parsedURN, err := cigourn.Parse(actorID)
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
		if TypeApi && gr.ID == parsedURN.(*api.Key).CustomerID {
			return gr.Name
		}
		// For salesforce account
		if TypeSalesForce && gr.ID == parsedURN.(*salesforce.Account).AccountID {
			return gr.Name
		}
	}

	return customerName
}
