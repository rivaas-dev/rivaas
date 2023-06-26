// Package updatekey updates a key.
package updatekey

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	workerTaskQueue = "apikey"
	workflowName    = "update-apikey"
)

// Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	temporalClient client.Client
	keysRepository db.DatabaseExecer
}

// New constructs a new Handler.
func New(temporalClient client.Client, keysRepository db.DatabaseExecer, tykClient *tyk.APIClient) *Handler {
	return &Handler{
		tykClient:      tykClient,
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
				Detail: err.Error(),
			},
		)
		return
	}
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
	response, err := h.callWorker(ctx, h.requestToWorkflowInput(&request))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.JSON(http.StatusCreated, h.workflowOutputToResponse(response))
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
		Hash:         request.Hash,
		Policies:     request.Policies,
		QuotaEndDate: request.QuotaEndDate,
		Quota:        request.Quota,
		Description:  request.Description,
	}
	if request.Active != nil {
		wInput.Active = request.Active
	}
	if request.Contact != nil {
		wInput.Contact = request.Contact
	}
	if request.RateLimit != nil {
		wInput.RateLimit = request.RateLimit
	}
	return wInput
}

// workflowOutputToResponse converts the workflow response body into the API response.
func (h *Handler) workflowOutputToResponse(workflowOutput *workflowOutput) *output {
	return &output{
		ActorID:      workflowOutput.ActorID,
		Policies:     workflowOutput.Policies,
		QuotaEndDate: workflowOutput.QuotaEndDate,
		Quota:        workflowOutput.Quota,
		Description:  workflowOutput.Description,
		CreatedDate:  workflowOutput.CreatedAt,
		Contact:      workflowOutput.Contact,
		Active:       workflowOutput.Active,
		RateLimit:    workflowOutput.RateLimit,
	}
}
