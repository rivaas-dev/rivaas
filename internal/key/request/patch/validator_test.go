package patch

import (
	"github.com/stretchr/testify/assert"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/date"
	"testing"
	"time"
)

func TestInvalidPolicy(t *testing.T) {
	a := assert.New(t)

	validator := NewValidator([]string{"test", "test1", "test2"})
	err := validator.ValidatePatch(map[string]interface{}{}, &Typed{Policies: []string{"lol no"}, UpdatePolicies: true})
	a.NotNil(err)
	a.Contains(err.Error(), "policy")
}

func TestInvalidInputParam(t *testing.T) {
	a := assert.New(t)

	validator := NewValidator([]string{"test", "test1", "test2"})
	err := validator.ValidatePatch(map[string]interface{}{"bla": "I dont exist"}, &Typed{})
	a.NotNil(err)
	a.Contains(err.Error(), "invalid request parameter")
}

func TestValidateValidPatches(t *testing.T) {
	a := assert.New(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, err := date.CreateYmdFromString(rawYmdDate.String())
	a.Nil(err)
	validPatches := []Typed{
		{
			Policies:           nil,
			UpdatePolicies:     false,
			QuotaEndDate:       nil,
			UpdateQuotaEndDate: false,
			Quota:              nil,
			UpdateQuota:        false,
			Description:        nil,
			UpdateDescription:  false,
		},
		{
			Policies:           []string{"test", "test1"},
			UpdatePolicies:     true,
			QuotaEndDate:       nil,
			UpdateQuotaEndDate: true,
		},
		{
			Policies:           []string{"test", "test1"},
			UpdatePolicies:     true,
			QuotaEndDate:       ymdDate,
			UpdateQuotaEndDate: true,
		},
	}

	validator := NewValidator([]string{"test", "test1", "test2"})
	for _, input := range validPatches {
		err := validator.ValidatePatch(map[string]interface{}{QuotaKey: 4}, &input)
		a.Nil(err)
	}
}
