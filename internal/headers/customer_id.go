package headers

import (
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/online"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

const CustomerIDHeader = "X-Customer-ID"

func GetCustomerID(ctx *goskell.Context) (string, online.User, error) {
	customerID := ctx.GetHeader(CustomerIDHeader)
	user, err := parseCustomerID(customerID)
	return customerID, user, err
}

func ValidateCustomerID(customerID string) error {
	_, err := parseCustomerID(customerID)
	return err
}

func parseCustomerID(customerID string) (online.User, error) {
	customerURN, err := cigourn.Parse(customerID)
	if err != nil {
		return online.User{}, fmt.Errorf("invalid authorization provided: %w", err)
	}

	var ok bool
	customerUser, ok := customerURN.(*online.User)
	if !ok {
		return online.User{}, errors.New("invalid authorization format")
	}

	return *customerUser, nil
}
