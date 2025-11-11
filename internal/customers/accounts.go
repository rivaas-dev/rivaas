package customers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

const (
	checkType      string = "type"
	checkTypeValue string = "api"
)

var ErrInvalidGroup = errors.New("invalid group or sub group")

// KeycloakAccount is used to fetch all groups from Keycloak and later
// parsed to Account / Customer json api structs for output
type KeycloakAccount struct {
	KeycloakAccountID            *string
	KeycloakAccountSalesforceID  string
	KeycloakCustomerID           *string
	KeycloakCustomerSalesforceID string
	CustomerName                 string
	AccountName                  string
	CustomerContactDetails       []Contact //customer level contacts
	AccountContactDetails        []Contact //account level contacts
}

// GroupsToAccount looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func (s *Service) GroupsToAccount(group *keycloak.Group, subGroups []*keycloak.Group) ([]*AccountResource, error) {
	sort.Slice(subGroups, func(i, j int) bool {
		return *subGroups[i].Name < *subGroups[j].Name
	})

	var accounts []*AccountResource
	// iterate the main groups
	for _, subGroup := range subGroups {
		// Iterate subgroup
		groupAccount, err := s.GroupToAccount(group, subGroup)

		if errors.Is(ErrInvalidGroup, err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if groupAccount == nil {
			continue
		}
		accounts = append(accounts, groupAccount)
	}

	return accounts, nil
}

// GroupToAccount converts a Keycloak group to an AccountResource
func (s *Service) GroupToAccount(group, subGroup *keycloak.Group) (*AccountResource, error) {
	// validate
	if !isGroupValid(group) || !isGroupValid(subGroup) || !isAPIAccount(*subGroup) {
		return nil, ErrInvalidGroup
	}

	// add an account
	var account AccountResource
	account.ID = *subGroup.ID
	account.Name = *subGroup.Name

	account.AccountContactDetails = extractContactDetails(*subGroup)

	// Add relationship to api account -> customer
	customer := CustomerResource{}
	customer.ID = *group.ID
	customer.Name = *group.Name
	customer.SalesforceID = extractSalesforceID(*group)
	customer.CustomerContactDetails = extractContactDetails(*group)

	// Add customer
	account.Customers = &customer

	return &account, nil
}

// isAPIAccount determines if the keycloak group is an api account
func isAPIAccount(group keycloak.Group) bool {
	if group.Attributes == nil {
		return false
	}

	attrMap := *group.Attributes
	for key, values := range attrMap {
		if len(values) > 0 && strings.ToLower(key) == checkType && strings.ToLower(values[0]) == checkTypeValue {
			return true
		}
	}

	return false
}

func (s *Service) GroupToAccountExtended(group, subGroup *keycloak.Group) (*AccountExtended, error) {
	account, err := s.GroupToAccount(group, subGroup)
	if err != nil {
		return nil, err
	}

	accountExtended := &AccountExtended{AccountResource: *account}

	// Maximum and current number of API keys
	pricingPlanIDs := (*subGroup.Attributes)[keycloak.PricingPlanIDAttributesKey]
	if len(pricingPlanIDs) == 0 {
		return nil, ErrPricingPlanNotFound
	}

	if len(pricingPlanIDs) > 1 {
		return nil, fmt.Errorf("more than one pricing plan found: %d", len(pricingPlanIDs))
	}

	accountExtended.Subscription, err = s.GetSubscription(context.Background(), *group.ID, *subGroup.ID, pricingPlanIDs[0])
	if err != nil {
		return nil, err
	}

	return accountExtended, nil
}

// extractSalesforceID extracts Salesforce ID from Keycloak group attributes
func extractSalesforceID(group keycloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	return attr.SalesforceID
}

// extractContactDetails extracts contact details from Keycloak group attributes
func extractContactDetails(group keycloak.Group) []Contact {
	var contacts []Contact
	customer, err := keycloak.ToSubGroupAttributes(*group.Attributes)
	if err != nil {
		return contacts
	}

	for contactID, contact := range customer.ContactDetails {
		contacts = append(contacts, Contact{
			ID:      contactID,
			Contact: contact,
		})
	}
	return contacts
}
