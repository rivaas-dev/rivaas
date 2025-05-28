package keycloak

import (
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"sort"
)

// Customer is in relation to an api account
type Customer struct {
	ID                     string    `jsonapi:"primary,customer"`
	Name                   string    `jsonapi:"attr,name"`
	SalesforceID           string    `jsonapi:"attr,salesforceID,omitempty"`
	CustomerContactDetails []Contact `jsonapi:"attr,contactDetails,omitempty"`
}

type Contact struct {
	ID string `jsonapi:"id" json:"id" binding:"required"`
	keycloak.Contact
}

type CustomerSearch struct {
	Name *string
}

func ParseCustomerGroups(groups []*keycloak.Group) []*Customer {
	sort.Slice(groups, func(i, j int) bool {
		return *groups[i].Name < *groups[j].Name
	})

	var customers []*Customer
	// iterate the main groups
	for _, group := range groups {
		// Iterate subgroup
		groupCustomer := ParseCustomerGroup(group)
		customers = append(customers, groupCustomer)
	}

	return customers
}

func ParseCustomerGroup(group *keycloak.Group) *Customer {
	return &Customer{
		ID:                     *group.ID,
		Name:                   *group.Name,
		SalesforceID:           getSalesforceId(*group),
		CustomerContactDetails: getCustomerContactDetails(*group),
	}
}

func getSalesforceId(group keycloak.Group) string {
	attr, err := keycloak.ToGroupAttributes(*group.Attributes)
	if err != nil {
		return ""
	}
	return attr.SalesforceID
}

func getCustomerContactDetails(group keycloak.Group) []Contact {
	var contacts []Contact
	customer, _ := keycloak.ToSubGroupAttributes(*group.Attributes)
	for contactID, contact := range customer.ContactDetails {
		contacts = append(contacts, Contact{
			ID:      contactID,
			Contact: contact,
		})
	}
	return contacts
}
