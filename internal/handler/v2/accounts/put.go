package accounts

import (
	"context"
	"errors"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"net/http"
)

// AccountInput represents PUT request body.
// It updates certain fields of account and customer.
// Empty fields overwrite the values since it's a PUT not PATCH!
type AccountInput struct {
	Path struct {
		ID string `uri:"id" binding:"required"` // account ID
	}
	Body struct {
		Data `json:"data" binding:"required"`
	}
}

type Data struct {
	Type       string     `json:"type" binding:"eq=accounts"`
	Attributes Attributes `json:"attributes" binding:"required"`
}

type Attributes struct {
	Customer              CustomerInput `json:"customer" binding:"required"`
	AccountContactDetails []Contact     `json:"contactDetails"`
}

func (i AccountInput) Validate() error {
	// we want to make sure there is exactly one financial contact
	var financialEmail string
	for _, contact := range append(i.Body.Attributes.Customer.CustomerContactDetails, i.Body.Attributes.AccountContactDetails...) {
		// if we already found financial contact, and then we got another one, return an error.
		// we expect only one financial contact.
		if contact.Type == keycloak.FinancialContactType && financialEmail != "" {
			log.Error().Str("invalidContact", contact.ID).
				Msg("customer has to have one financial contact")
			return errors.New("customer has to have one financial contact")
		}

		// if it's the only one, save it.
		if contact.Type == keycloak.FinancialContactType {
			financialEmail = contact.Email
		}
	}

	return nil
}

type CustomerInput struct {
	ID                     string    `json:"id" binding:"required"` // customer ID
	CustomerContactDetails []Contact `json:"contactDetails"`
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

	oldFinancialEmail, err := h.updateSolvimon(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("failed to update email in solvimon")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	err = keycloak.UpdateCustomerAccount(ctx, h.keycloakClient, keycloak.CustomerUpdate{
		Account: keycloak.Account{
			ID:       request.Path.ID,
			Contacts: contactsToMap(request.Body.Attributes.AccountContactDetails),
		},
		ID:       request.Body.Attributes.Customer.ID,
		Contacts: contactsToMap(request.Body.Attributes.Customer.CustomerContactDetails),
	})
	if err != nil {
		// revert Solvimon first
		_, solvimonErr := h.updateSolvimonEmail(ctx, request.Body.Attributes.Customer.ID, oldFinancialEmail)
		if solvimonErr != nil {
			log.Error().Err(solvimonErr).
				Str("oldEmail", oldFinancialEmail).
				Str("customerID", request.Body.Attributes.Customer.ID).
				Msg("failed to rollback email in solvimon after keycloak error")
		}

		log.Error().Err(err).
			Str("customerID", request.Body.Attributes.Customer.ID).
			Str("accountID", request.Path.ID).
			Msg("failed to update email in keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	group, err := h.keycloakClient.GetGroupByID(ctx, request.Body.Attributes.Customer.ID)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Body.Attributes.Customer.ID).
			Str("accountID", request.Path.ID).
			Msg("failed to retrieve keycloak group, but update was successful")
		goskell.JsonAPIError(ctx, "updated account successfully but failed to return the new data", err, http.StatusInternalServerError)
		return
	}

	// Parse the group data
	parsedKeyCloakGroup := parseGroup(group)

	// Build response
	response, err := jsonapi.Marshal(parsedKeyCloakGroup)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Body.Attributes.Customer.ID).
			Str("accountID", request.Path.ID).
			Msg("failed to return keycloak group, but update was successful")
		goskell.JsonAPIError(ctx, "updated account successfully but failed to return the new data", err, http.StatusInternalServerError)
		return
	}
	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}

func contactsToMap(contacts []Contact) map[string]keycloak.Contact {
	if len(contacts) == 0 {
		return nil
	}

	m := make(map[string]keycloak.Contact, len(contacts))
	for _, contact := range contacts {
		m[contact.ID] = contact.Contact
	}
	return m
}

func (h *Handler) updateSolvimon(ctx context.Context, request AccountInput) (oldEmail string, err error) {
	var financialEmail string
	for _, contact := range append(request.Body.Attributes.Customer.CustomerContactDetails, request.Body.Attributes.AccountContactDetails...) {
		if contact.Type == keycloak.FinancialContactType {
			financialEmail = contact.Email
			break
		}
	}
	if financialEmail == "" {
		return "", nil
	}

	return h.updateSolvimonEmail(ctx, request.Body.Attributes.Customer.ID, financialEmail)
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
