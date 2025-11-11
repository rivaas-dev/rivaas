// Package keys defines all methods of the API key.
package keys

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/companyinfo/gourn"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.temporal.io/sdk/client"
)

// Worker addresses.
const (
	postWorkerTaskQueue = "apikey"
	postWorkflowName    = "create-apikey"
)

var (
	// ErrNoValidAPIPlan is returned when an account doesn't have a valid API plan
	ErrNoValidAPIPlan = errors.New("this account is not a valid API plan. Please assign/select an API plan before creating API keys")
)

// PostInput represents POST request body.
type PostInput struct {
	Headers struct {
		CreatorID string `header:"X-Customer-ID"` // The reference to the creator. It binds an API key to a customer/user in a URN format.
	}
	Body PostData `json:"data" binding:"required"`
}

type PostData struct {
	Type       string         `json:"type" binding:"eq=keys"`
	Attributes PostAttributes `json:"attributes" binding:"required"`
}

type PostAttributes struct {
	Name        string                   `json:"name"`               // Name of the key. If empty, filled in with the default text - apikey.DefaultKeyName
	ActorID     string                   `json:"actorID"`            // The reference to the actor. It binds an API key to a client/user in the legacy format.
	CustomerID  string                   `json:"customerID"`         // The reference to the actor customer ID
	AccountID   string                   `json:"accountID"`          // The reference to the actor account ID
	Description string                   `json:"description"`        // Description for the key (optional).
	Contact     *apikey.Contact          `json:"contacts,omitempty"` // Contacts information.
	Active      *bool                    `json:"active"`             // Defines the status of the key.
	Environment apikey.ApikeyEnvironment `json:"environment"`        // Defines if a key is for prod or sandbox environment. // Defines if a key is for prod or sandbox environment.'
	Labels      *map[string]string       `json:"labels,omitempty"`   // Contains user specified labels for categorization

}

// Validate validates POST request body.
func (i *PostInput) Validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	if len(i.Body.Attributes.Name) > apikey.NameMaxLength {
		return fmt.Errorf("maximum length is %d, %d given", apikey.NameMaxLength, len(i.Body.Attributes.Name))
	}

	// if not specified in the request, set prod as default environment
	if i.Body.Attributes.Environment == "" {
		i.Body.Attributes.Environment = apikey.ProdEnv
	}

	// Validate contact emails.
	if i.Body.Attributes.Contact != nil && len(i.Body.Attributes.Contact.Emails) > 0 {
		for _, email := range i.Body.Attributes.Contact.Emails {
			if !validation.ValidateEmail(email.Address) {
				return errors.New("one or more contact emails are incorrect")
			}
		}
	}
	// validates if an environment is set to a valid environment
	if !validation.ValidateEnvironment(i.Body.Attributes.Environment) {
		return errors.New("environment should be either 'production' or 'sandbox'")
	}

	// Validate CustomerID
	err := headers.ValidateCustomerID(i.Headers.CreatorID)
	if err != nil {
		return errors.New("the creator (X-Customer-ID) should be an Online user")
	}

	// validates actorID
	if i.Body.Attributes.ActorID == "" && (i.Body.Attributes.CustomerID == "" || i.Body.Attributes.AccountID == "") {
		return errors.New(`no "actor_id" or "customer_id"/"account_id" is provided in body`)
	}

	if i.Body.Attributes.ActorID != "" {
		_, err := cigourn.Parse(i.Body.Attributes.ActorID)
		if err != nil {
			return err
		}
	}

	return nil
}

// WorkflowPostInput represents the workflow's request body for a POST request.
type WorkflowPostInput struct {
	Name        string                   // API Key name
	ActorID     *gourn.URN               // The reference to the actor. It binds an API key to a customer/user.
	CreatorID   *gourn.URN               // The reference to the creator. It binds an API key to a customer/user.
	CustomerID  *string                  // References the actor customer ID
	AccountID   *string                  // References the actor user ID
	Policies    []string                 // The access policies to give, leave empty for none.
	ExpiresAt   *time.Time               // Date on which the key quota will expire at 00.00 (optional).
	Quota       int64                    // The amount of calls the API Key can make (optional).
	Description string                   // Description for the key (optional).
	Contact     apikey.Contact           // Contacts information.
	Active      *bool                    // Defines the status of the key.
	RateLimit   apikey.RateLimit         // Defines rate limit of the key.
	Environment apikey.ApikeyEnvironment // Defines if a key is for prod or sandbox environment.
	Labels      *map[string]string       // Contains user specified labels for categorization
}

