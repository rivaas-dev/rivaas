package customers

import (
	"context"
	"fmt"
	"sort"

	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// CustomerResource is in relation to an api account
type CustomerResource struct {
	ID                     string    `json:"id" jsonapi:"primary,customers"`
	Name                   string    `json:"name" jsonapi:"attr,name"`
	SalesforceID           string    `json:"salesforceID" jsonapi:"attr,salesforceID,omitempty"`
	CustomerContactDetails []Contact `json:"contactDetails" jsonapi:"attr,contactDetails,omitempty"`
}

type Contact struct {
	ID string `jsonapi:"id" json:"id" binding:"required"`
	keycloak.Contact
}

type CustomerSearch struct {
	Name *string
	ID   *string
}

// CustomerService handles customer-related operations
type CustomerService struct {
	dataProvider CustomerProvider
}

// NewCustomerService creates a new CustomerService
func NewCustomerService(dataProvider CustomerProvider) *CustomerService {
	return &CustomerService{
		dataProvider: dataProvider,
	}
}

// GetCustomer retrieves a customer by ID
func (s *CustomerService) GetCustomer(ctx context.Context, id string) (*CustomerResource, error) {
	if id == "" {
		return nil, ErrInvalidCustomerID
	}

	data, err := s.dataProvider.GetCustomer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer data: %w", err)
	}

	return s.customerToResource(data), nil
}

// ListCustomers retrieves multiple customers (keeping existing functionality)
func (s *CustomerService) ListCustomersFromGroups(groups []*keycloak.Group) ([]*CustomerResource, error) {
	if groups == nil {
		return nil, ErrInvalidInput
	}

	// Sort groups by name
	sort.Slice(groups, func(i, j int) bool {
		if groups[i] == nil || groups[i].Name == nil {
			return false
		}
		if groups[j] == nil || groups[j].Name == nil {
			return true
		}
		return *groups[i].Name < *groups[j].Name
	})

	var customers []*CustomerResource
	for _, group := range groups {
		if group == nil {
			continue
		}
		customer, err := s.parseCustomerGroup(group)
		if err != nil {
			// Log error but continue processing other customers
			continue
		}
		customers = append(customers, customer)
	}

	return customers, nil
}

// parseCustomerGroup converts a Keycloak group to a CustomerResource (internal method)
func (s *CustomerService) parseCustomerGroup(group *keycloak.Group) (*CustomerResource, error) {
	if group == nil || group.ID == nil || group.Name == nil {
		return nil, ErrInvalidInput
	}

	return &CustomerResource{
		ID:                     *group.ID,
		Name:                   *group.Name,
		SalesforceID:           s.extractSalesforceID(*group),
		CustomerContactDetails: s.extractCustomerContacts(*group),
	}, nil
}

// customerToResource converts Customer to CustomerResource
func (s *CustomerService) customerToResource(data *Customer) *CustomerResource {
	var contacts []Contact
	for _, contactData := range data.Contacts {
		contacts = append(contacts, Contact{
			ID: contactData.ID,
			Contact: keycloak.Contact{
				Email: contactData.Email,
			},
		})
	}

	return &CustomerResource{
		ID:                     data.ID,
		Name:                   data.Name,
		SalesforceID:           data.SalesforceID,
		CustomerContactDetails: contacts,
	}
}

// extractSalesforceID extracts Salesforce ID from Keycloak group
func (s *CustomerService) extractSalesforceID(group keycloak.Group) string {
	if group.Attributes == nil {
		return ""
	}

	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	
	return attr.SalesforceID
}

// extractCustomerContacts returns contact details from a Keycloak group.
func (s *CustomerService) extractCustomerContacts(group keycloak.Group) []Contact {
	if group.Attributes == nil {
		return nil
	}
	attrs, err := keycloak.ToSubGroupAttributes(*group.Attributes)
	if err != nil {
		return nil
	}
	contacts := make([]Contact, 0, len(attrs.ContactDetails))
	for id, contact := range attrs.ContactDetails {
		contacts = append(contacts, Contact{
			ID:      id,
			Contact: contact,
		})
	}
	return contacts
}
