package post

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

func TestValidator_ValidatePost_Success(t *testing.T) {
	a := assert.New(t)
	nextYear := time.Now().AddDate(1, 0, 0)
	d := date.YmdDate{Time: nextYear}
	body := Post{
		Policies:     []string{"existingPolicy"},
		ActorID:      "1234",
		QuotaEndDate: &d,
	}

	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policies := []string{"existingPolicy", "anotherOne"}
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil).Times(2)
	validator := NewValidator(policyCli)
	err := validator.ValidatePost(context.Background(), body)
	a.Nil(err)
	// also valid
	body.QuotaEndDate = nil
	err = validator.ValidatePost(context.Background(), body)
	a.Nil(err)
}

func TestValidator_ValidatePost_InvalidPolicy(t *testing.T) {
	a := assert.New(t)
	body := Post{
		Policies: []string{"nope"},
		ActorID:  "1234",
	}
	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policies := []string{"existingPolicy", "anotherOne"}
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	validator := NewValidator(policyCli)
	err := validator.ValidatePost(context.Background(), body)
	a.Error(err)
	a.Contains(err.Error(), "policy")
}

func TestValidator_ValidatePost_InvalidDate(t *testing.T) {
	a := assert.New(t)
	d, _ := date.CreateYmdFromString("2017-03-12")
	body := Post{
		Policies:     []string{"existingPolicy"},
		ActorID:      "1234",
		QuotaEndDate: d,
	}
	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policies := []string{"existingPolicy", "anotherOne"}
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	validator := NewValidator(policyCli)
	err := validator.ValidatePost(context.Background(), body)
	a.Error(err)
	a.Contains(err.Error(), "future")
}

func TestValidator_ValidatePost_InvalidActor(t *testing.T) {
	a := assert.New(t)
	body := Post{
		Policies: []string{"existingPolicy"},
		ActorID:  "",
	}
	m := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(m)
	policies := []string{"existingPolicy", "anotherOne"}
	policyCli.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	validator := NewValidator(policyCli)
	err := validator.ValidatePost(context.Background(), body)
	a.Error(err)
	a.Contains(err.Error(), "actor")
}

func createTykPolicies(ids []string) []tyk.Policy {
	var out []tyk.Policy
	for _, id := range ids {
		out = append(out, tyk.Policy{Id: id})
	}
	return out
}
