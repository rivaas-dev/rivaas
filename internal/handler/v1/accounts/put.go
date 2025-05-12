package accounts

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
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
		Customer              CustomerInput `json:"customer" binding:"required"`
		AccountContactDetails []Contact     `json:"contactDetails"`
	}
}

func (i AccountInput) Validate() error {
	// we want to make sure there is exactly one financial contact
	var financialEmail string
	for _, contact := range append(i.Body.Customer.CustomerContactDetails, i.Body.AccountContactDetails...) {
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

	if financialEmail == "" {
		log.Error().Msg("customer has to have exactly one financial contact")
		return errors.New("missing financial contact")
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
	if err := ctx.BindJSON(&request.Body); err != nil {
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

	if err := request.Validate(); err != nil {
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

	oldFinancialEmail, err := h.updateSolvimon(ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("failed to update email in solvimon")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusInternalServerError),
				Status: http.StatusInternalServerError,
				Detail: err.Error(),
			},
		)
		return
	}

	err = keycloak.UpdateCustomerAccount(ctx, h.keycloakClient, keycloak.CustomerUpdate{
		Account: keycloak.Account{
			ID:       request.Path.ID,
			Contacts: contactsToMap(request.Body.AccountContactDetails),
		},
		ID:       request.Body.Customer.ID,
		Contacts: contactsToMap(request.Body.Customer.CustomerContactDetails),
	})
	if err != nil {
		// revert Solvimon first
		_, solvimonErr := h.updateSolvimonEmail(ctx, request.Body.Customer.ID, oldFinancialEmail)
		if solvimonErr != nil {
			log.Error().Err(solvimonErr).
				Str("oldEmail", oldFinancialEmail).
				Str("customerID", request.Body.Customer.ID).
				Msg("failed to rollback email in solvimon after keycloak error")
		}

		log.Error().Err(err).
			Str("customerID", request.Body.Customer.ID).
			Msg("failed to update email in keycloak")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusInternalServerError),
				Status: http.StatusInternalServerError,
				Detail: err.Error(),
			},
		)
		return
	}
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
	for _, contact := range append(request.Body.Customer.CustomerContactDetails, request.Body.AccountContactDetails...) {
		if contact.Type == keycloak.FinancialContactType {
			financialEmail = contact.Email
			break
		}
	}
	if financialEmail == "" {
		return "", nil
	}

	return h.updateSolvimonEmail(ctx, request.Body.Customer.ID, financialEmail)
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
