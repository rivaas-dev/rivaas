package customers

import "context"

// CustomerProvider defines the interface for retrieving customer data
type CustomerProvider interface {
	GetCustomer(ctx context.Context, id string) (*Customer, error)
	GetAccount(ctx context.Context, customerID, accountID string) (*Account, error)
	ListCustomerAccounts(ctx context.Context, customerID string) ([]*Account, error)
}

// Customer represents customer information from external sources
type Customer struct {
	ID           string
	Name         string
	SalesforceID string
	Contacts     []ContactData
}

// Account represents account information from external sources
type Account struct {
	ID                   string
	Name                 string
	CustomerID           string
	CustomerName         string
	CustomerSalesforceID string
	Contacts             []ContactData
	PricingPlanIDs       []string
}

// ContactData represents contact information
type ContactData struct {
	ID    string
	Email string
}
