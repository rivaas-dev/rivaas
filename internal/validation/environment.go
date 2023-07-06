package validation

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
)

// ValidateEnvironment validates if the environment is set and correct value
func ValidateEnvironment(env string) bool {
	if env != db.ProdEnv && env != db.SandboxEnv {
		return false
	}
	return true
}
