package patch

import (
	"context"
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request/validation"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/policy/api"
)

type validationFunc = func(ctx context.Context, patch *Typed) error

// Validator validates patch requests
type Validator struct {
	policyClient        api.ClientInterface
	validationFunctions []validationFunc
	validParameters     []string
}

// NewValidator constructor
func NewValidator(clientInterface api.ClientInterface) *Validator {
	v := &Validator{policyClient: clientInterface}
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

// ValidatePatch validate the patch requests by executing all the validation functions
func (v *Validator) ValidatePatch(ctx context.Context, rawInput map[string]interface{}, patch *Typed) error {
	// ensure there are no non-existing parameters
	if err := v.validateParameters(ctx, rawInput); err != nil {
		return err
	}
	for _, f := range v.validationFunctions {
		if err := f(ctx, patch); err != nil {
			return err
		}
	}

	return nil
}

// validateParameters check if there are non-existent parameters in the map
func (v *Validator) validateParameters(ctx context.Context, input map[string]interface{}) error {
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

// validatePolicies check if all the policies in the request are also in the validators list
func (v *Validator) validatePolicies(ctx context.Context, patch *Typed) error {
	if !patch.UpdatePolicies {
		return nil
	}

	policies, err := api.ListPolicies(ctx, v.policyClient)
	if err != nil {
		return errors.New("failed to retrieve policies to validate the request")
	}

	return validation.ValidatePolicies(policies, patch.Policies)
}

// validateQuotaEndDate should be in the future
func (v *Validator) validateQuotaEndDate(ctx context.Context, patch *Typed) error {
	return validation.ValidateQuotaEndDate(patch.QuotaEndDate)
}
