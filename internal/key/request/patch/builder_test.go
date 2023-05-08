package patch

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"testing"
	"time"
)

type PatchTestValidInput struct {
	Input         map[string]interface{}
	ValidationObj Typed
}

type PatchTestInvalidInput struct {
	Input map[string]interface{}
	Error error
}

type PatchTestToMap struct {
	Patch         Typed
	ValidationMap map[string]interface{}
}

func Test_BuildTypedPatchFromMap_Valid(t *testing.T) {
	a := assert.New(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, err := date.CreateYmdFromString(rawYmdDate.String())
	a.Nil(err)
	value4 := int64(4)
	valueMinus1 := int64(-1)
	descriptionText := "description"
	inputs := []PatchTestValidInput{
		{
			Input: map[string]interface{}{
				QuotaEndDateKey: ymdDate.String(),
				PoliciesKey:     []interface{}{},
				QuotaKey:        float64(4),
				DescriptionKey:  "description",
			},
			ValidationObj: Typed{
				Policies:           []string{},
				UpdatePolicies:     true,
				QuotaEndDate:       ymdDate,
				UpdateQuotaEndDate: true,
				Quota:              &value4,
				UpdateQuota:        true,
				Description:        &descriptionText,
				UpdateDescription:  true,
			},
		},
		{
			Input: map[string]interface{}{
				QuotaEndDateKey: nil,
				PoliciesKey:     []interface{}{"yes", "hello"},
				QuotaKey:        float64(-1),
				DescriptionKey:  nil,
			},
			ValidationObj: Typed{
				Policies:           []string{"yes", "hello"},
				UpdatePolicies:     true,
				QuotaEndDate:       nil,
				UpdateQuotaEndDate: true,
				Quota:              &valueMinus1,
				UpdateQuota:        true,
				Description:        nil,
				UpdateDescription:  true,
			},
		},
		{
			Input: map[string]interface{}{},
			ValidationObj: Typed{
				UpdatePolicies:     false,
				UpdateQuotaEndDate: false,
				UpdateQuota:        false,
				UpdateDescription:  false,
			},
		},
	}
	for _, input := range inputs {
		p, err := BuildTypedPatchFromMap(input.Input)
		a.Nil(err)
		a.Equal(input.ValidationObj.Policies, p.Policies)
		a.Equal(input.ValidationObj.UpdatePolicies, p.UpdatePolicies)
		a.Equal(input.ValidationObj.UpdateQuota, p.UpdateQuota)
		a.Equal(input.ValidationObj.Quota, p.Quota)
		a.Equal(input.ValidationObj.UpdateDescription, p.UpdateDescription)
		a.Equal(input.ValidationObj.Description, p.Description)
		a.Equal(input.ValidationObj.UpdateQuotaEndDate, p.UpdateQuotaEndDate)
		a.Equal(input.ValidationObj.QuotaEndDate, p.QuotaEndDate)
	}
}

func Test_BuildTypedPatchFromMap_Invalid(t *testing.T) {
	a := assert.New(t)
	inputs := []PatchTestInvalidInput{
		// invalid policy type
		{
			Input: map[string]interface{}{
				PoliciesKey: 4,
			},
			Error: errors.New(InvalidPoliciesError),
		},
		// invalid quota type
		{
			Input: map[string]interface{}{
				QuotaKey: "yaz",
			},
			Error: errors.New(InvalidQuotaError),
		},
		// invalid quota end date
		{
			Input: map[string]interface{}{
				QuotaEndDateKey: "yaz",
			},
			Error: errors.New(InvalidQuotaEndDateError),
		},
		// invalid quota end date
		{
			Input: map[string]interface{}{
				QuotaEndDateKey: 4,
			},
			Error: errors.New(InvalidQuotaEndDateError),
		},
		// invalid description
		{
			Input: map[string]interface{}{
				DescriptionKey: 23,
			},
			Error: errors.New(InvalidDescriptionError),
		},
	}
	for _, input := range inputs {
		p, err := BuildTypedPatchFromMap(input.Input)
		a.Nil(p)
		a.NotNil(err)
		a.Equal(input.Error.Error(), err.Error())
	}
}

func Test_ToDBMap(t *testing.T) {
	a := assert.New(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, err := date.CreateYmdFromString(rawYmdDate.String())
	a.Nil(err)
	var n *string
	l := "bla"
	inputs := []PatchTestToMap{
		{
			Patch: Typed{
				QuotaEndDate:       ymdDate,
				UpdateQuotaEndDate: true,
				Description:        nil,
				UpdateDescription:  true,
			},
			ValidationMap: map[string]interface{}{
				QuotaEndDateKey: ymdDate.String(),
				DescriptionKey:  n,
			},
		},
		{
			Patch: Typed{
				QuotaEndDate:       nil,
				UpdateQuotaEndDate: false,
				Description:        nil,
				UpdateDescription:  false,
			},
			ValidationMap: map[string]interface{}{},
		},
		{
			Patch: Typed{
				QuotaEndDate:       nil,
				UpdateQuotaEndDate: true,
				Description:        &l,
				UpdateDescription:  true,
			},
			ValidationMap: map[string]interface{}{
				QuotaEndDateKey: nil,
				DescriptionKey:  &l,
			},
		},
	}
	for _, input := range inputs {
		a.Equal(input.ValidationMap, input.Patch.ToDBPatchMap())
	}
}
