package customers

import (
	"context"
	"fmt"

	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// KeycloakAdapter implements CustomerProvider using Keycloak as the data source
type KeycloakAdapter struct {
	client keycloak.Client
}

// NewKeycloakAdapter creates a new Keycloak adapter
func NewKeycloakAdapter(client keycloak.Client) *KeycloakAdapter {
	return &KeycloakAdapter{
		client: client,
	}
}

// GetCustomer retrieves customer data from Keycloak
func (a *KeycloakAdapter) GetCustomer(ctx context.Context, id string) (*Customer, error) {
	if id == "" {
		return nil, ErrInvalidCustomerID
	}

	group, err := a.client.GetGroupByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer from keycloak: %w", err)
	}

	if group == nil {
		return nil, ErrCustomerNotFound
	}

	return &Customer{
		ID:           *group.ID,
		Name:         *group.Name,
		SalesforceID: a.extractSalesforceID(*group),
		Contacts:     a.extractContacts(*group),
	}, nil
}

// GetAccount retrieves account data from Keycloak
func (a *KeycloakAdapter) GetAccount(ctx context.Context, customerID, accountID string) (*Account, error) {
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}
	if accountID == "" {
		return nil, ErrInvalidAccountID
	}

	group, err := a.client.GetGroupByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer group: %w", err)
	}

	subgroup, err := a.client.GetSubGroupByID(*group, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account subgroup: %w", err)
	}

	if !a.isValidGroup(group) || !a.isValidGroup(subgroup) || !a.isAPIAccount(*subgroup) {
		return nil, ErrInvalidAccount
	}

	return &Account{
		ID:                   *subgroup.ID,
		Name:                 *subgroup.Name,
		CustomerID:           *group.ID,
		CustomerName:         *group.Name,
		CustomerSalesforceID: a.extractSalesforceID(*group),
		Contacts:             a.extractContacts(*subgroup),
		PricingPlanIDs:       a.extractPricingPlanIDs(*subgroup),
	}, nil
}

// ListCustomerAccounts retrieves all accounts for a customer from Keycloak
func (a *KeycloakAdapter) ListCustomerAccounts(ctx context.Context, customerID string) ([]*Account, error) {
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}

	group, err := a.client.GetGroupByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer group: %w", err)
	}

	if !a.isValidGroup(group) {
		return nil, ErrInvalidCustomer
	}

	subGroups := group.SubGroups
	if subGroups == nil {
		return []*Account{}, nil
	}

	var accounts []*Account
	for _, subGroup := range *subGroups {
		if a.isValidGroup(subGroup) && a.isAPIAccount(*subGroup) {
			accounts = append(accounts, &Account{
				ID:                   *subGroup.ID,
				Name:                 *subGroup.Name,
				CustomerID:           *group.ID,
				CustomerName:         *group.Name,
				CustomerSalesforceID: a.extractSalesforceID(*group),
				Contacts:             a.extractContacts(*subGroup),
				PricingPlanIDs:       a.extractPricingPlanIDs(*subGroup),
			})
		}
	}

	return accounts, nil
}

// isValidGroup validates that a Keycloak group is properly initialized
func (a *KeycloakAdapter) isValidGroup(group *keycloak.Group) bool {
	return group != nil && group.ID != nil && group.Name != nil && group.Attributes != nil && len(*group.Attributes) != 0
}

// isAPIAccount determines if the keycloak group is an api account
func (a *KeycloakAdapter) isAPIAccount(group keycloak.Group) bool {
	if group.Attributes == nil {
		return false
	}

	attrMap := *group.Attributes
	for key, values := range attrMap {
		if len(values) > 0 && key == CheckTypeAttribute && values[0] == CheckTypeAPIValue {
			return true
		}
	}
	return false
}

// extractSalesforceID extracts the Salesforce ID from Keycloak group attributes
func (a *KeycloakAdapter) extractSalesforceID(group keycloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	return attr.SalesforceID
}

// extractContacts extracts contact details from Keycloak group attributes
func (a *KeycloakAdapter) extractContacts(group keycloak.Group) []ContactData {
	var contacts []ContactData
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

// extractPricingPlanIDs extracts pricing plan IDs from Keycloak group attributes
func (a *KeycloakAdapter) extractPricingPlanIDs(group keycloak.Group) []string {
	if group.Attributes == nil {
		return nil
	}

	attrMap := *group.Attributes
	if pricingPlanIDs, exists := attrMap[keycloak.PricingPlanIDAttributesKey]; exists {
		return pricingPlanIDs
	}
	return nil
}
