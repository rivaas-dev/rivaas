package key

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
)

func TestHandlerList_DBError(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys", nil)
	repo.EXPECT().GetKeys(gomock.Any()).Return(nil, errors.New("myError"))
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKeys(c)
	// get and compare results
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Equal(DBCommunicationErrorText, mappie["Title"])
}

func TestHandlerList_SuccessWithActor(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys?actor_id=hi", nil)
	description := "yes"
	l := time.Now()
	key := Key{
		Hash:         "hash1",
		ActorID:      "actor1",
		CreatedAt:    l,
		Description:  &description,
		QuotaEndDate: nil,
	}
	repo.EXPECT().GetKeys(gomock.Any()).Return([]*Key{&key}, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKeys(c)
	// get and compare results
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusOK, w.Result().StatusCode)
	a.Nil(err)
	var mappie []map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	expected := []map[string]interface{}{
		{
			"hash":          "hash1",
			"actor_id":      "actor1",
			"description":   "yes",
			"creation_date": l.Format(time.RFC3339Nano),
		},
	}
	a.Equal(expected, mappie)
}

func TestHandlerGet_TykError(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	httpResponse := http.Response{StatusCode: http.StatusOK}
	description := "yes"
	l := time.Now()
	key := Key{
		Hash:         "hash1",
		ActorID:      "actor1",
		CreatedAt:    l,
		Description:  &description,
		QuotaEndDate: nil,
	}
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tyk.SessionState{}, &httpResponse,
		errors.New("error"))
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(&key, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	// get and compare results
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Equal(GatewayCommunicationErrorText, mappie["Title"])
}

func TestHandlerGet_NoTykKey(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	httpResponse := http.Response{StatusCode: http.StatusNotFound}
	description := "yes"
	l := time.Now()
	key := Key{
		Hash:         "hash1",
		ActorID:      "actor1",
		CreatedAt:    l,
		Description:  &description,
		QuotaEndDate: nil,
	}
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tyk.SessionState{}, &httpResponse, nil)
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(&key, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	// get and compare results
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusOK, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	expected := map[string]interface{}{
		"actor_id":      "actor1",
		"description":   "yes",
		"creation_date": l.Format(time.RFC3339Nano),
	}
	a.Equal(expected, mappie)
}

func TestHandlerGet_SuccessQuota(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	setTime, err := date.CreateYmdFromString("2022-02-16")
	a.Nil(err)
	tykResponse := tyk.SessionState{
		ApplyPolicies:  []string{"p1", "p2"},
		QuotaMax:       500,
		QuotaRemaining: 400,
		Expires:        setTime.Unix(),
	}
	httpResponse := http.Response{StatusCode: 200}
	description := "yes"
	l := time.Now()
	key := Key{
		Hash:         "hash1",
		ActorID:      "actor1",
		CreatedAt:    l,
		Description:  &description,
		QuotaEndDate: nil,
	}
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykResponse, &httpResponse, nil)
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(&key, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	// get and compare results
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusOK, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	expected := map[string]interface{}{
		"actor_id":      "actor1",
		"quota":         float64(400),
		"description":   "yes",
		"policies":      []interface{}{"p1", "p2"},
		"creation_date": l.Format(time.RFC3339Nano),
	}
	a.Equal(expected, mappie)
}

func TestHandlerGet_SuccessUnlimitedQuota(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	setTime, err := date.CreateYmdFromString("2022-02-16")
	a.Nil(err)
	tykResponse := tyk.SessionState{
		ApplyPolicies:  []string{"p1", "p2"},
		QuotaMax:       -1,
		QuotaRemaining: -1,
		Expires:        setTime.Unix(),
	}

	httpResponse := http.Response{StatusCode: 200}
	description := "yes"
	l := time.Now()
	key := Key{
		Hash:         "hash1",
		ActorID:      "actor1",
		CreatedAt:    l,
		Description:  &description,
		QuotaEndDate: nil,
	}
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykResponse, &httpResponse, nil)
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(&key, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	result, err := io.ReadAll(w.Result().Body)
	// get and compare results
	a.Equal(http.StatusOK, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	expected := map[string]interface{}{
		"actor_id":      "actor1",
		"description":   "yes",
		"policies":      []interface{}{"p1", "p2"},
		"creation_date": l.Format(time.RFC3339Nano),
	}
	a.Equal(expected, mappie)
}

func TestHandlerGet_NotFound(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(nil, nil)
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	_, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusNotFound, w.Result().StatusCode)
	a.Nil(err)
}

func TestHandlerGet_DBError(t *testing.T) {
	// setup all test objects
	a, repo, client, policyCli, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/keys/hash1", nil)
	repo.EXPECT().GetKeyByHash(gomock.Any()).Return(nil, errors.New("error"))
	// execution
	handler := NewHandler(client, policyCli, repo)
	handler.HandleGETKey(c)
	// get and compare results
	res, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(res, &mappie)
	a.Nil(err)
	a.Equal(DBCommunicationErrorText, mappie["Title"])
}
