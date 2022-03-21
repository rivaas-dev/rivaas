package patch

import (
	"fmt"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request/validation"
)

type validationFunc = func(patch *Typed) error

//Validator validates patch requests
type Validator struct {
	policies            []string
	validationFunctions []validationFunc
	validParameters     []string
}

//NewValidator constructor
func NewValidator(policies []string) *Validator {
	v := &Validator{policies: policies}
	v.validationFunctions = []validationFunc{
		v.validatePolicies,
		v.validateQuotaEndDate,
	}
	v.validParameters = []string{
		QuotaEndDateKey,
		QuotaKey,
		PoliciesKey,
		DescriptionKey,
	}
	return v
}

//ValidatePatch validate the patch requests by executing all the validation functions
func (v *Validator) ValidatePatch(rawInput map[string]interface{}, patch *Typed) error {
	// ensure there are no non-existing parameters
	if err := v.validateParameters(rawInput); err != nil {
		return err
	}
	for _, f := range v.validationFunctions {
		if err := f(patch); err != nil {
			return err
		}
	}

	return nil
}

//validateParameters check if there are non-existent parameters in the map
func (v *Validator) validateParameters(input map[string]interface{}) error {
	for inputParam := range input {
		found := false
		for _, validParam := range v.validParameters {
			if inputParam == validParam {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid request parameter `%s`", inputParam)
		}
	}

	return nil
}

//validatePolicies check if all the policies in the request are also in the validators list
func (v *Validator) validatePolicies(patch *Typed) error {
	if !patch.UpdatePolicies {
		return nil
	}
	return validation.ValidatePolicies(v.policies, patch.Policies)
}

//validateQuotaEndDate should be in the future
func (v *Validator) validateQuotaEndDate(patch *Typed) error {
	return validation.ValidateQuotaEndDate(patch.QuotaEndDate)
}
