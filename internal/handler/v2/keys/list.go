// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"fmt"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/online"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const urlPrefix = "/keys"

type FilterParam struct {
	Filter map[string]string
	Match  map[string]string
}

var (
	// validFilters valid filter keywords in query string
	validFilters = map[string]struct{}{
		"actorID": {}, // ActorID full urn of the customer. Example: urn:api:key:19fd5d47-91ea-4cac-a8e0-ea9e295fa44b:f82dddf1-4f06-4fa0-80bd-c2014d5f9540:01HX6GV6CV8NPR1XQRRQQZTMXN
	}
	validMatch = map[string]struct{}{
		"description": {}, // Description of the api key. Usually is the customer name and type of key, like Prod key customer X
		"customerID":  {}, // CustomerID is the ID of the overall Customer. Example: 19fd5d47-91ea-4cac-a8e0-ea9e295fa44b
		"accountID":   {}, // AccountID is the ID of the account. An account can be for example API. Example: f82dddf1-4f06-4fa0-80bd-c2014d5f9540
	}
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	CustomerID       string       `header:"X-Customer-ID"` // The reference to the creator. It binds an API key to a customer/user in a URN format.
	ParsedCustomerID *online.User // todo replace with an api user urn
	Filter           map[string]string
	Match            map[string]string
	PaginationParams internal.PaginationParams
}

// ListOutput represents the list of key's information.
type ListOutput struct {
	ID           string            `jsonapi:"primary,keys"`
	CustomerName string            `jsonapi:"attr,customerName"`
	Hash         string            `jsonapi:"attr,hash"`
	CreatorID    string            `jsonapi:"attr,creatorID"`
	ActorID      string            `jsonapi:"attr,actorID"`
	CreationAt   string            `jsonapi:"attr,creationDate"`
	Contact      Contact           `jsonapi:"attr,contacts"`
	Active       bool              `jsonapi:"attr,active"`
	RateLimit    RateLimit         `jsonapi:"attr,rateLimit"`
	ExpiresAt    *date.Date        `jsonapi:"attr,expiresAt"`
	Description  *string           `jsonapi:"attr,description"`
	Environment  ApikeyEnvironment `jsonapi:"attr,environment"`
	Labels       map[string]string `jsonapi:"attr,labels"`
}

