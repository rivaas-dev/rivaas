package headers

import (
	"gitlab.ci.fdmg.org/ci-api/cigourn/online"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type Authorization struct {
	Roles        []string    `header:"CI-Role"`       // CIRoleHeader
	CustomerID   string      `header:"X-Customer-ID"` // CustomerIDHeader
	CustomerUser online.User // a parsed version of the CustomerID
}

func GetAuthorization(ctx *goskell.Context) (authorization Authorization, err error) {
	authorization.Roles = GetRoles(ctx)
	authorization.CustomerID, _, err = GetCustomerID(ctx)
	if err != nil {
		return authorization, err
	}

	authorization.CustomerUser, err = parseCustomerID(authorization.CustomerID)
	if err != nil {
		return authorization, err
	}

	return authorization, nil
}
