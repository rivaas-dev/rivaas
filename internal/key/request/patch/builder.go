package patch

import (
	"errors"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

const (
	QuotaEndDateKey = "quota_end_date"
	PoliciesKey     = "policies"
	QuotaKey        = "quota"
	DescriptionKey  = "description"

	InvalidQuotaEndDateError = "invalid quota_end_date parameter, should be a date with yyyy-mm-dd format"
	InvalidPoliciesError     = "invalid policy parameter, should be of type []string"
	InvalidQuotaError        = "invalid quota parameter, should be of type int"
	InvalidDescriptionError  = "invalid description parameter"
)

type hydrationFunction = func(input map[string]interface{}) error

//Typed patch object hydrated from map. It holds booleans that indicate if the values/fields need to be updated.
//It's added for typing convenience (one place to do type conversion).
type Typed struct {
	Policies       []string
	UpdatePolicies bool

	QuotaEndDate       *date.YmdDate
	UpdateQuotaEndDate bool

	Quota       *int64
	UpdateQuota bool

	Description       *string
	UpdateDescription bool
}

//BuildTypedPatchFromMap builds/hydrates a typed patch object
func BuildTypedPatchFromMap(input map[string]interface{}) (*Typed, error) {
	p := &Typed{}
	hydrationFunctions := []hydrationFunction{
		p.hydratePoliciesFromMap,
		p.hydrateQuotaEndDateFromMap,
		p.hydrateQuotaFromMap,
		p.hydrateDescriptionFromMap,
	}
	for _, f := range hydrationFunctions {
		if err := f(input); err != nil {
			return nil, err
		}
	}

	return p, nil
}

//ToDBPatchMap based on properties converts it to a map that can be used by the Update method from Gorm
func (p *Typed) ToDBPatchMap() map[string]interface{} {
	patch := map[string]interface{}{}
	// We have to convert the endDate to string for Gorm
	if p.UpdateQuotaEndDate && p.QuotaEndDate != nil {
		patch[QuotaEndDateKey] = p.QuotaEndDate.String()
	} else if p.UpdateQuotaEndDate && p.QuotaEndDate == nil {
		patch[QuotaEndDateKey] = nil
	}
	if p.UpdateDescription {
		patch[DescriptionKey] = p.Description
	}

	return patch
}

func (p *Typed) hydratePoliciesFromMap(input map[string]interface{}) error {
	rawPolicies, exists := input[PoliciesKey]
	if !exists {
		p.UpdatePolicies = false
		return nil
	}
	p.UpdatePolicies = true
	interfacePolicies, ok := rawPolicies.([]interface{})
	if !ok {
		return errors.New(InvalidPoliciesError)
	}
	stringPolicies := []string{}
	for _, val := range interfacePolicies {
		stringPolicies = append(stringPolicies, val.(string))
	}
	p.Policies = stringPolicies

	return nil
}

func (p *Typed) hydrateQuotaEndDateFromMap(input map[string]interface{}) error {
	rawQuotaEndDate, exists := input[QuotaEndDateKey]
	if !exists {
		p.UpdateQuotaEndDate = false
		return nil
	}
	p.UpdateQuotaEndDate = true
	// Early return in case of nil (which is a valid value), otherwise everything dies when casting
	if rawQuotaEndDate == nil {
		p.QuotaEndDate = nil
		return nil
	}
	switch rawQuotaEndDate.(type) {
	case string:
		break
	default:
		return errors.New(InvalidQuotaEndDateError)
	}

	parsedQuotaEndDate, err := date.CreateYmdFromString(rawQuotaEndDate.(string))
	if err != nil {
		return errors.New(InvalidQuotaEndDateError)
	}
	p.QuotaEndDate = parsedQuotaEndDate
	return nil
}

func (p *Typed) hydrateQuotaFromMap(input map[string]interface{}) error {
	rawQuota, exists := input[QuotaKey]
	if !exists {
		p.UpdateQuota = false
		return nil
	}

	parsedQuota, ok := rawQuota.(float64) // float64 is the default type
	if !ok {
		return errors.New(InvalidQuotaError)
	}

	p.UpdateQuota = true
	parsedInt := int64(parsedQuota)
	p.Quota = &parsedInt
	return nil
}

func (p *Typed) hydrateDescriptionFromMap(input map[string]interface{}) error {
	rawDescription, exists := input[DescriptionKey]
	if !exists {
		p.UpdateDescription = false
		return nil
	}
	p.UpdateDescription = true
	if rawDescription == nil {
		p.Description = nil
		return nil
	}

	parsed, ok := rawDescription.(string)
	if !ok {
		return errors.New(InvalidDescriptionError)
	}
	p.Description = &parsed
	return nil
}
