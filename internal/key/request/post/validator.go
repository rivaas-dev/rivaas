package post

import (
	"errors"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request/validation"
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
		v.validateQuotaEndDate,
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
	return validation.ValidatePolicies(v.policies, body.Policies)
}

//validateActor empty is not allowed (perhaps we should eventually add more checks here)
func (v *Validator) validateActor(body Post) error {
	if body.ActorID == "" {
		return errors.New("invalid actor ID, empty string not allowed")
	}

	return nil
}

//validateQuotaEndDate should be in the future
func (v *Validator) validateQuotaEndDate(body Post) error {
	return validation.ValidateQuotaEndDate(body.QuotaEndDate)
}