// PostOutput represents POST response body.
type PostOutput struct {
	ID             string                   `jsonapi:"primary,keys"`
	Key            string                   `jsonapi:"attr,key"` // The created API key which can be used to access APIs.
	Name           string                   `jsonapi:"attr,name"`
	CustomerName   string                   `jsonapi:"attr,customerName"`
	Hash           string                   `jsonapi:"attr,hash,omitempty"`
	Environment    apikey.ApikeyEnvironment `jsonapi:"attr,environment"`
	ActorID        string                   `jsonapi:"attr,actorID"`
	CreatorID      string                   `jsonapi:"attr,creatorID"`
	Policies       []string                 `jsonapi:"attr,policies"`
	ExpiresAt      *time.Time               `jsonapi:"attr,expiresAt"`
	CreatedAt      time.Time                `jsonapi:"attr,createdAt,omitempty"`
	Quota          int64                    `jsonapi:"attr,quota"`
	QuotaRemaining int64                    `jsonapi:"attr,quotaRemaining"`
	Description    string                   `jsonapi:"attr,description"`
	Contact        apikey.Contact           `jsonapi:"attr,contacts"`
	Active         bool                     `jsonapi:"attr,active"`
	RateLimit      apikey.RateLimit         `jsonapi:"attr,rateLimit"`
	Labels         map[string]string        `jsonapi:"attr,labels"`
}

// WorkflowPostOutput represents the workflow response body of a POST request.
type WorkflowPostOutput struct {
	ID             string                   `json:"id"`
	Key            string                   `json:"key"`
	Environment    apikey.ApikeyEnvironment `json:"environment"`
	Hash           string                   `json:"hash"`
	Name           string                   `json:"name"`
	ActorID        string                   `json:"actorID"`
	CreatorID      string                   `json:"creatorID"`
	Policies       []string                 `json:"policies"`
	ExpiresAt      *time.Time               `json:"expiresAt"`
	Quota          int64                    `json:"quota"`
	QuotaRemaining int64                    `json:"quotaRemaining"`
	Description    string                   `json:"description"`
	CreationDate   time.Time                `json:"creationDate"`
	Contact        apikey.Contact           `json:"contact"`
	Active         bool                     `json:"active"`
	RateLimit      apikey.RateLimit         `json:"rateLimit"`
	Labels         *map[string]string       `json:"labels"`
}

