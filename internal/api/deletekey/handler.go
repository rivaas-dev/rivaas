// Package deletekey defines delete API key handler.
package deletekey

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	workerTaskQueue = "apikey"
	workflowName    = "delete-apikey"
)

// Handler handles keys requests
type Handler struct {
	temporalClient client.Client
	keysRepository db.DatabaseExecer
}

// New constructs a new Handler.
func New(temporalClient client.Client, keysRepository db.DatabaseExecer) *Handler {
	return &Handler{
		temporalClient: temporalClient,
		keysRepository: keysRepository,
	}
}

// Handle handles endpoint requests.
func (h *Handler) Handle(ctx *goskell.Context) {
	// Parse request request.
	var request input
	if err := ctx.ShouldBindUri(&request); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
			},
		)
		return
	}

	// Find the key in database.
	dbKey, err := h.keysRepository.GetKey(request.Hash)
	if err != nil {
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	if dbKey == nil {
		log.Err(err).Msg("key not found in database")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusNotFound})
		return
	}

	// Call the worker.
	err = h.callWorker(ctx, h.requestToWorkflowInput(&request))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.Status(http.StatusNoContent)
}

func (h *Handler) callWorker(ctx *goskell.Context, request workflowInput) error {
	// Submit request to the worker.
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workerTaskQueue,
	}
	workflowRun, err := h.temporalClient.ExecuteWorkflow(ctx, workflowOptions, workflowName, request)
	if err != nil {
		return err
	}
	log.Info().
		Str("WorkflowID", workflowRun.GetID()).
		Str("RunID", workflowRun.GetRunID()).
		Msg("workflow is started")

	// Get the worker's response.
	var response workflowOutput
	err = workflowRun.Get(ctx, &response)
	if err != nil {
		return err
	}

	return nil
}

// requestToWorkflowInput converts request body into workflow's input.
func (h *Handler) requestToWorkflowInput(request *input) workflowInput {
	return workflowInput{
		Hash: request.Hash,
	}
}
