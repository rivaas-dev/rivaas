package validation

import (
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"time"
)

//ValidateQuotaEndDate validate if quota end date is in the future
func ValidateQuotaEndDate(quotaEndDate *date.YmdDate) error {
	// nil is a valid date
	if quotaEndDate == nil {
		return nil
	}
	if quotaEndDate.Before(time.Now()) {
		return errors.New("quota end date must be in the future")
	}

	return nil
}

//ValidatePolicies validate if the input policies are in the valid policies list
func ValidatePolicies(validPolicies []string, inputPolicies []string) error {
	var policyFound bool
	for _, reqPolicy := range inputPolicies {
		policyFound = false
		// check if policy is available
		for _, availablePolicy := range validPolicies {
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