// POST handles POST requests on the endpoint.
func (h *Handler) POST(ctx *goskell.Context) {
	// Parse request request.
	var request PostInput
	if err := ctx.ShouldBindJSON(&request); err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	if err := ctx.ShouldBindHeader(&request); err != nil {
		goskell.JsonAPIError(ctx, "header validation", err, http.StatusBadRequest)
		return
	}

	if err := request.Validate(ctx, h.tykClient); err != nil {
		goskell.JsonAPIError(ctx, "input validation", err, http.StatusBadRequest)
		return
	}

	accountExt, err := h.customerService.GetAccountExtended(ctx.Request.Context(), request.Body.Attributes.CustomerID, request.Body.Attributes.AccountID)
	if errors.Is(err, customers.ErrInvalidGroup) {
		goskell.JsonAPIError(ctx, "API key creation failed", ErrNoValidAPIPlan, http.StatusUnprocessableEntity)
		return
	}
	if errors.Is(err, customers.ErrPricingPlanNotFound) {
		log.Error().Err(err).
			Str("customerID", request.Body.Attributes.CustomerID).
			Str("accountID", request.Body.Attributes.AccountID).
			Msg("pricing plan not found for account")
		goskell.JsonAPIError(ctx, "API key creation failed",
			errors.New("subscription plan is not properly configured"),
			http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		goskell.JsonAPIError(ctx, "getting an account", err, http.StatusBadRequest)
		return
	}

	key := NewKey(
		request.Body.Attributes.ActorID,
		request.Body.Attributes.CustomerID,
		request.Body.Attributes.AccountID,
		request.Headers.CreatorID,
	)
	if !h.isSubscriptionAuthorized(ctx, key, accountExt.Subscription, request.Body.Attributes.Environment) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	workflowRequest, err := h.postRequestToWorkflowInput(ctx, &request)
	if errors.Is(err, customers.ErrPricingPlanNotFound) {
		log.Error().Err(err).
			Str("customerID", request.Body.Attributes.CustomerID).
			Str("accountID", request.Body.Attributes.AccountID).
			Msg("pricing plan not found for account")
		goskell.JsonAPIError(ctx, "API key creation failed",
			errors.New("subscription plan is not properly configured"),
			http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	// Call the worker.
	response, err := h.POSTWorker(ctx, workflowRequest)
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	jsonAPIResp, err := jsonapi.Marshal(h.workflowOutputToPostResponse(ctx, response))
	if err != nil {
		log.Err(err).Msg("error marshalling json response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	internal.WriteJSONAPIResponse(ctx, jsonAPIResp, http.StatusCreated, nil)
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
func (h *Handler) postRequestToWorkflowInput(ctx context.Context, request *PostInput) (WorkflowPostInput, error) {
	var err error

	// Get pricing plan ID from account extended info
	accountExt, err := h.customerService.GetAccountExtended(ctx, request.Body.Attributes.CustomerID, request.Body.Attributes.AccountID)
	if err != nil {
		return WorkflowPostInput{}, err
	}

	quotaPolicyName, err := h.customerService.GetPricingPlanQuotaPolicyName(accountExt.Subscription.PricingPlanID)
	if errors.Is(err, customers.ErrPricingPlanNotFound) {
		return WorkflowPostInput{}, err
	}
	if err != nil {
		return WorkflowPostInput{}, err
	}

	log.Info().
		Str("pricingPlanID", accountExt.Subscription.PricingPlanID).
		Str("quotaPolicyName", quotaPolicyName).
		Msg("retrieved quota policy name for API key creation")

	policies := h.apiKeyDefaults.Policies
	if quotaPolicyName != "" {
		policies = append(policies, quotaPolicyName)
	} else {
		log.Warn().
			Str("pricingPlanID", accountExt.Subscription.PricingPlanID).
			Msg("quota policy name is empty, not adding to policies")
	}

	wInput := WorkflowPostInput{
		Name:        request.Body.Attributes.Name,
		CustomerID:  &request.Body.Attributes.CustomerID,
		AccountID:   &request.Body.Attributes.AccountID,
		Description: request.Body.Attributes.Description,
		Active:      request.Body.Attributes.Active,
		Environment: request.Body.Attributes.Environment,
		Labels:      request.Body.Attributes.Labels,

		Policies:  policies,
		ExpiresAt: nil,
		RateLimit: apikey.RateLimit{}, // set to empty, no rate limit on key level
	}

	if request.Body.Attributes.Contact != nil {
		wInput.Contact = *request.Body.Attributes.Contact
	}
	if request.Body.Attributes.ActorID != "" {
		wInput.ActorID, err = gourn.Parse(request.Body.Attributes.ActorID)
		if err != nil {
			return wInput, err
		}
	}
	if request.Headers.CreatorID != "" {
		wInput.CreatorID, err = gourn.Parse(request.Headers.CreatorID)
		if err != nil {
			return wInput, err
		}
	}
	return wInput, nil
}

// workflowOutputToResponse converts workflow response into API response.
func (h *Handler) workflowOutputToPostResponse(ctx context.Context, workflowOutput *WorkflowPostOutput) *PostOutput {
	var labels map[string]string
	if workflowOutput.Labels != nil {
		labels = *workflowOutput.Labels
	}

	customerName, err := h.getCustomerName(ctx, workflowOutput.ActorID)
	if err != nil {
		log.Err(err).Msg("failed to get customer name")
	}

	return &PostOutput{
		ID:             workflowOutput.ID,
		Key:            workflowOutput.Key,
		Name:           workflowOutput.Name,
		CustomerName:   customerName,
		Hash:           workflowOutput.Hash,
		Environment:    workflowOutput.Environment,
		ActorID:        workflowOutput.ActorID,
		CreatorID:      workflowOutput.CreatorID,
		Policies:       policies.FilterString(workflowOutput.Policies),
		ExpiresAt:      workflowOutput.ExpiresAt,
		CreatedAt:      workflowOutput.CreationDate,
		Quota:          workflowOutput.Quota,
		QuotaRemaining: workflowOutput.QuotaRemaining,
		Description:    workflowOutput.Description,
		Contact:        workflowOutput.Contact,
		Active:         workflowOutput.Active,
		RateLimit:      workflowOutput.RateLimit,
		Labels:         labels, // todo add more data like in get and patch endpoints
	}
}
