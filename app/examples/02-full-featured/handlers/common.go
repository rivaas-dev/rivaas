// Package handlers provides request validation and utility functions for the full-featured example.
package handlers

import (
	"strings"
	"time"
)

// Validate validates the CreateUserRequest.
func (r *CreateUserRequest) Validate() error {
	if r.Name == "" {
		return WrapError(ErrValidationFailed, "name is required")
	}
	if len(r.Name) < 2 || len(r.Name) > 100 {
		return WrapError(ErrValidationFailed, "name must be between 2 and 100 characters")
	}
	if r.Email == "" {
		return WrapError(ErrValidationFailed, "email is required")
	}
	if !IsValidEmail(r.Email) {
		return WrapError(ErrValidationFailed, "invalid email format")
	}
	if r.Age < 0 || r.Age > 150 {
		return WrapError(ErrValidationFailed, "age must be between 0 and 150")
	}
	return nil
}

// Validate validates the CreateOrderRequest.
func (r *CreateOrderRequest) Validate() error {
	if r.UserID <= 0 {
		return WrapError(ErrValidationFailed, "user_id must be a positive integer")
	}
	if len(r.Items) == 0 {
		return WrapError(ErrValidationFailed, "at least one item is required")
	}
	for i, item := range r.Items {
		if item.ProductID <= 0 {
			return WrapError(ErrValidationFailed, "items[%d].product_id must be positive", i)
		}
		if item.Quantity <= 0 {
			return WrapError(ErrValidationFailed, "items[%d].quantity must be positive", i)
		}
		if item.Price < 0 {
			return WrapError(ErrValidationFailed, "items[%d].price must be non-negative", i)
		}
	}
	return nil
}

// GenerateUserID generates a unique user ID.
func GenerateUserID() int {
	return int(time.Now().Unix())
}

// GenerateOrderID generates a unique order ID.
func GenerateOrderID() int {
	return int(time.Now().Unix())
}

// IsValidEmail performs basic email validation.
func IsValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
