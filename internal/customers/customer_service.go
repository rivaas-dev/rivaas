package customers

import (
	"context"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
)

// CustomerResource is a wrapper for customer.Customer that uses pointers for Account structs
// to avoid the "reflect: call of reflect.Value.IsNil on struct Value" error in jsonapi.Marshal
type CustomerResource struct {
	ID           string                      `json:"id" jsonapi:"primary,customers"`
	Name         string                      `json:"name" jsonapi:"attr,name"`
	SalesforceID string                      `json:"salesforceID,omitempty" jsonapi:"attr,salesforceID,omitempty"`
	Contacts     map[string]customer.Contact `json:"contacts,omitempty" jsonapi:"attr,contacts,omitempty"`
}

// CustomerService handles account-related operations
type CustomerService struct {
	customerService customer.Service
}

// NewCustomerService creates a new CustomerService
func NewCustomerService(customerService customer.Service) *CustomerService {
	return &CustomerService{
		customerService: customerService,
	}
}

// GetCustomer retrieves a customer by ID
func (s *CustomerService) GetCustomer(ctx context.Context, id string) (*CustomerResource, error) {
	customer, err := s.customerService.GetCustomer(ctx, id)
	if err != nil {
		return nil, err
	}

	customerResource := convertToCustomerResource(customer)

	return customerResource, nil
}

// GetCustomersPaginated retrieves multiple customers from Keycloak groups
func (s *CustomerService) GetCustomersPaginated(ctx context.Context, params customer.ListParams) ([]*CustomerResource, error) {
	customers, err := s.customerService.ListCustomers(ctx, params)
	if err != nil {
		return nil, err
	}
	var customerResource []*CustomerResource
	for _, group := range customers {
		customer := convertToCustomerResource(group)

		customerResource = append(customerResource, customer)
	}

	return customerResource, nil
}

func (s *CustomerService) GetCustomersCount(ctx context.Context, params customer.ListParams) (int, error) {
	customers, err := s.customerService.ListCustomers(ctx, params)
	if err != nil {
		return 0, err
	}

	return len(customers), nil
}

// convertToCustomerResource converts a slice of customer.Customer to a slice of CustomerResource
// This is needed to avoid the "reflect: call of reflect.Value.IsNil on struct Value" error
// when marshaling the customer data with jsonapi.Marshal
func convertToCustomerResource(customerInput customer.Customer) *CustomerResource {
	customerResources := &CustomerResource{
		ID:           customerInput.ID,
		Name:         customerInput.Name,
		SalesforceID: customerInput.SalesforceID,
		Contacts:     customerInput.Contacts,
	}
	return customerResources
}