// LIST handles GET requests on the endpoint to get list of keys.
func (h *Handler) LIST(ctx *goskell.Context) {
	// Parse request body.
	request, err := bindRequest(ctx, h.defaultPageSize, h.maxPageSize)
	if err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	// Fetch keys from the database.
	_, span := goot.Span(ctx.Request.Context(), "get_from_database")
	keys, totalResults, err := h.getAPIKeys(request)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, nil) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Prepare the response.
	jsonAPIResponse, err := jsonapi.Marshal(h.convertListDBResultToJSON(ctx, keys))
	if err != nil {
		log.Err(err).Msg("failed to marshal JSON API response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	//Adds the pagination links to the end of the response
	if ja, ok := jsonAPIResponse.(*jsonapi.ManyPayload); ok {
		ja.Links = (*jsonapi.Links)(internal.GeneratePageLinks(&internal.PaginationParams{
			Size: request.PaginationParams.Size,
			Page: request.PaginationParams.Page,
		}, uint(totalResults),
			urlPrefix))
		ja.Meta = &jsonapi.Meta{
			"totalResults": totalResults,
			"totalPages":   uint32(math.Ceil(float64(totalResults) / float64(request.PaginationParams.Size))),
		}
	}

	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
}

func (h *Handler) getAPIKeys(request *ListInput) (keys []*db.Key, totalResults int64, err error) {
	// todo add admin user access here
	// if admin {
	//   	searchParams := NewAdminSearchParameters(request.Filter, request.Match)
	// }
	searchParams := NewCustomerSearchParameters(request.Filter, request.Match, request.ParsedCustomerID.CustomerID)
	return h.keysRepository.GetKeysPaginated(searchParams, request.PaginationParams.Size, request.PaginationParams.Page)
}

func (h *Handler) convertListDBResultToJSON(ctx *goskell.Context, keys []*db.Key) []*ListOutput {
	response := make([]*ListOutput, 0)
	for _, key := range keys {
		rl := RateLimit{
			Rate: 0,
			Per:  0,
		}

		if key.Metadata != nil {
			if data, ok := key.Metadata["rate_limit"]; ok {
				castedData := data.(map[string]any)
				rl.Rate = castedData["Rate"].(float64)
				rl.Per = castedData["Per"].(float64)
			}
		}

		// call keycloak to get the customer name, set it as unknown to avoid panic when not found
		keycloakGroups, err := h.keycloakClient.GetGroups(ctx, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
		if err != nil {
			log.Err(err).Msg("cant find client in keycloak")
		}
		// Define the response
		response = append(response, &ListOutput{
			ID:           key.ID,
			CustomerName: internal.GetCustomerName(keycloakGroups, *key),
			CreatorID:    key.CreatorID,
			Hash:         key.Hash,
			ActorID:      key.ActorID,
			ExpiresAt:    key.ExpiresAt,
			Description:  key.Description,
			CreationAt:   key.CreatedAt.Format(time.RFC3339),
			Contact:      Contact(key.Contact),
			Active:       key.Active,
			RateLimit:    rl,
			Environment:  key.Environment,
			Labels:       key.Labels,
		})
	}
	return response
}

// bindRequest bind the request to the Request struct.
func bindRequest(ctx *goskell.Context, defaultPageSize, maxPageSize uint16) (*ListInput, error) {
	var request ListInput

	if err := ctx.ShouldBind(&request); err != nil {
		return nil, err
	}

	if err := ctx.ShouldBindHeader(&request); err != nil {
		return nil, err
	}

	// Validate CustomerID
	customerID, err := cigourn.Parse(request.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization provided: %w", err)
	}

	var ok bool
	request.ParsedCustomerID, ok = customerID.(*online.User)
	if !ok {
		log.Error().Str("customerID", request.CustomerID).Msg("invalid authorization format: has to be an Online user")
		return nil, errors.New("invalid authorization format")
	}

	// Pagination
	request.PaginationParams = internal.PaginationParams{Page: 1, Size: defaultPageSize}
	if page := ctx.QueryMap("page"); len(page) != 0 {
		if pageRaw, ok := page["number"]; ok {
			number, err := strconv.Atoi(pageRaw)
			if err != nil || number <= 0 {
				return nil, errors.New("invalid input for page. Must be a number and can't be 0 or lower")
			}
			request.PaginationParams.Page = uint32(number)
		}
		if sizeRaw, ok := page["size"]; ok {
			size, err := strconv.ParseUint(sizeRaw, 10, 8)
			if err != nil || size <= 0 {
				return nil, errors.New("invalid input for size. Must be a number and can't be 0 or lower")
			}
			if uint16(size) > maxPageSize {
				return nil, fmt.Errorf("invalid page size. The max page size is: %d", maxPageSize)
			}
			request.PaginationParams.Size = uint16(size)
		}
	}

	//Filtering
	request.Filter = ctx.QueryMap("filter")
	request.Match = ctx.QueryMap("match")
	if err := validateQuery(FilterParam{Filter: request.Filter, Match: request.Match}); err != nil {
		return nil, err
	}

	return &request, nil

}

// ValidateQuery validates query data structure in detail
func validateQuery(param FilterParam) error {
	// ValidateFilters filters.
	for keyword, value := range param.Filter {
		if _, ok := validFilters[keyword]; !ok || len(strings.TrimSpace(value)) == 0 {
			return fmt.Errorf("invalid filter attribute `%s`", keyword)
		}
	}
	// ValidateMatch filters.
	for keyword, value := range param.Match {
		if _, ok := validMatch[keyword]; !ok || len(strings.TrimSpace(value)) == 0 {
			return fmt.Errorf("invalid match attribute `%s`", keyword)
		}
	}
	return nil
}
