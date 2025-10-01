package customers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"
	"sort"
	"strings"
)

const (
	checkType      string = "type"
	checkTypeValue string = "api"
)

var ErrInvalidGroup = errors.New("invalid group or sub group")

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
	Customers             *Customer `json:"customers" jsonapi:"relation,customers"`
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
func (s *Service) GroupsToAccount(group *keycloak.Group, subGroups []*keycloak.Group) ([]*Account, error) {
	sort.Slice(subGroups, func(i, j int) bool {
		return *subGroups[i].Name < *subGroups[j].Name
	})

	var accounts []*Account
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

func (s *Service) GroupToAccount(group, subGroup *keycloak.Group) (*Account, error) {
	// validate
	if !isGroupValid(group) || !isGroupValid(subGroup) || !isApiAccount(*subGroup) {
		return nil, ErrInvalidGroup
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

	return &account, nil
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
	account, err := s.GroupToAccount(group, subGroup)
	if err != nil {
		return nil, err
	}

	accountExtended := &AccountExtended{Account: *account}

	// Maximum and current number of API keys
	pricingPlanIDs := (*subGroup.Attributes)[keycloak.PricingPlanIDAttributesKey]
	if len(pricingPlanIDs) == 0 {
		return nil, errors.New("no pricing plan found")
	}

	if len(pricingPlanIDs) > 1 {
		return nil, fmt.Errorf("more than one pricing plan found: %d", len(pricingPlanIDs))
	}

	accountExtended.Subscription, err = s.GetSubscription(group, subGroup, pricingPlanIDs[0])
	if err != nil {
		return nil, err
	}

	return accountExtended, nil
}

func (s *Service) GetAccountExtended(ctx context.Context, customerID, accountID string) (*AccountExtended, error) {
	_, span := goot.Span(ctx, "get_group_from_keycloak",
		attribute.String("customerID", customerID),
	)
	group, err := s.keycloakClient.GetGroupByID(ctx, customerID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to retrieve keycloak group")
		return nil, err
	}
	goot.EndSpan(span)

	_, span = goot.Span(ctx, "get_subgroup_from_keycloak",
		attribute.String("accountID", accountID),
	)
	subgroup, err := s.keycloakClient.GetSubGroupByID(*group, accountID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to retrieve keycloak subgroup")
		return nil, err
	}
	goot.EndSpan(span)

	//Parse the group data
	_, span = goot.Span(ctx, "group_to_account")
	account, err := s.GroupToAccountExtended(group, subgroup)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to convert groups to customer")
		return nil, err
	}
	goot.EndSpan(span)

	return account, nil
}
