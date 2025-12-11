package accounts

import (
	"context"
	"errors"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
	"net/http"
)

// AccountInput represents PUT request body.
// It updates certain fields of account and customer.
// Empty fields overwrite the values since it's a PUT not PATCH!
type AccountInput struct {
	Path struct {
		CustomerID string `uri:"customerID" binding:"required"`
		AccountID  string `uri:"accountID" binding:"required"`
	}
	Body struct {
		Data `json:"data" binding:"required"`
	}
}

type Data struct {
	ID         string     `uri:"id" binding:"required"` // account id
	Type       string     `json:"type" binding:"eq=accounts"`
	Attributes Attributes `json:"attributes" binding:"required"`
}

type Attributes struct {
	Customer              CustomerInput `json:"customer" binding:"required"`
	AccountContactDetails []Contact     `json:"contactDetails"`
}

type Contact struct {
	ID string `jsonapi:"id" json:"id" binding:"required"`
	customer.Contact
}

type CustomerInput struct {
	ID                     string    `json:"id" binding:"required"` // customer ID
	CustomerContactDetails []Contact `json:"contactDetails"`
}

func (i AccountInput) Validate() error {
	// we want to make sure there is exactly one financial contact
	var financialEmail string
	if i.Body.Attributes.Customer.ID == "" {
		return errors.New("customer ID is required")
	}
	if i.Body.Attributes.Customer.ID != i.Path.CustomerID {
		return errors.New("customer IDs in the body and URL are not equal")
	}

	for _, contact := range append(i.Body.Attributes.Customer.CustomerContactDetails, i.Body.Attributes.AccountContactDetails...) {
		// if we already found financial contact, and then we got another one, return an error.
		// we expect only one financial contact.
		if contact.Type == customer.FinancialContactType && financialEmail != "" {
			log.Error().Str("invalidContact", contact.ID).
				Msg("customer has to have one financial contact")
			return errors.New("customer has to have one financial contact")
		}

		// if it's the only one, save it.
		if contact.Type == customer.FinancialContactType {
			financialEmail = contact.Email
		}
	}

	return nil
}

// PUT handles PUT requests on the endpoint.
func (h *Handler) PUT(ctx *goskell.Context) {
	// Parse PUT request.
	var request AccountInput
	if err := ctx.BindUri(&request.Path); err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}
	if err := ctx.ShouldBindJSON(&request.Body); err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	if err := request.Validate(); err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	if !h.isAuthorized(ctx, &Customer{
		ID: request.Path.CustomerID,
		Account: Account{
			ID: request.Path.AccountID,
		},
	}) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// 1. update solvimon
	_, span := goot.Span(ctx.Request.Context(), "update_solvimon")
	oldFinancialEmail, err := h.updateSolvimon(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("failed to update email in solvimon")
		goot.EndSpanWithError(span, err, "failed to update email in solvimon")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// 2. update keycloak
	_, span = goot.Span(ctx.Request.Context(), "update_keycloak")
	err = h.customerService.UpdateAccount(ctx, customer.CustomerUpdate{
		ID:       request.Path.CustomerID,
		Contacts: contactsToMap(request.Body.Attributes.Customer.CustomerContactDetails),
		Account: customer.Account{
			ID: request.Path.AccountID,

			SalesforceContactDetails: map[string]customer.Contact{},
		},
	})
	if err != nil {
		// revert (1) Solvimon first
		_, solvimonErr := h.updateSolvimonEmail(ctx, request.Path.CustomerID, oldFinancialEmail)
		if solvimonErr != nil {
			// if error happened even here, then log it, add to traces
			// but don't return just yet, go back to the original error
			log.Error().Err(solvimonErr).
				Str("oldEmail", oldFinancialEmail).
				Str("customerID", request.Path.CustomerID).
				Msg("failed to rollback email in solvimon after keycloak error")
			span.SetAttributes(
				attribute.String("error_stack_message", "failed to rollback email in solvimon after keycloak error"),
				attribute.String("error_stack", solvimonErr.Error()),
			)
		}

		// now process original error
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to update email in keycloak")
		goot.EndSpanWithError(span, err, "failed to retrieve keycloak subgroup")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Now get and parse the group data for the response, the spans are created inside the GetAccountExtended function
	account, err := h.customerService.GetAccountExtended(ctx.Request.Context(), request.Path.CustomerID, request.Path.AccountID)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to convert groups to customer")
		goskell.JsonAPIError(ctx, "updated account successfully but failed to create a response", err, http.StatusInternalServerError)
		return
	}

	response, err := jsonapi.Marshal(account)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to return the account, but failed to marshal account response")
		goskell.JsonAPIError(ctx, "updated account successfully but failed to return the new data", err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}

func contactsToMap(contacts []Contact) map[string]customer.Contact {
	if len(contacts) == 0 {
		return nil
	}

	m := make(map[string]customer.Contact, len(contacts))
	for _, contact := range contacts {
		m[contact.ID] = contact.Contact
	}
	return m
}

func (h *Handler) updateSolvimon(ctx context.Context, request AccountInput) (oldEmail string, err error) {
	var financialEmail string
	for _, contact := range append(request.Body.Attributes.Customer.CustomerContactDetails, request.Body.Attributes.AccountContactDetails...) {
		if contact.Type == customer.FinancialContactType {
			financialEmail = contact.Email
			break
		}
	}
	if financialEmail == "" {
		return "", nil
	}

	return h.updateSolvimonEmail(ctx, request.Path.CustomerID, financialEmail)
}

func (h *Handler) updateSolvimonEmail(ctx context.Context, customerReference, email string) (oldEmail string, err error) {
	customer, err := h.solvimonClient.Customer.GetByReference(ctx, customerReference)
	if err != nil {
		return "", err
	}

	_, err = h.solvimonClient.Customer.Update(ctx, customer.ID, &solvimon.CustomerUpdateInput{
		Email: email,
	})
	return customer.Email, err
}
