package post

import (
	"context"
	"errors"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request/validation"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/policy/api"
)

type validationFunc = func(ctx context.Context, post Post) error

// Validator validates requests
type Validator struct {
	policyClient        api.ClientInterface
	validationFunctions []validationFunc
}

// NewValidator constructor
func NewValidator(clientInterface api.ClientInterface) *Validator {
	v := &Validator{policyClient: clientInterface}
	f := []validationFunc{
		v.validatePolicies,
		v.validateActor,
		v.validateQuotaEndDate,
	}
	v.validationFunctions = f

	return v
}

// ValidatePost validate the post requests by executing all the validation functions
func (v *Validator) ValidatePost(ctx context.Context, body Post) error {
	for _, f := range v.validationFunctions {
		if err := f(ctx, body); err != nil {
			return err
		}
	}

	return nil
}

// validatePolicies check if all the policies in the request are also in the validators list
func (v *Validator) validatePolicies(ctx context.Context, body Post) error {
	policies, err := api.ListPolicies(ctx, v.policyClient)
	if err != nil {
		return errors.New("failed to retrieve policies to validate the request")
	}

	return validation.ValidatePolicies(policies, body.Policies)
}

// validateActor empty is not allowed (perhaps we should eventually add more checks here)
func (v *Validator) validateActor(ctx context.Context, body Post) error {
	if body.ActorID == "" {
		return errors.New("invalid actor ID, empty string not allowed")
	}

	return nil
}

// validateQuotaEndDate should be in the future
func (v *Validator) validateQuotaEndDate(ctx context.Context, body Post) error {
	return validation.ValidateQuotaEndDate(body.QuotaEndDate)
}
