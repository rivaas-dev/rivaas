package customers

import (
	"context"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"go.companyinfo.dev/keycloak"
)

//go:generate mockgen -source=service.go -destination=service_mock.go -package=customers

// ServiceInterface defines the interface for customer service operations
type ServiceInterface interface {
	// Customer Operations
	GetCustomer(ctx context.Context, id string) (*CustomerResource, error)
	GetCustomersPaginated(ctx context.Context, params customer.ListParams) ([]*CustomerResource, error)
	GetCustomersCount(ctx context.Context, params customer.ListParams) (int, error)

	// Account Operations
	GetAccount(ctx context.Context, customerID, accountID string) (*customer.Account, error)
	GetAccountExtended(ctx context.Context, customerID, accountID string) (*ExtendedAccountResource, error)
	ListAccounts(ctx context.Context, customerID string) ([]customer.Account, error)
	ListAccountsPaginated(ctx context.Context, customerID string, first, max int) ([]*AccountResource, error)
	GetAccountCount(ctx context.Context, customerID string) (int, error)
	UpdateAccount(ctx context.Context, customerUpdate customer.CustomerUpdate) error

	// Subscription Operations
	GetSubscription(ctx context.Context, customerID, accountID, pricingPlanID string) (Subscription, error)
	GetCurrentAPIKeyCount(customerID, accountID string) (production int, sandbox int, err error)
	GetPricingPlanQuotaPolicyName(pricingPlanID string) (string, error)
}

// Service provides customer-related operations including subscription management,
// quota retrieval, and customer data processing.
// This is a facade that coordinates the various domain services.
type Service struct {
	customerService     *CustomerService
	subscriptionService *SubscriptionService
	accountService      *AccountService
}

// New constructs a new Service with all required dependencies.
func New(
	keysRepository db.DatabaseExecer,
	keycloakClient keycloak.Client,
	pricingPlans map[string]PricingPlan,
) *Service {
	// Create domain services
	subscriptionService := NewSubscriptionService(keysRepository, pricingPlans)
	customerServiceClient := customer.NewService(&keycloakClient)
	customerService := NewCustomerService(*customerServiceClient)
	accountService := NewAccountService(*customerServiceClient)

	return &Service{
		customerService:     customerService,
		subscriptionService: subscriptionService,
		accountService:      accountService,
	}
}

// Customer Operations

// GetCustomer retrieves a customer by ID
func (s *Service) GetCustomer(ctx context.Context, id string) (*CustomerResource, error) {
	return s.customerService.GetCustomer(ctx, id)
}

// GetCustomersPaginated retrieves multiple customers from Keycloak groups
func (s *Service) GetCustomersPaginated(ctx context.Context, params customer.ListParams) ([]*CustomerResource, error) {
	return s.customerService.GetCustomersPaginated(ctx, params)
}

func (s *Service) GetCustomersCount(ctx context.Context, params customer.ListParams) (int, error) {
	return s.customerService.GetCustomersCount(ctx, params)
}

// Account Operations

// GetAccount retrieves an account by customer ID and account ID
func (s *Service) GetAccount(ctx context.Context, customerID, accountID string) (*customer.Account, error) {
	return s.accountService.GetAccount(ctx, customerID, accountID)
}

// GetAccountExtended retrieves an account with subscription information
func (s *Service) GetAccountExtended(ctx context.Context, customerID, accountID string) (*ExtendedAccountResource, error) {
	return s.accountService.GetAccountExtended(ctx, customerID, accountID, s.subscriptionService)
}

// ListAccounts retrieves all accounts for a customer
func (s *Service) ListAccounts(ctx context.Context, customerID string) ([]customer.Account, error) {
	return s.accountService.ListAccounts(ctx, customerID)
}

func (s *Service) ListAccountsPaginated(ctx context.Context, customerID string, first, max int) ([]*AccountResource, error) {
	return s.accountService.ListAccountsPaginated(ctx, customerID, first, max)
}

func (s *Service) GetAccountCount(ctx context.Context, customerID string) (int, error) {
	return s.accountService.GetAccountCount(ctx, customerID)
}

// UpdateAccount updates a customer account with the provided information
func (s *Service) UpdateAccount(ctx context.Context, customerUpdate customer.CustomerUpdate) error {
	return s.accountService.UpdateAccount(ctx, customerUpdate)
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
