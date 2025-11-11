package customers

import "errors"

// Keycloak attribute constants
const (
	CheckTypeAttribute = "type"
	CheckTypeAPIValue  = "api"
)

// Business rule constants
const (
	MaxContactsPerCustomer = 100 // reasonable limit for contacts
)

// Domain errors
var (
	ErrInvalidCustomerID   = errors.New("customer ID cannot be empty")
	ErrInvalidAccountID    = errors.New("account ID cannot be empty")
	ErrCustomerNotFound    = errors.New("customer not found")
	ErrAccountNotFound     = errors.New("account not found")
	ErrInvalidCustomer     = errors.New("invalid customer data")
	ErrInvalidAccount      = errors.New("invalid account data")
	ErrPricingPlanNotFound = errors.New("pricing plan not found")
	ErrInvalidInput        = errors.New("invalid input provided")
)
