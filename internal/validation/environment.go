package validation

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
)

func ValidateEnvironment(env string) bool {
	if env != db.ProdEnv && env != db.SandboxEnv {
		return false
	}
	return true
}
