package customers

import (
	"encoding/json"
	"errors"
	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"sort"
	"strings"
)

const (
	checkType      string = "type"
	checkTypeValue string = "api"
)

type AccountExtended struct {
	Account
	Subscription Subscription `json:"subscription,omitempty" jsonapi:"attr,subscription,omitempty"`
}

// MarshalJSON helps marshal embedded struct as a part of Account JSONAPI object
func (a *AccountExtended) MarshalJSON() ([]byte, error) {
	payloaderAccount, err := jsonapi.Marshal(&a.Account)
	if err != nil {
		return nil, err
	}

	payloaderSubscription, err := jsonapi.Marshal(&a.Subscription)
	if err != nil {
		return nil, err
	}

	accountSerialized, ok := payloaderAccount.(*jsonapi.OnePayload)
	if !ok {
		return nil, errors.New("failed to assert jsonapi type")
	}

	subscriptionSerialized, ok := payloaderSubscription.(*jsonapi.OnePayload)
	if !ok {
		return nil, errors.New("failed to assert jsonapi type")
	}

	if len(subscriptionSerialized.Data.Attributes) > 0 {
		accountSerialized.Data.Attributes["subscription"] = subscriptionSerialized.Data.Attributes
	}
	return json.Marshal(accountSerialized)
}

// Account is the base data element
type Account struct {
	ID                    string    `json:"id" jsonapi:"primary,accounts"`
	Customers             *Customer `json:"customers" jsonapi:"relation,customer"`
	Name                  string    `json:"name" jsonapi:"attr,name"`
	AccountContactDetails []Contact `json:"contactDetails" jsonapi:"attr,contactDetails,omitempty"`
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

// GroupsToAccount looks at the first subgroup of the main and tries
// to find groups that have attribute `type` = `api`. It only goes one level down
// from the main group.
func (s *Service) GroupsToAccount(group *keycloak.Group, subGroups []*keycloak.Group) []*Account {
	sort.Slice(subGroups, func(i, j int) bool {
		return *subGroups[i].Name < *subGroups[j].Name
	})

	var accounts []*Account
	// iterate the main groups
	for _, subGroup := range subGroups {
		// Iterate subgroup
		groupAccount := s.GroupToAccount(group, subGroup)
		if groupAccount == nil {
			continue
		}
		accounts = append(accounts, groupAccount)
	}

	return accounts
}

func (s *Service) GroupToAccount(group, subGroup *keycloak.Group) *Account {
	// validate
	if !isGroupValid(group) || !isGroupValid(subGroup) || !isApiAccount(*subGroup) {
		return nil
	}

	// add an account
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

// isApiAccount determines if the keycloak group is an api account
func isApiAccount(group keycloak.Group) bool {
	attrMap := *group.Attributes
	for key, value := range attrMap {
		if strings.ToLower(key) == checkType && strings.ToLower(value[0]) == checkTypeValue {
			return true
		}
	}

	return false
}

func (s *Service) GroupToAccountExtended(group, subGroup *keycloak.Group) (*AccountExtended, error) {
	account := s.GroupToAccount(group, subGroup)
	if account == nil {
		return nil, errors.New("could not convert group to account")
	}

	accountExtended := &AccountExtended{Account: *account}

	// Maximum and current number of API keys
	pricingPlanIDs := (*subGroup.Attributes)[keycloak.PricingPlanIDAttributesKey]
	if len(pricingPlanIDs) > 0 { // price plan ID should be verified and is responsibility of the Provisioning Flow
		var err error
		accountExtended.Subscription, err = s.GetSubscription(group, subGroup, pricingPlanIDs[0])
		if err != nil {
			return nil, err
		}
	}

	return accountExtended, nil
}
