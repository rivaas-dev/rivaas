package keycloak

import (
	"context"
	"errors"
	"fmt"
	"github.com/Nerzal/gocloak/v13"
	"github.com/rs/zerolog/log"
	"net/http"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	actorIDAttributesKey = "actorID"
)

// GroupAttributes actorID
type GroupAttributes struct {
	ActorID string
}

// ToMap Returns the value
func (a GroupAttributes) ToMap() *map[string][]string {
	return &map[string][]string{
		actorIDAttributesKey: {a.ActorID},
	}
}

// ToGroupAttributes checks if the value is present
func ToGroupAttributes(attributes map[string][]string) (GroupAttributes, error) {
	sfSlice, ok := attributes[actorIDAttributesKey]
	if !ok {
		return GroupAttributes{}, fmt.Errorf("failed to find actor ID with key '%s'", actorIDAttributesKey)
	}
	if len(sfSlice) != 1 {
		return GroupAttributes{}, fmt.Errorf("failed to find actor ID: invalid length %d", len(sfSlice))
	}

	return GroupAttributes{
		ActorID: sfSlice[0],
	}, nil
}

// GetGroups calls keycloak api and retrieves the groups
func (c *Client) GetGroups(ctx context.Context) ([]*gocloak.Group, error) {

	groups, err := c.client.GetGroups(ctx, c.token.AccessToken, c.config.Realm, gocloak.GetGroupsParams{
		Full:                gocloak.BoolP(true),
		BriefRepresentation: gocloak.BoolP(false), // with this flag we also get attributes in the response
	})
	var apiErr *gocloak.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		// the group doesn't exist
		return nil, ErrNotFound
	}

	if err != nil {
		// unknown error occurred
		return nil, fmt.Errorf("failed to get a group: %w", err)
	}

	return groups, nil
}

// GetGroupByActorID fetches a single group based on actotID
func (c *Client) GetGroupByActorID(ctx context.Context, actorID string) (*gocloak.Group, error) {
	// Set variable that has the total number of pages
	currentPage := 0
	var (
		groups []*gocloak.Group
		err    error
	)
	for {
		groups, err = c.getGroups(ctx, currentPage*c.config.PageSize, c.config.PageSize)
		if err != nil {
			// unknown error occurred, finish search
			return nil, fmt.Errorf("failed to get a group: %w", err)
		}

		// iterate result and look for the Reference
		group, ok := getGroupWithActorID(groups, actorID)
		if ok {
			// we found the group with a given salesforce ID, finish search
			return group, nil
		}

		if len(groups) < c.config.PageSize {
			// last page, finish search
			return nil, ErrNotFound
		}

		currentPage++
	}
}

func getGroupWithActorID(groups []*gocloak.Group, actorID string) (*gocloak.Group, bool) {
	for _, group := range groups {
		if group == nil || group.Attributes == nil {
			continue // group is empty or doesn't have attributes, skip
		}
		attr, err := ToGroupAttributes(*group.Attributes)
		if err != nil {
			log.Err(err).Msg("failed to get attributes of a group")
			continue // couldn't find a actor ID in the group, skip
		}
		if attr.ActorID == actorID {
			return group, true // we found the group with a given actor ID, finish search
		}
	}
	return nil, false
}

func (c *Client) getGroups(ctx context.Context, skip, pageSize int) ([]*gocloak.Group, error) {
	groups, err := c.client.GetGroups(ctx, c.token.AccessToken, c.config.Realm, gocloak.GetGroupsParams{
		Max:                 gocloak.IntP(pageSize),
		First:               gocloak.IntP(skip),
		BriefRepresentation: gocloak.BoolP(false), // with this flag we also get attributes in the response
	})
	var apiErr *gocloak.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		// the group doesn't exist
		return nil, ErrNotFound
	}

	if err != nil {
		// unknown error occurred
		return nil, fmt.Errorf("failed to get a group: %w", err)
	}

	return groups, nil
}
