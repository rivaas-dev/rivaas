package keycloak

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestClient_New_APIError(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestClient_New(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestAddAuthTokenToRequest(t *testing.T) {
	authBody, err := os.ReadFile("./testdata/auth-response.json")
	assert.Nil(t, err)
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(authBody)
		assert.Nil(t, err)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	req := &resty.Request{}
	err = client.AddAuthTokenToRequest(client.client.RestyClient(), req)
	assert.NoError(t, err)
	assert.Equal(t, req.Token, "123token123")
}

func TestAddAuthTokenToRequest_RefreshError(t *testing.T) {
	authBody, err := os.ReadFile("./testdata/auth-response.json")
	assert.Nil(t, err)
	logInRequested := false
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logInRequested {
			// start returning error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(authBody)
		assert.Nil(t, err)
		logInRequested = true
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	req := &resty.Request{}
	client.tokenCreated = time.Time{} // to make it seem like the token expired
	err = client.AddAuthTokenToRequest(client.client.RestyClient(), req)
	assert.Error(t, err)
}

func TestRefreshToken_Refresh(t *testing.T) {
	authBody, err := os.ReadFile("./testdata/auth-response.json")
	assert.Nil(t, err)
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(authBody)
		assert.Nil(t, err)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// to make it seem like the token already expired but refresh token is stil valid
	client.token.ExpiresIn = 0
	client.token.RefreshExpiresIn = 360000
	err = client.refreshToken(context.Background())
	assert.NoError(t, err)
}

func TestRefreshToken_Login(t *testing.T) {
	authBody, err := os.ReadFile("./testdata/auth-response.json")
	assert.Nil(t, err)
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(authBody)
		assert.Nil(t, err)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// to make it seem like the access and refresh tokens already expired
	client.token.ExpiresIn = 0
	client.token.RefreshExpiresIn = 0
	err = client.refreshToken(context.Background())
	assert.NoError(t, err)
}

func TestRefreshToken_TokenStillValid(t *testing.T) {
	authBody, err := os.ReadFile("./testdata/auth-response.json")
	assert.Nil(t, err)
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(authBody)
		assert.Nil(t, err)
	}))
	defer ht.Close()

	client, err := New(context.Background(),
		Config{
			URL:      ht.URL,
			Realm:    "myrealm",
			ClientID: "admin-cli",
		})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// to make it seem like the access and refresh tokens haven't expired yet
	client.token.ExpiresIn = 3600000
	client.token.RefreshExpiresIn = 3600000
	err = client.refreshToken(context.Background())
	assert.NoError(t, err)
}
