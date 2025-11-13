package customers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"
)

// AccountService handles account-related operations
type AccountService struct {
	dataProvider        CustomerProvider
	subscriptionService *SubscriptionService
}

// NewAccountService creates a new AccountService
func NewAccountService(dataProvider CustomerProvider, subscriptionService *SubscriptionService) *AccountService {
	return &AccountService{
		dataProvider:        dataProvider,
		subscriptionService: subscriptionService,
	}
}

// GetAccount retrieves an account by customer ID and account ID
func (s *AccountService) GetAccount(ctx context.Context, customerID, accountID string) (*AccountResource, error) {
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}
	if accountID == "" {
		return nil, ErrInvalidAccountID
	}

	accountData, err := s.dataProvider.GetAccount(ctx, customerID, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account data: %w", err)
	}

	return s.accountToResource(accountData), nil
}

// GetAccountExtended retrieves an account with subscription information
func (s *AccountService) GetAccountExtended(ctx context.Context, customerID, accountID string) (*AccountExtended, error) {
	_, span := goot.Span(ctx, "get_account_extended",
		attribute.String("customerID", customerID),
		attribute.String("accountID", accountID),
	)
	defer goot.EndSpan(span)

	accountData, err := s.dataProvider.GetAccount(ctx, customerID, accountID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to get account data")
		return nil, fmt.Errorf("failed to get account data: %w", err)
	}

	account := s.accountToResource(accountData)

	// Get subscription information
	pricingPlanID := "custom" // Default fallback
	if len(accountData.PricingPlanIDs) == 0 {
		log.Warn().
			Str("customerID", customerID).
			Str("accountID", accountID).
			Msg("no pricing plan found in Keycloak, using 'custom' as fallback")
	} else if len(accountData.PricingPlanIDs) > 1 {
		return nil, fmt.Errorf("more than one pricing plan found: %d", len(accountData.PricingPlanIDs))
	} else {
		pricingPlanID = accountData.PricingPlanIDs[0]
	}

	subscription, err := s.subscriptionService.GetSubscription(ctx, customerID, accountID, pricingPlanID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to get subscription")
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return &AccountExtended{
		AccountResource: *account,
		Subscription:    subscription,
	}, nil
}

// ListAccounts retrieves all accounts for a customer
func (s *AccountService) ListAccounts(ctx context.Context, customerID string) ([]*AccountResource, error) {
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}

	accountsData, err := s.dataProvider.ListCustomerAccounts(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	var accounts []*AccountResource
	for _, accountData := range accountsData {
		accounts = append(accounts, s.accountToResource(accountData))
	}

	return accounts, nil
}

// accountToResource converts Account to AccountResource
func (s *AccountService) accountToResource(data *Account) *AccountResource {
	var contacts []Contact
	for _, contactData := range data.Contacts {
		contacts = append(contacts, Contact{
			ID: contactData.ID,
			Contact: keycloak.Contact{
				Email: contactData.Email,
			},
		})
	}

	// Create customer relationship
	customer := &CustomerResource{
		ID:           data.CustomerID,
		Name:         data.CustomerName,
		SalesforceID: data.CustomerSalesforceID,
	}

	return &AccountResource{
		ID:                    data.ID,
		Name:                  data.Name,
		Customers:             customer,
		AccountContactDetails: contacts,
	}
}

// AccountExtended represents an account with subscription information
type AccountExtended struct {
	AccountResource
	Subscription Subscription `json:"subscription,omitempty" jsonapi:"attr,subscription,omitempty"`
}

// MarshalJSON helps marshal embedded struct as a part of AccountResource JSONAPI object
func (a *AccountExtended) MarshalJSON() ([]byte, error) {
	payloaderAccount, err := jsonapi.Marshal(&a.AccountResource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal account: %w", err)
	}

	payloaderSubscription, err := jsonapi.Marshal(&a.Subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subscription: %w", err)
	}

	accountSerialized, ok := payloaderAccount.(*jsonapi.OnePayload)
	if !ok {
		return nil, errors.New("failed to assert account jsonapi type")
	}

	subscriptionSerialized, ok := payloaderSubscription.(*jsonapi.OnePayload)
	if !ok {
		return nil, errors.New("failed to assert subscription jsonapi type")
	}

	if len(subscriptionSerialized.Data.Attributes) > 0 {
		accountSerialized.Data.Attributes["subscription"] = subscriptionSerialized.Data.Attributes
	}

	return json.Marshal(accountSerialized)
}

// AccountResource is the base data element
type AccountResource struct {
	ID                    string            `json:"id" jsonapi:"primary,accounts"`
	Customers             *CustomerResource `json:"customers" jsonapi:"relation,customers"`
	Name                  string            `json:"name" jsonapi:"attr,name"`
	AccountContactDetails []Contact         `json:"contactDetails" jsonapi:"attr,contactDetails,omitempty"`
}
