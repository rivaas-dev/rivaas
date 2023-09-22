package keycloak

import (
	"context"
	"fmt"
	"github.com/Nerzal/gocloak/v13"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"strings"
	"time"
)

const defaultSize = 50

type Config struct {
	URL          string
	Realm        string // a realm where we create groups, subgroups, etc.
	ClientID     string // client id can be found in the web panel
	ClientSecret string // client secret is generated in the web panel
	PageSize     int    // size of the page used for group search
}

type Client struct {
	token        *gocloak.JWT
	tokenCreated time.Time
	client       *gocloak.GoCloak
	config       Config
}

func New(ctx context.Context, config Config) (*Client, error) {
	var err error
	c := &Client{
		tokenCreated: time.Now(),
		config:       config,
		client:       gocloak.NewClient(config.URL),
	}

	// we want to have a middleware to refresh token before every request
	httpClient := resty.New()
	httpClient = httpClient.OnBeforeRequest(c.AddAuthTokenToRequest)
	c.client.SetRestyClient(httpClient)

	// login to keycloak
	c.token, err = c.client.LoginClient(ctx, c.config.ClientID, c.config.ClientSecret, c.config.Realm)
	if err != nil {
		return nil, fmt.Errorf("failed to login to keyCloak: %w", err)
	}

	if config.PageSize == 0 {
		config.PageSize = defaultSize
	}
	return c, nil
}

func (c *Client) AddAuthTokenToRequest(_ *resty.Client, request *resty.Request) error {
	if strings.Contains(request.URL, "/token") { // skip all auth requests
		return nil
	}
	err := c.refreshToken(request.Context())
	if err != nil {
		return err
	}

	request.SetAuthToken(c.token.AccessToken)
	return nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	if c.tokenCreated.Add(time.Duration(c.token.ExpiresIn) * time.Second).After(time.Now()) {
		return nil // not expired yet
	}

	var (
		newToken *gocloak.JWT
		err      error
	)
	// refresh token is still valid, let's try to refresh it
	if c.tokenCreated.Add(time.Duration(c.token.RefreshExpiresIn) * time.Second).After(time.Now()) {
		c.tokenCreated = time.Now()
		log.Debug().Msg("CustomerProvisionWorkflow: refreshing keycloak token...")
		newToken, err = c.client.RefreshToken(ctx, c.token.RefreshToken, c.config.ClientID, c.config.ClientSecret, c.config.Realm)
		if err == nil {
			c.token = newToken
			return nil // we managed to re-login
		}
		// if we failed, lets try logging in instead of refreshing
	}

	c.tokenCreated = time.Now()
	log.Debug().Msg("CustomerProvisionWorkflow: logging in to keycloak...")
	newToken, err = c.client.LoginClient(ctx, c.config.ClientID, c.config.ClientSecret, c.config.Realm)
	if err != nil {
		return err
	}

	c.token = newToken
	return nil
}
