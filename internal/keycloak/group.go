package keycloak

import (
	"context"
	"errors"
	"fmt"
	"github.com/Nerzal/gocloak/v13"
	"net/http"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	salesforceIDAttributesKey = "salesforceID"
)

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

type GroupAttributes struct {
	SalesforceID string
}

func (a GroupAttributes) ToMap() *map[string][]string {
	return &map[string][]string{
		salesforceIDAttributesKey: {a.SalesforceID},
	}
}

func ToGroupAttributes(attributes map[string][]string) (GroupAttributes, error) {
	sfSlice, ok := attributes[salesforceIDAttributesKey]
	if !ok {
		return GroupAttributes{}, fmt.Errorf("failed to find salesforce ID with key '%s'", salesforceIDAttributesKey)
	}
	if len(sfSlice) != 1 {
		return GroupAttributes{}, fmt.Errorf("failed to find salesforce ID: invalid length %d", len(sfSlice))
	}

	return GroupAttributes{
		SalesforceID: sfSlice[0],
	}, nil
}
