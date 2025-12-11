package customers

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
)

const apiType = "api"

var ErrNoValidAPIPlan = errors.New("account is not an API account")

type AccountResource struct {
	ID                    string             `json:"id" jsonapi:"primary,accounts"`
	Customers             *CustomerResource  `json:"customers" jsonapi:"relation,customers"`
	Name                  string             `json:"name" jsonapi:"attr,name"`
	AccountContactDetails []customer.Contact `json:"contactDetails" jsonapi:"attr,contactDetails,omitempty"`
}

// AccountService handles account-related operations
type AccountService struct {
	customerService customer.Service
}

// NewAccountService creates a new AccountService
func NewAccountService(customerService customer.Service) *AccountService {
	return &AccountService{
		customerService: customerService,
	}
}

// GetAccount retrieves an account by customer ID and account ID
func (s *AccountService) GetAccount(ctx context.Context, customerID, accountID string) (*customer.Account, error) {
	customerResult, err := s.customerService.GetCustomerAccount(ctx, customerID, accountID)
	if err != nil {
		return nil, err
	}

	return &customerResult.Accounts[0], nil
}

// ExtendedAccountResource represents an account with subscription information
type ExtendedAccountResource struct {
	ID           string            `json:"id" jsonapi:"primary,accounts"`
	Name         string            `json:"name" jsonapi:"attr,name"`
	Subscription Subscription      `json:"subscription" jsonapi:"attr,subscription"`
	Customers    *CustomerResource `json:"customers" jsonapi:"relation,customers"`
}

// GetAccountExtended retrieves an account with subscription information
func (s *AccountService) GetAccountExtended(ctx context.Context, customerID, accountID string, subscriptionService *SubscriptionService) (*ExtendedAccountResource, error) {
	customer, err := s.customerService.GetCustomerAccount(ctx, customerID, accountID)
	if err != nil {
		return nil, err
	}

	// should only return 1 group which is the api group
	account := customer.Accounts[0]
	if account.Type != apiType {
		return nil, ErrNoValidAPIPlan
	}

	// Get subscription information
	pricingPlanID := "custom" // Default fallback
	if account.PricingPlanID == "" {
		log.Warn().
			Str("customerID", customerID).
			Str("accountID", accountID).
			Msg("no pricing plan found in Keycloak, using 'custom' as fallback")
	} else {
		pricingPlanID = account.PricingPlanID
	}

	subscription, err := subscriptionService.GetSubscription(ctx, customerID, accountID, pricingPlanID)
	if err != nil {
		return nil, err
	}

	// Create a CustomerResource from the customer
	customerResource := &CustomerResource{
		ID:           customer.ID,
		Name:         customer.Name,
		SalesforceID: customer.SalesforceID,
		Contacts:     customer.Contacts,
	}

	// Create an ExtendedAccountResource with the account and subscription information
	return &ExtendedAccountResource{
		ID:           account.ID,
		Name:         account.Name,
		Subscription: subscription,
		Customers:    customerResource,
	}, nil
}

// ListAccounts retrieves all accounts for a customer
func (s *AccountService) ListAccounts(ctx context.Context, customerID string) ([]customer.Account, error) {
	return s.customerService.ListAccounts(ctx, customerID)
}

// ListAccountsPaginated retrieves accounts for a customer with pagination
func (s *AccountService) ListAccountsPaginated(ctx context.Context, customerID string, first, max int) ([]*AccountResource, error) {
	accounts, err := s.customerService.ListAccounts(ctx, customerID)
	if err != nil {
		return nil, err
	}
	customer, err := s.customerService.GetCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}

	// prevents the max to be more than the length of the list
	if len(accounts) < max {
		max = len(accounts)
	}

	var accountResources []*AccountResource
	// for pagination reasons splits the list
	for _, group := range accounts[first:max] {
		account := convertToAccountResource(group, customer)

		accountResources = append(accountResources, account)
	}

	return accountResources, nil
}

// GetAccountCount retrieves the count of accounts for a customer
func (s *AccountService) GetAccountCount(ctx context.Context, customerID string) (int, error) {
	accounts, err := s.customerService.ListAccounts(ctx, customerID)
	if err != nil {
		return 0, err
	}

	return len(accounts), err
}

// UpdateAccount updates a customer account with the provided information
func (s *AccountService) UpdateAccount(ctx context.Context, customerUpdate customer.CustomerUpdate) error {
	// Validate the customer update data
	if err := customerUpdate.Validate(); err != nil {
		return err
	}

	// Call the UpdateCustomerAccount method on the underlying customerService
	return s.customerService.UpdateCustomerAccount(ctx, customerUpdate)
}

func convertToAccountResource(account customer.Account, customerGroup customer.Customer) *AccountResource {
	var contacts []customer.Contact
	for _, contact := range account.SalesforceContactDetails {
		contacts = append(contacts, customer.Contact{
			Email:     contact.Email,
			FirstName: contact.FirstName,
			LastName:  contact.LastName,
			Type:      contact.Type,
		})
	}

	return &AccountResource{
		ID: account.ID,
		Customers: &CustomerResource{
			ID:           customerGroup.ID,
			Name:         customerGroup.Name,
			SalesforceID: customerGroup.SalesforceID,
			Contacts:     customerGroup.Contacts,
		},
		Name:                  account.Name,
		AccountContactDetails: contacts,
	}
}
