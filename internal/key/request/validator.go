package request

import (
	"errors"
	"fmt"
	"time"
)

type validationFunc = func(post Post) error

//Validator validates requests
type Validator struct {
	policies            []string
	validationFunctions []validationFunc
}

//NewValidator constructor
func NewValidator(policies []string) *Validator {
	v := &Validator{policies: policies}
	f := []validationFunc{
		v.validatePolicies,
		v.validateActor,
		v.validateExpirationDate,
	}
	v.validationFunctions = f

	return v
}

//ValidatePost validate the post requests by executing all the validation functions
func (v *Validator) ValidatePost(body Post) error {
	for _, f := range v.validationFunctions {
		if err := f(body); err != nil {
			return err
		}
	}

	return nil
}

//validatePolicies check if all the policies in the request are also in the validators list
func (v *Validator) validatePolicies(body Post) error {
	var policyFound bool
	for _, reqPolicy := range body.Policies {
		policyFound = false
		// check if policy is available
		for _, availablePolicy := range v.policies {
			if reqPolicy == availablePolicy {
				policyFound = true
				break
			}
		}
		// exit condition
		if !policyFound {
			return fmt.Errorf("policy `%s` not available", reqPolicy)
		}
	}
	return nil
}

//validateActor empty is not allowed (perhaps we should eventually add more checks here)
func (v *Validator) validateActor(body Post) error {
	if body.ActorID == "" {
		return errors.New("invalid actor ID, empty string not allowed")
	}

	return nil
}

//validateExpirationDate should be in the future
func (v *Validator) validateExpirationDate(body Post) error {
	if body.ExpirationDate == nil {
		return nil
	}

	if body.ExpirationDate.Before(time.Now()) {
		return errors.New("expiration date must be in the future")
	}

	return nil
}
