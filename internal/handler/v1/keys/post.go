// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/companyinfo/gourn"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/online"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	postWorkerTaskQueue = "apikey"
	postWorkflowName    = "create-apikey"
)

// PostInput represents POST request body.
type PostInput struct {
	ActorID     string             `json:"actor_id"`                                // The reference to the actor. It binds an API key to a client/user in the legacy format.
	CreatorID   string             `header:"X-Customer-ID"`                         // The reference to the creator. It binds an API key to a customer/user in a URN format.
	CustomerID  string             `json:"customer_id"`                             // The reference to the actor customer ID
	AccountID   string             `json:"account_id"`                              // The reference to the actor account ID
	Policies    []string           `json:"policies"             binding:"required"` // The access policies to give, leave empty for none.
	ExpiresAt   *date.Date         `json:"expires_at"`                              // Date on which the key quota will expire at 00.00 (optional).
	Quota       int64              `json:"quota"                binding:"min=-1"`   // The amount of calls the API Key can make (optional).
	Description string             `json:"description"`                             // Description for the key (optional).
	Contact     *Contact           `json:"contacts,omitempty"`                      // Contacts information.
	Active      *bool              `json:"active"`                                  // Defines the status of the key.
	RateLimit   *RateLimit         `json:"rate_limit,omitempty"`                    // Defines rate limit of the key.
	Environment ApikeyEnvironment  `json:"environment"`                             // Defines if a key is for prod or sandbox environment. // Defines if a key is for prod or sandbox environment.'
	Labels      *map[string]string `json:"labels,omitempty"`                        // Contains user specified labels for categorization
}

// Validate validates POST request body.
func (i *PostInput) Validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	// Validate policies.
	if !validation.ValidatePolicyIDs(ctx, tykAPI, i.Policies) {
		return errors.New("invalid policy")
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
	// if not specified in the request, set prod as default environment
	if i.Environment == "" {
		i.Environment = ProdEnv
	}
	// validates if an environment is set to a valid environment
	if !validation.ValidateEnvironment(i.Environment) {
		return errors.New("environment should be either 'production' or 'sandbox'")
	}

	// Validate CustomerID
	customerID, err := cigourn.Parse(i.CreatorID)
	if err != nil {
		return fmt.Errorf("invalid X-Customer-ID provided: %w", err)
	}

	if _, ok := customerID.(*online.User); !ok {
		return errors.New("the creator (X-Customer-ID) should be an Online user")
	}

	// validates actorID
	if i.ActorID == "" && (i.CustomerID == "" || i.AccountID == "") {
		return errors.New(`no "actor_id" or "customer_id"/"account_id" is provided in body`)
	}

	if i.ActorID != "" {
		_, err := cigourn.Parse(i.ActorID)
		if err != nil {
			return err
		}
	}

	return nil
}

// WorkflowPostInput represents the workflow's request body for a POST request.
type WorkflowPostInput struct {
	ActorID     *gourn.URN         // The reference to the actor. It binds an API key to a customer/user.
	CreatorID   *gourn.URN         // The reference to the creator. It binds an API key to a customer/user.
	CustomerID  *string            // References the actor customer ID
	AccountID   *string            // References the actor user ID
	Policies    []string           // The access policies to give, leave empty for none.
	ExpiresAt   *date.Date         // Date on which the key quota will expire at 00.00 (optional).
	Quota       int64              // The amount of calls the API Key can make (optional).
	Description string             // Description for the key (optional).
	Contact     Contact            // Contacts information.
	Active      *bool              // Defines the status of the key.
	RateLimit   RateLimit          // Defines rate limit of the key.
	Environment ApikeyEnvironment  // Defines if a key is for prod or sandbox environment.
	Labels      *map[string]string // Contains user specified labels for categorization
	Region      string             // Region identifier (e.g., "de", or empty for default)
}

// PostOutput represents POST response body.
type PostOutput struct {
	ID  string `json:"id"`  // The key identifier. This is a unique identifier to each API key.
	Key string `json:"key"` // The created API key which can be used to access APIs.
}

// WorkflowPostOutput represents the workflow response body of a POST request.
type WorkflowPostOutput struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// POST handles POST requests on the endpoint.
func (h *Handler) POST(ctx *goskell.Context) {
	// Parse request request.
	var request PostInput
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

	if err := ctx.ShouldBindHeader(&request); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
	}

	// Get the region from the header
	region := h.getRegion(ctx)
	tykClient := h.getTykClient(region)

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

	if !h.isAuthorized(ctx, nil) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	workflowRequest, err := h.postRequestToWorkflowInput(&request, region)
	if err != nil {
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
	response, err := h.POSTWorker(ctx, workflowRequest)
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.JSON(http.StatusCreated, h.workflowOutputToPostResponse(response))
}

// POSTWorker calls workflow on the POST request.
func (h *Handler) POSTWorker(ctx *goskell.Context, request WorkflowPostInput) (*WorkflowPostOutput, error) {
	// Submit request to the worker.
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: postWorkerTaskQueue,
	}
	workflowRun, err := h.temporalClient.ExecuteWorkflow(ctx.Request.Context(), workflowOptions, postWorkflowName, request)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("WorkflowID", workflowRun.GetID()).
		Str("RunID", workflowRun.GetRunID()).
		Msg("workflow is started")

	// Get the worker's response.
	var response WorkflowPostOutput
	err = workflowRun.Get(ctx, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// requestToWorkflowInput converts request body into workflow's input.
func (h *Handler) postRequestToWorkflowInput(request *PostInput, region string) (WorkflowPostInput, error) {
	var err error
	wInput := WorkflowPostInput{
		CustomerID:  &request.CustomerID,
		AccountID:   &request.AccountID,
		Policies:    policies.FilterString(request.Policies),
		ExpiresAt:   request.ExpiresAt,
		Description: request.Description,
		Active:      request.Active,
		Environment: request.Environment,
		Labels:      request.Labels,
		Region:      region,
	}

	if request.Contact != nil {
		wInput.Contact = *request.Contact
	}
	if request.RateLimit != nil {
		wInput.RateLimit = *request.RateLimit
	}
	if request.ActorID != "" {
		wInput.ActorID, err = gourn.Parse(request.ActorID)
		if err != nil {
			return wInput, err
		}
	}
	if request.CreatorID != "" {
		wInput.CreatorID, err = gourn.Parse(request.CreatorID)
		if err != nil {
			return wInput, err
		}
	}
	return wInput, nil
}

// workflowOnputToResponse converts workflow response into API response.
func (h *Handler) workflowOutputToPostResponse(workflowOutput *WorkflowPostOutput) *PostOutput {
	return &PostOutput{
		ID:  workflowOutput.ID,
		Key: workflowOutput.Key,
	}
}
