package key

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/request/patch"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gorm.io/gorm"
)

func TestHandlerPatch_TykUpdateError(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{
		patch.QuotaKey: float64(-1),
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)
	tykModifyResponse := tyk.ApiModifyKeySuccess{
		Key:     "12345",
		KeyHash: "123",
		Status:  "NO",
	}
	tykStateResponse := tyk.SessionState{
		ApplyPolicies:  []string{"p1", "p2"},
		QuotaMax:       -1,
		QuotaRemaining: -1,
	}
	dbReturnKey := Key{
		Hash:         "12345",
		ActorID:      "actor",
		QuotaEndDate: &ymdDate.Time,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	httpResponse := http.Response{StatusCode: 500}
	// Set high expectations
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykStateResponse, &httpResponse, nil)
	client.EXPECT().UpdateKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykModifyResponse, &httpResponse,
		errors.New("yes"))
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(&dbReturnKey, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	// validate result
	a.Nil(err)
	a.Contains(string(result), GatewayCommunicationErrorText)
}

func TestHandlerPatch_TykUpdateBadResponseCode(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{
		patch.QuotaKey: float64(-1),
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)
	tykModifyResponse := tyk.ApiModifyKeySuccess{
		Key:     "12345",
		KeyHash: "123",
		Status:  "NO",
	}
	tykStateResponse := tyk.SessionState{
		ApplyPolicies:  []string{"p1", "p2"},
		QuotaMax:       -1,
		QuotaRemaining: -1,
	}
	dbReturnKey := Key{
		Hash:         "12345",
		ActorID:      "actor",
		QuotaEndDate: &ymdDate.Time,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	httpResponse := http.Response{StatusCode: 500}
	// Set high expectations
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykStateResponse, &httpResponse, nil)
	client.EXPECT().UpdateKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykModifyResponse, &httpResponse, nil)
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(&dbReturnKey, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	// validate result
	a.Nil(err)
	a.Contains(string(result), GatewayCommunicationErrorText)
}

func TestHandlerPatch_TykNotFound(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{
		patch.QuotaKey: float64(-1),
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)
	dbReturnKey := Key{
		Hash:         "12345",
		ActorID:      "actor",
		QuotaEndDate: &ymdDate.Time,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	httpResponse := http.Response{StatusCode: 404}
	// Set high expectations
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tyk.SessionState{}, &httpResponse, nil)
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(&dbReturnKey, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusNotFound, w.Result().StatusCode)
	a.Nil(err)
	// validate result
	a.Nil(err)
	a.Contains(string(result), GatewayKeyNotFoundText)
}

func TestHandlerPatch_TykRetrieveError(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{
		patch.QuotaKey: float64(-1),
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)
	dbReturnKey := Key{
		Hash:         "12345",
		ActorID:      "actor",
		QuotaEndDate: &ymdDate.Time,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	// Set high expectations
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tyk.SessionState{}, nil, errors.New("yes baby"))
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(&dbReturnKey, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	// validate result
	a.Nil(err)
	a.Contains(string(result), GatewayCommunicationErrorText)
}

func TestHandlerPatch_BadRequest_ValidationErr(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	patchMap := map[string]interface{}{
		patch.PoliciesKey: []string{"syk"},
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)

	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies([]string{"existingPolicy", "well hello"}), nil, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Nil(err)
	a.Equal(http.StatusBadRequest, w.Result().StatusCode)
	a.Contains(string(result), "policy")
}

func TestHandlerPatch_BadRequest_WhatAreYouEvenDoing(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string("yes lawd")))
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Nil(err)
	a.Equal(http.StatusBadRequest, w.Result().StatusCode)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	// validate result
	a.Nil(err)
	a.Equal(http.StatusText(http.StatusBadRequest), mappie["Title"])
}

func TestHandlerPatch_BadRequest_BuildErr(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	patchMap := map[string]interface{}{
		patch.QuotaEndDateKey: 4,
	}
	badBody, _ := json.Marshal(patchMap)

	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(badBody)))
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Nil(err)
	a.Equal(http.StatusBadRequest, w.Result().StatusCode)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	// validate result
	a.Nil(err)
	a.Equal(patch.InvalidQuotaEndDateError, mappie["Title"])
}

func TestHandlerPatch_DBError(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{patch.QuotaEndDateKey: ymdDate.String()}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)

	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	// Set high expectations
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	// validate result
	a.Nil(err)
	a.Equal("some error", mappie["Title"])
}

func TestHandlerPatch_KeyNotFound(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	patchMap := map[string]interface{}{patch.QuotaEndDateKey: ymdDate.String()}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)

	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	// Set high expectations
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(nil, gorm.ErrRecordNotFound)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusNotFound, w.Result().StatusCode)
	a.Nil(err)
	a.Contains(string(result), DBKeyNotFoundText)
}

func TestHandlerPatch_Success(t *testing.T) {
	// build test objects
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	rawYmdDate := date.YmdDate{Time: time.Now().AddDate(0, 0, 1)}
	ymdDate, _ := date.CreateYmdFromString(rawYmdDate.String())
	description := "description"
	patchMap := map[string]interface{}{
		patch.QuotaEndDateKey: ymdDate.String(),
		patch.PoliciesKey:     []interface{}{"existingPolicy"},
		patch.QuotaKey:        float64(4),
		patch.DescriptionKey:  &description,
	}
	jsonBody, err := json.Marshal(patchMap)
	a.Nil(err)
	tykModifyResponse := tyk.ApiModifyKeySuccess{
		Key:     "12345",
		KeyHash: "123",
		Status:  tykStatusOK,
	}
	tykStateResponse := tyk.SessionState{
		ApplyPolicies:  []string{"p1", "p2"},
		QuotaMax:       -1,
		QuotaRemaining: -1,
	}
	dbReturnKey := Key{
		Hash:         "12345",
		ActorID:      "actor",
		QuotaEndDate: &ymdDate.Time,
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
		Description:  &description,
	}
	c.Request = httptest.NewRequest(http.MethodPatch, "/keys/1234", strings.NewReader(string(jsonBody)))
	policies := []string{"existingPolicy", "well hello"}
	httpResponse := http.Response{StatusCode: 200}
	// Set high expectations
	client.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykStateResponse, &httpResponse, nil)
	client.EXPECT().UpdateKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tykModifyResponse, &httpResponse, nil)
	repo.EXPECT().UpdateKeyByHash(gomock.Any(), gomock.Any()).Return(&dbReturnKey, nil)
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	// handle the request
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePATCHKey(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusOK, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	// validate result
	a.Nil(err)
	a.Equal("actor", mappie["actor_id"])
	a.Equal(ymdDate.String(), mappie["quota_end_date"])
}

func createTykPolicies(ids []string) []tyk.Policy {
	var out []tyk.Policy
	for _, id := range ids {
		out = append(out, tyk.Policy{Id: id})
	}
	return out
}
