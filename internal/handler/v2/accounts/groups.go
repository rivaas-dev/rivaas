package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"sort"
	"strings"
)

// parseGroups looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func parseGroups(groups []*keycloak.Group) []*Account {
	sort.Slice(groups, func(i, j int) bool {
		return *groups[i].Name < *groups[j].Name
	})

	var accounts []*Account
	// iterate the main groups
	for _, group := range groups {
		// Iterate subgroup
		groupAccounts := parseGroup(group)
		accounts = append(accounts, groupAccounts...)
	}

	return accounts
}

func parseGroup(group *keycloak.Group) []*Account {
	var accounts []*Account

	// Iterate subgroup
	for _, subGroup := range *group.SubGroups {
		// validate
		if isApiAccount(subGroup) {
			account := Account{}
			account.ID = *subGroup.ID
			account.Name = *subGroup.Name
			account.AccountContactDetails = getCustomerContactDetails(subGroup)
			// Add relationship to api account -> customer
			customer := Customer{}
			customer.ID = *group.ID
			customer.Name = *group.Name
			customer.SalesforceID = getSalesforceId(*group)
			customer.CustomerContactDetails = getCustomerContactDetails(*group)
			// Add customer
			account.Customers = &customer
			accounts = append(accounts, &account)
		}
	}

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
