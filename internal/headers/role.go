package headers

import (
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"slices"
	"strings"
)

const (
	CIRoleHeader        = "CI-Role"
	CIRoleAdministrator = "administrator"
	CIRoleEmpty         = ""
	roleSeparator       = ","
)

func GetRoles(ctx *goskell.Context) []string {
	rolesHeader := ctx.GetHeader(CIRoleHeader)
	return strings.Split(rolesHeader, roleSeparator)
}

func IsAdministrator(roles []string) bool {
	return slices.Contains(roles, CIRoleAdministrator)
}
