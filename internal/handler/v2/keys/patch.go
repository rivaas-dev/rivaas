// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
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
	Name        *string            `json:"name"`               // name of the API Key
	Description *string            `json:"description"`        // Description for the key (optional).
	Contact     *apikey.Contact    `json:"contacts,omitempty"` // Contacts information.
	Active      *bool              `json:"active,omitempty"`   // Defines the status of the key.
	Labels      *map[string]string `json:"labels"`             // Contains user specified labels for categorization
}

// Validate validates the PATCH request body.
func (i *PatchInput) Validate(ctx *goskell.Context, tykAPI *tyk.APIClient, existingKey *db.Key) error {
	if i.Body.Attributes.Name != nil && len(*i.Body.Attributes.Name) > apikey.NameMaxLength {
		return fmt.Errorf("maximum length is %d, %d given", apikey.NameMaxLength, len(*i.Body.Attributes.Name))
	}

	// Validate contact emails.
	if i.Body.Attributes.Contact != nil && len(i.Body.Attributes.Contact.Emails) > 0 {
		for _, email := range i.Body.Attributes.Contact.Emails {
			if !validation.ValidateEmail(email.Address) {
				return errors.New("one or more contact emails are incorrect")
			}
		}
	}

	return nil
}

// workflowInput represents the workflow request body.
type workflowInput struct {
	ID          string             // Key ID.
	Name        *string            // Key name.
	Description *string            // Description for the key (optional).
	Contact     *apikey.Contact    // Contacts information.
	Active      *bool              // Defines the status of the key.
	Labels      *map[string]string // Contains user specified labels for categorization
}

// workflowOutput represents the workflow response body.
type workflowOutput struct {
	ID             string
	Name           string
	ActorID        string
	CreatorID      string
	Policies       []string
	ExpiresAt      *time.Time
	Quota          int64
	QuotaRemaining int64
	Description    string
	CreatedAt      time.Time
	Contact        apikey.Contact
	Active         bool
	RateLimit      apikey.RateLimit
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

	// Find the key in database.
	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.Path.ID),
	)
	dbKey, err := h.keysRepository.GetKey(request.Path.ID)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	if errors.Is(err, db.ErrKeyNotFound) || dbKey == nil {
		goot.EndSpanWithError(span, errors.New("key not found"), "key not found")
		log.Err(err).Msg("key not found in database")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusNotFound), err, http.StatusNotFound)
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

	if err := request.Validate(ctx, h.tykClient, dbKey); err != nil {
		goskell.JsonAPIError(ctx, "validation error", err, http.StatusBadRequest)
		return
	}

	if !h.isAuthorizedWithBody(ctx, NewKeyActorID(
		dbKey.ActorID,
		dbKey.CreatorID,
	), &Body{
		Active: request.Body.Attributes.Active,
	}) {
		// The appropriate response is already handled in "isAuthorizedWithPatch()"
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
		Name:        request.Body.Attributes.Name,
		Description: request.Body.Attributes.Description,
	}
	if request.Body.Attributes.Active != nil {
		wInput.Active = request.Body.Attributes.Active
	}
	if request.Body.Attributes.Contact != nil {
		wInput.Contact = request.Body.Attributes.Contact
	}
	if request.Body.Attributes.Labels != nil {
		wInput.Labels = request.Body.Attributes.Labels
	}
	return wInput
}

// workflowOutputToResponse converts the workflow response body into the API response.
func (h *Handler) workflowOutputToPATCHResponse(ctx *goskell.Context, dbKey *db.Key, workflowOutput *workflowOutput) *apikey.APIKey {
	var labels map[string]string
	if workflowOutput.Labels != nil {
		labels = *workflowOutput.Labels
	}

	customerName, err := h.getCustomerName(ctx, dbKey.ActorID)
	if err != nil {
		log.Err(err).Msg("failed to get customer name")
	}

	return &apikey.APIKey{
		ID:             workflowOutput.ID,
		Name:           workflowOutput.Name,
		CustomerName:   customerName,
		Hash:           dbKey.Hash,
		CreatedAt:      date.FormatTime(dbKey.CreatedAt),
		DeletedAt:      date.FormatTimeToPtr(dbKey.DeletedAt),
		Environment:    dbKey.Environment,
		ActorID:        workflowOutput.ActorID,
		CreatorID:      workflowOutput.CreatorID,
		Policies:       policies.FilterString(workflowOutput.Policies),
		ExpiresAt:      date.FormatTimeToPtr(workflowOutput.ExpiresAt),
		Quota:          workflowOutput.Quota,
		QuotaRemaining: workflowOutput.QuotaRemaining,
		Description:    workflowOutput.Description,
		Contact:        workflowOutput.Contact,
		Active:         workflowOutput.Active,
		RateLimit:      workflowOutput.RateLimit,
		Labels:         labels,
	}
}
