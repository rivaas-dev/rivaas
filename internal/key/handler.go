package key

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/api"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/request/patch"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/request/post"
	policy "gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy/api"
)

// Handler handles keys requests
type Handler struct {
	keysClient        api.ClientInterface
	keysRepository    RepositoryInterface
	postReqValidator  *post.Validator
	patchReqValidator *patch.Validator
}

// NewHandler constructor
func NewHandler(keyClient api.ClientInterface, policyClient policy.ClientInterface, keysRepository RepositoryInterface) *Handler {
	postValidator := post.NewValidator(policyClient)
	patchValidator := patch.NewValidator(policyClient)

	return &Handler{keysClient: keyClient, keysRepository: keysRepository, postReqValidator: postValidator, patchReqValidator: patchValidator}
}
