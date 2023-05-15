package patch

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
)

func TestInvalidPolicy(t *testing.T) {
	a := assert.New(t)
	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policies := []string{"test", "test1", "test2"}
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	validator := NewValidator(policyCli)
	err := validator.ValidatePatch(context.Background(), map[string]interface{}{}, &Typed{Policies: []string{"lol no"}, UpdatePolicies: true})
	a.NotNil(err)
	a.Contains(err.Error(), "policy")
}

func TestInvalidInputParam(t *testing.T) {
	a := assert.New(t)

	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	validator := NewValidator(policyCli)
	err := validator.ValidatePatch(context.Background(), map[string]interface{}{"bla": "I dont exist"}, &Typed{})
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

	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies([]string{"test", "test1", "test2"}), nil, nil).Times(2)
	validator := NewValidator(policyCli)

	for _, input := range validPatches {
		err := validator.ValidatePatch(context.Background(), map[string]interface{}{QuotaKey: 4}, &input)
		a.Nil(err)
	}
}

func createTykPolicies(ids []string) []tyk.Policy {
	var out []tyk.Policy
	for _, id := range ids {
		out = append(out, tyk.Policy{Id: id})
	}
	return out
}
