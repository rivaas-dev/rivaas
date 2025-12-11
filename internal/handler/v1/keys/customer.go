package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/ci-api/cigourn/salesforce"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
)

// GetCustomerName compares the keycloak group id with the database key actor id.
func getCustomerName(customers []customer.Customer, key db.Key) string {
	return getCustomerNameByActorID(customers, key.ActorID)
}

// GetCustomerNameByActorID compares the keycloak group id with the database key actor id.
func getCustomerNameByActorID(customers []customer.Customer, actorID string) string {
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
	for _, c := range customers {
		// For api account
		if TypeApi && c.ID == parsedURN.(*api.Key).CustomerID {
			return c.Name
		}
		// For salesforce account
		if TypeSalesForce && c.ID == parsedURN.(*salesforce.Account).AccountID {
			return c.Name
		}
	}

	return customerName
}
