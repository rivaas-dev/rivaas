package policy

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy/api"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"net/http"
)

// FailedToFetchPoliciesErrorText message is used to indicate that we couldn't retrieve lost of policy from tyk
const FailedToFetchPoliciesErrorText = "failed to fetch a list of available policy"

// HandleGETPolicies return the list of policy (the shortest handler ever)
func (h *Handler) HandleGETPolicies(c *gin.Context) {
	// retrieve the policies list
	policies, err := api.ListPolicies(c, h.policiesClient)
	if err != nil {
		log.WithError(err).Error(FailedToFetchPoliciesErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: FailedToFetchPoliciesErrorText, Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, policies)
}
