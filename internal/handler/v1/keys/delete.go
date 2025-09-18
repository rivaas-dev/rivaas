// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	deleteWorkerTaskQueue = "apikey"
	deleteWorkflowName    = "delete-apikey"
)

// WorkflowDeleteInput represents the workflow input for a DELETE request.
type WorkflowDeleteInput struct {
	ID string
}

// DELETE handles DELETE requests on the endpoint.
func (h *Handler) DELETE(ctx *goskell.Context) {
	// Parse request request.
	var request KeyID
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
	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.ID),
	)
	dbKey, err := h.keysRepository.GetKey(request.ID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	if dbKey == nil {
		goot.EndSpanWithError(span, err, "key not found")
		log.Err(err).Msg("key not found in database")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusNotFound})
		return
	}
	if dbKey.DeletedAt != nil {
		err := errors.New("key was already deleted")
		goot.EndSpanWithError(span, err, "key was already deleted")
		log.Error().Msg("key was already deleted")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, dbKey) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Call the worker.
	err = h.callDeleteWorker(ctx, h.deleteRequestToWorkflowInput(&request))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.Status(http.StatusNoContent)
}

func (h *Handler) callDeleteWorker(ctx *goskell.Context, request WorkflowDeleteInput) error {
	// Submit request to the worker.
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: deleteWorkerTaskQueue,
	}
	workflowRun, err := h.temporalClient.ExecuteWorkflow(ctx.Request.Context(), workflowOptions, deleteWorkflowName, request)
	if err != nil {
		return err
	}
	log.Info().
		Str("WorkflowID", workflowRun.GetID()).
		Str("RunID", workflowRun.GetRunID()).
		Msg("workflow is started")

	// Get the worker's response.
	err = workflowRun.Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

// requestToWorkflowInput converts request body into workflow's input.
func (h *Handler) deleteRequestToWorkflowInput(request *KeyID) WorkflowDeleteInput {
	return WorkflowDeleteInput{
		ID: request.ID,
	}
}
