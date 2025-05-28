package keycloak

import (
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"sort"
	"strings"
)

const (
	checkType      string = "type"
	checkTypeValue string = "api"
)

// Account is the base data element
type Account struct {
	ID                    string    `jsonapi:"primary,accounts"`
	Customers             *Customer `jsonapi:"relation,customer"`
	Name                  string    `jsonapi:"attr,name"`
	AccountContactDetails []Contact `jsonapi:"attr,contactDetails,omitempty"`
}

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

// ParseAccountGroups looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func ParseAccountGroups(group *keycloak.Group, subGroups []*keycloak.Group) []*Account {
	sort.Slice(subGroups, func(i, j int) bool {
		return *subGroups[i].Name < *subGroups[j].Name
	})

	var accounts []*Account
	// iterate the main groups
	for _, subGroup := range subGroups {
		// Iterate subgroup
		groupAccount := ParseAccountGroup(group, subGroup)
		if groupAccount == nil {
			continue
		}
		accounts = append(accounts, groupAccount)
	}

	return accounts
}

func ParseAccountGroup(group, subGroup *keycloak.Group) *Account {
	// validate
	if group == nil || subGroup == nil {
		return nil
	}

	if !isApiAccount(*subGroup) {
		return nil
	}

	var account Account
	account.ID = *subGroup.ID
	account.Name = *subGroup.Name
	account.AccountContactDetails = getCustomerContactDetails(*subGroup)
	// Add relationship to api account -> customer
	customer := Customer{}
	customer.ID = *group.ID
	customer.Name = *group.Name
	customer.SalesforceID = getSalesforceId(*group)
	customer.CustomerContactDetails = getCustomerContactDetails(*group)
	// Add customer
	account.Customers = &customer

	return &account
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
