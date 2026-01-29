// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"net/http"
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
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

// PatchInput represents the PATCH request body.
type PatchInput struct {
	ID          string             `uri:"id" binding:"required"` // Key ID.
	Policies    *[]string          `json:"policies"`             // The access policies to give, leave empty for none.
	ExpiresAt   *date.Date         `json:"expires_at"`           // Date on which the key quota will expire at 00.00 (optional).
	Quota       *int64             `json:"quota"`                // The amount of calls the API Key can make (optional).
	Description *string            `json:"description"`          // Description for the key (optional).
	Contact     *Contact           `json:"contacts,omitempty"`   // Contacts information.
	Active      *bool              `json:"active,omitempty"`     // Defines the status of the key.
	RateLimit   *RateLimit         `json:"rate_limit"`           // Defines rate limit of the key.
	Labels      *map[string]string `json:"labels"`               // Contains user specified labels for categorization
}

// Validate validates the PATCH request body.
func (i *PatchInput) Validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	// Validate policies.
	if i.Policies != nil {
		if !validation.ValidatePolicyIDs(ctx, tykAPI, *i.Policies) {
			return errors.New("invalid policy")
		}
	}
	// Validate quota end date.
	if i.ExpiresAt != nil {
		if !validation.ValidateEndDate(i.ExpiresAt) {
			return errors.New("quota end date must be greater than today")
		}
	}
	// Validate contact emails.
	if i.Contact != nil && len(i.Contact.Emails) > 0 {
		if !validation.ValidateEmails(i.Contact.Emails) {
			return errors.New("one or more contact emails are incorrect")
		}
	}

	return nil
}

// workflowInput represents the workflow request body.
type workflowInput struct {
	ID          string             // Key ID.
	Policies    *[]string          // The access policies to give, leave empty for none.
	ExpiresAt   *date.Date         // Date on which the key quota will expire at 00.00 (optional).
	Quota       *int64             // The amount of calls the API Key can make (optional).
	Description *string            // Description for the key (optional).
	Contact     *Contact           // Contacts information.
	Active      *bool              // Defines the status of the key.
	RateLimit   *RateLimit         // Defines rate limit of the key.
	Labels      *map[string]string // Contains user specified labels for categorization
	Region      string             // Region identifier (e.g., "de", or empty for default)
}

// output represents response body.
type output struct {
	ID             string             `json:"id"`
	ActorID        string             `json:"actor_id"`
	CreatorID      string             `json:"creator_id"`
	Policies       []string           `json:"policies"`
	ExpiresAt      *date.Date         `json:"expires_at"`
	Quota          int64              `json:"quota"`
	QuotaRemaining int64              `json:"quota_remaining"`
	Description    string             `json:"description"`
	CreatedDate    time.Time          `json:"created_date"`
	Contact        Contact            `json:"contacts"`
	Active         bool               `json:"active"`
	RateLimit      RateLimit          `json:"rate_limit"`
	Labels         *map[string]string `json:"labels"`
}

// workflowOutput represents the workflow response body.
type workflowOutput struct {
	ID             string
	ActorID        string
	CreatorID      string
	Policies       []string
	ExpiresAt      *date.Date
	Quota          int64
	QuotaRemaining int64
	Description    string
	CreatedAt      time.Time
	Contact        Contact
	Active         bool
	RateLimit      RateLimit
	Labels         *map[string]string
}

// PATCH handles PATCH requests of the endpoint.
func (h *Handler) PATCH(ctx *goskell.Context) {
	// Parse request request.
	var request PatchInput
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

	// Get the region from the header
	region := h.getRegion(ctx)
	tykClient := h.getTykClient(region)
	keysRepository := h.getKeysRepository(region)

	if err := request.Validate(ctx, tykClient); err != nil {
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
	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.ID),
	)
	dbKey, err := keysRepository.GetKey(request.ID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	if dbKey == nil {
		goot.EndSpanWithError(span, errors.New("key not found"), "key not found")
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
	response, err := h.callPATCHWorker(ctx, h.patchRequestToWorkflowInput(&request, region))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.JSON(http.StatusOK, h.workflowOutputToPATCHResponse(response))
}

// callWorker calls workflow.
func (h *Handler) callPATCHWorker(ctx *goskell.Context, request workflowInput) (*workflowOutput, error) {
	// Submit request to the worker.
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workerTaskQueue,
	}
	workflowRun, err := h.temporalClient.ExecuteWorkflow(ctx.Request.Context(), workflowOptions, workflowName, request)
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
func (h *Handler) patchRequestToWorkflowInput(request *PatchInput, region string) workflowInput {
	wInput := workflowInput{
		ID:          request.ID,
		Policies:    request.Policies,
		ExpiresAt:   request.ExpiresAt,
		Quota:       request.Quota,
		Description: request.Description,
		Region:      region,
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
	if request.Labels != nil {
		wInput.Labels = request.Labels
	}
	return wInput
}

// workflowOutputToResponse converts the workflow response body into the API response.
func (h *Handler) workflowOutputToPATCHResponse(workflowOutput *workflowOutput) *output {
	return &output{
		ID:             workflowOutput.ID,
		ActorID:        workflowOutput.ActorID,
		CreatorID:      workflowOutput.CreatorID,
		Policies:       policies.FilterString(workflowOutput.Policies),
		ExpiresAt:      workflowOutput.ExpiresAt,
		Quota:          workflowOutput.Quota,
		QuotaRemaining: workflowOutput.QuotaRemaining,
		Description:    workflowOutput.Description,
		CreatedDate:    workflowOutput.CreatedAt,
		Contact:        workflowOutput.Contact,
		Active:         workflowOutput.Active,
		RateLimit:      workflowOutput.RateLimit,
		Labels:         workflowOutput.Labels,
	}
}
