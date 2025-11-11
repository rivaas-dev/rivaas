package customers

import (
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
)

// isGroupValid validates that a Keycloak group is properly initialized and has attributes.
func isGroupValid(group *keycloak.Group) bool {
	return group != nil && group.Attributes != nil && len(*group.Attributes) != 0
}
