package request

import (
	"github.com/stretchr/testify/assert"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/date"
	"testing"
	"time"
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
	validator := NewValidator([]string{"existingPolicy", "anotherOne"})
	err := validator.ValidatePost(body)
	a.Nil(err)
	// also valid
	body.QuotaEndDate = nil
	err = validator.ValidatePost(body)
	a.Nil(err)
}

func TestValidator_ValidatePost_InvalidPolicy(t *testing.T) {
	a := assert.New(t)
	body := Post{
		Policies: []string{"nope"},
		ActorID:  "1234",
	}
	validator := NewValidator([]string{"existingPolicy", "anotherOne"})
	err := validator.ValidatePost(body)
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
	validator := NewValidator([]string{"existingPolicy", "anotherOne"})
	err := validator.ValidatePost(body)
	a.Error(err)
	a.Contains(err.Error(), "future")
}

func TestValidator_ValidatePost_InvalidActor(t *testing.T) {
	a := assert.New(t)
	body := Post{
		Policies: []string{"existingPolicy"},
		ActorID:  "",
	}
	validator := NewValidator([]string{"existingPolicy", "anotherOne"})
	err := validator.ValidatePost(body)
	a.Error(err)
	a.Contains(err.Error(), "actor")
}
