// Package createkey defines create a new API key handler.
package createkey

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	workerTaskQueue = "apikey"
	workflowName    = "create-apikey"
)

// Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	temporalClient client.Client
}

// New constructs a new Handler.
func New(temporalClient client.Client, tykClient *tyk.APIClient) *Handler {
	return &Handler{
		tykClient:      tykClient,
		temporalClient: temporalClient,
	}
}

// Handle handles endpoint requests.
func (h *Handler) Handle(ctx *goskell.Context) {
	// Parse request request.
	var request input
	if err := ctx.ShouldBindJSON(&request); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			},
		)
		return
	}
	if err := request.validate(ctx, h.tykClient); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			},
		)
		return
	}

	// Call the worker.
	response, err := h.callWorker(ctx, h.requestToWorkflowInput(&request))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.JSON(http.StatusCreated, h.workflowOnputToResponse(response))
}

// callWorker calls workflow.
func (h *Handler) callWorker(ctx *goskell.Context, request workflowInput) (*workflowOutput, error) {
	// Submit request to the worker.
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workerTaskQueue,
	}
	workflowRun, err := h.temporalClient.ExecuteWorkflow(ctx, workflowOptions, workflowName, request)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("WorkflowID", workflowRun.GetID()).
		Str("RunID", workflowRun.GetRunID()).
		Msg("workflow is started")

	// Get the worker's response.
	var response workflowOutput
	err = workflowRun.Get(ctx, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// requestToWorkflowInput converts request body into workflow's input.
func (h *Handler) requestToWorkflowInput(request *input) workflowInput {
	wInput := workflowInput{
		ActorID:      request.ActorID,
		Policies:     request.Policies,
		QuotaEndDate: request.QuotaEndDate,
		Quota:        request.Quota,
		Description:  request.Description,
		Active:       request.Active,
	}
	if request.Contact != nil {
		wInput.Contact = *request.Contact
	}
	if request.RateLimit != nil {
		wInput.RateLimit = *request.RateLimit
	}
	return wInput
}

// workflowOnputToResponse converts workflow response into API response.
func (h *Handler) workflowOnputToResponse(workflowOutput *workflowOutput) *output {
	return &output{
		Key:  workflowOutput.Key,
		Hash: workflowOutput.Hash,
	}
}
