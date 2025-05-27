// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	workerTaskQueue = "apikey"
	workflowName    = "update-apikey"
)

// PatchInput represents the PATCH request body.
type PatchInput struct {
	Path struct {
		ID string `uri:"id" binding:"required"` // key ID
	}
	Body struct {
		PatchData `json:"data" binding:"required"`
	}
}

type PatchData struct {
	Type       string          `json:"type" binding:"eq=keys"`
	Attributes PatchAttributes `json:"attributes" binding:"required"`
}

type PatchAttributes struct {
	Policies    *[]string          `json:"policies"`           // The access policies to give, leave empty for none.
	ExpiresAt   *date.Date         `json:"expiresAt"`          // Date on which the key quota will expire at 00.00 (optional).
	Quota       *int64             `json:"quota"`              // The amount of calls the API Key can make (optional).
	Description *string            `json:"description"`        // Description for the key (optional).
	Contact     *Contact           `json:"contacts,omitempty"` // Contacts information.
	Active      *bool              `json:"active,omitempty"`   // Defines the status of the key.
	RateLimit   *RateLimit         `json:"rateLimit"`          // Defines rate limit of the key.
	Labels      *map[string]string `json:"labels"`             // Contains user specified labels for categorization

}

// Validate validates the PATCH request body.
func (i *PatchInput) Validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	// Validate policies.
	if i.Body.Attributes.Policies != nil {
		if !validation.ValidatePolicies(ctx, tykAPI, *i.Body.Attributes.Policies) {
			return errors.New("invalid policy")
		}
	}
	// Validate quota end date.
	if i.Body.Attributes.ExpiresAt != nil {
		if !validation.ValidateEndDate(i.Body.Attributes.ExpiresAt) {
			return errors.New("quota end date must be greater than today")
		}
	}
	// Validate contact emails.
	if i.Body.Attributes.Contact != nil && len(i.Body.Attributes.Contact.Emails) > 0 {
		if !validation.ValidateEmail(i.Body.Attributes.Contact.Emails) {
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
	if err := ctx.ShouldBindUri(&request.Path); err != nil {
		goskell.JsonAPIError(ctx, "URI binding error", err, http.StatusBadRequest)
		return
	}
	if err := ctx.ShouldBindJSON(&request.Body); err != nil {
		goskell.JsonAPIError(ctx, "Body binding error", err, http.StatusBadRequest)
		return
	}
	if err := request.Validate(ctx, h.tykClient); err != nil {
		goskell.JsonAPIError(ctx, "validation error", err, http.StatusBadRequest)
		return
	}

	// Find the key in database.
	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.Path.ID),
	)
	dbKey, err := h.keysRepository.GetKey(request.Path.ID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	if dbKey == nil {
		goot.EndSpanWithError(span, errors.New("key not found"), "key not found")
		log.Err(err).Msg("key not found in database")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusNotFound), err, http.StatusNotFound)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, NewKeyActorID(
		dbKey.ActorID,
		dbKey.CreatorID,
	)) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Call the worker.
	response, err := h.callPATCHWorker(ctx, h.patchRequestToWorkflowInput(&request))
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	jsonAPIResponse, err := jsonapi.Marshal(h.workflowOutputToPATCHResponse(ctx, dbKey, response))
	if err != nil {
		log.Err(err).Msg("error marshalling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
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
func (h *Handler) patchRequestToWorkflowInput(request *PatchInput) workflowInput {
	wInput := workflowInput{
		ID:          request.Path.ID,
		Policies:    request.Body.Attributes.Policies,
		ExpiresAt:   request.Body.Attributes.ExpiresAt,
		Quota:       request.Body.Attributes.Quota,
		Description: request.Body.Attributes.Description,
	}
	if request.Body.Attributes.Active != nil {
		wInput.Active = request.Body.Attributes.Active
	}
	if request.Body.Attributes.Contact != nil {
		wInput.Contact = request.Body.Attributes.Contact
	}
	if request.Body.Attributes.RateLimit != nil {
		wInput.RateLimit = request.Body.Attributes.RateLimit
	}
	if request.Body.Attributes.Labels != nil {
		wInput.Labels = request.Body.Attributes.Labels
	}
	return wInput
}

// workflowOutputToResponse converts the workflow response body into the API response.
func (h *Handler) workflowOutputToPATCHResponse(ctx *goskell.Context, dbKey *db.Key, workflowOutput *workflowOutput) *APIKey {
	var labels map[string]string
	if workflowOutput.Labels != nil {
		labels = *workflowOutput.Labels
	}

	customerName, err := h.getCustomerName(ctx, dbKey)
	if err != nil {
		log.Err(err).Msg("failed to get customer name")
	}

	return &APIKey{
		ID:             workflowOutput.ID,
		CustomerName:   customerName,
		Hash:           dbKey.Hash,
		CreationDate:   dbKey.CreatedAt.Format(time.RFC3339),
		Environment:    dbKey.Environment,
		ActorID:        workflowOutput.ActorID,
		CreatorID:      workflowOutput.CreatorID,
		Policies:       workflowOutput.Policies,
		ExpiresAt:      workflowOutput.ExpiresAt,
		Quota:          workflowOutput.Quota,
		QuotaRemaining: workflowOutput.QuotaRemaining,
		Description:    workflowOutput.Description,
		CreatedDate:    workflowOutput.CreatedAt.Format(time.RFC3339),
		Contact:        workflowOutput.Contact,
		Active:         workflowOutput.Active,
		RateLimit:      workflowOutput.RateLimit,
		Labels:         labels,
	}
}
