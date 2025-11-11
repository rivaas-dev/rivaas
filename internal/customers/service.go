package customers

import (
	"context"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// Service provides customer-related operations including subscription management,
// quota retrieval, and customer data processing.
// This is a facade that coordinates the various domain services.
type Service struct {
	customerService     *CustomerService
	accountService      *AccountService
	subscriptionService *SubscriptionService
}

// New constructs a new Service with all required dependencies.
func New(
	keysRepository db.DatabaseExecer,
	keycloakClient keycloak.Client,
	pricingPlans map[string]PricingPlan,
) *Service {
	// Create the adapter
	keycloakAdapter := NewKeycloakAdapter(keycloakClient)

	// Create domain services
	subscriptionService := NewSubscriptionService(keysRepository, pricingPlans)
	customerService := NewCustomerService(keycloakAdapter)
	accountService := NewAccountService(keycloakAdapter, subscriptionService)

	return &Service{
		customerService:     customerService,
		accountService:      accountService,
		subscriptionService: subscriptionService,
	}
}

// Customer Operations

// GetCustomer retrieves a customer by ID
func (s *Service) GetCustomer(ctx context.Context, id string) (*CustomerResource, error) {
	return s.customerService.GetCustomer(ctx, id)
}

// ListCustomers retrieves multiple customers from Keycloak groups
func (s *Service) ListCustomersFromGroups(ctx context.Context, groups []*keycloak.Group) ([]*CustomerResource, error) {
	return s.customerService.ListCustomersFromGroups(groups)
}

// Account Operations

// GetAccount retrieves an account by customer ID and account ID
func (s *Service) GetAccount(ctx context.Context, customerID, accountID string) (*AccountResource, error) {
	return s.accountService.GetAccount(ctx, customerID, accountID)
}

// GetAccountExtended retrieves an account with subscription information
func (s *Service) GetAccountExtended(ctx context.Context, customerID, accountID string) (*AccountExtended, error) {
	return s.accountService.GetAccountExtended(ctx, customerID, accountID)
}

// ListAccounts retrieves all accounts for a customer
func (s *Service) ListAccounts(ctx context.Context, customerID string) ([]*AccountResource, error) {
	return s.accountService.ListAccounts(ctx, customerID)
}

// ListAccountsFromGroups converts Keycloak groups to accounts (for handlers that fetch groups directly)
func (s *Service) ListAccountsFromGroups(ctx context.Context, customerGroup *keycloak.Group, accountGroups []*keycloak.Group) ([]*AccountResource, error) {
	if customerGroup == nil || customerGroup.ID == nil {
		return nil, ErrInvalidCustomer
	}

	var accounts []*AccountResource
	for _, accountGroup := range accountGroups {
		if accountGroup == nil {
			continue
		}

		// Use the adapter to convert to Account and then to AccountResource
		accountData := &Account{
			ID:                   *accountGroup.ID,
			Name:                 *accountGroup.Name,
			CustomerID:           *customerGroup.ID,
			CustomerName:         *customerGroup.Name,
			CustomerSalesforceID: s.extractSalesforceIDFromGroup(*customerGroup),
			Contacts:             s.extractContactsFromGroup(*accountGroup),
			PricingPlanIDs:       s.extractPricingPlanIDsFromGroup(*accountGroup),
		}

		accounts = append(accounts, s.accountService.accountToResource(accountData))
	}

	return accounts, nil
}

// Subscription Operations

// GetSubscription retrieves subscription information for a customer account
func (s *Service) GetSubscription(ctx context.Context, customerID, accountID, pricingPlanID string) (Subscription, error) {
	return s.subscriptionService.GetSubscription(ctx, customerID, accountID, pricingPlanID)
}

// GetCurrentAPIKeyCount retrieves the current count of active API keys for a customer
func (s *Service) GetCurrentAPIKeyCount(customerID, accountID string) (production int, sandbox int, err error) {
	return s.subscriptionService.GetCurrentAPIKeyCount(customerID, accountID)
}

// GetPricingPlanQuotaPolicyName retrieves the quota policy name from a pricing plan
func (s *Service) GetPricingPlanQuotaPolicyName(pricingPlanID string) (string, error) {
	return s.subscriptionService.GetPricingPlanQuotaPolicyName(pricingPlanID)
}

// extractSalesforceIDFromGroup extracts the Salesforce ID from a Keycloak group
func (s *Service) extractSalesforceIDFromGroup(group keycloak.Group) string {
	if group.Attributes == nil {
		return ""
	}
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	return attr.SalesforceID
}

// extractContactsFromGroup extracts the contacts from a Keycloak group
func (s *Service) extractContactsFromGroup(group keycloak.Group) []ContactData {
	var contacts []ContactData
	if group.Attributes == nil {
		return contacts
	}

	customer, err := keycloak.ToSubGroupAttributes(*group.Attributes)
	if err != nil {
		return contacts
	}

	for contactID, contact := range customer.ContactDetails {
		contacts = append(contacts, ContactData{
			ID:    contactID,
			Email: contact.Email,
		})
	}
	return contacts
}

// extractPricingPlanIDsFromGroup extracts the pricing plan IDs from a Keycloak group
func (s *Service) extractPricingPlanIDsFromGroup(group keycloak.Group) []string {
	if group.Attributes == nil {
		return nil
	}

	attrMap := *group.Attributes
	if pricingPlanIDs, exists := attrMap[keycloak.PricingPlanIDAttributesKey]; exists {
		return pricingPlanIDs
	}
	return nil
}
