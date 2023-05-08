package key

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/request/post"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler_TykError(t *testing.T) {
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	body := post.Post{
		Policies: []string{"existingPolicy"},
		ActorID:  "1234",
	}
	jsonBody, err := json.Marshal(body)
	a.Nil(err)
	c.Request = httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(string(jsonBody)))
	client.EXPECT().AddKey(gomock.Any(), gomock.Any()).Return(tyk.ApiModifyKeySuccess{}, nil, errors.New("tyk error"))
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies([]string{"existingPolicy", "well hello"}), nil, nil)

	handler := NewHandler(client, policyClient, repo)
	handler.HandlePOST(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Contains(mappie["Title"], "tyk error")
}

func TestHandler_DatabaseError(t *testing.T) {
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	body := post.Post{
		Policies: []string{"existingPolicy"},
		ActorID:  "1234",
	}
	jsonBody, err := json.Marshal(body)
	a.Nil(err)
	c.Request = httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(string(jsonBody)))
	client.EXPECT().AddKey(gomock.Any(), gomock.Any()).Return(tyk.ApiModifyKeySuccess{Key: "12345", KeyHash: "123"},
		nil, nil)
	client.EXPECT().DeleteKey(gomock.Any(), gomock.Any(), gomock.Any()).Return(tyk.ApiStatusMessage{}, nil,
		errors.New("delete error"))
	repo.EXPECT().StoreKey(gomock.Any()).Return(errors.New("storage error"))
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies([]string{"existingPolicy", "well hello"}), nil, nil)

	handler := NewHandler(client, policyClient, repo)
	handler.HandlePOST(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusInternalServerError, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Contains(mappie["Title"], "storage error")
}

func TestHandler_InvalidPolicy(t *testing.T) {
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	body := post.Post{
		Policies: []string{"nope"},
		ActorID:  "1234",
	}
	jsonBody, err := json.Marshal(body)
	a.Nil(err)
	c.Request = httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(string(jsonBody)))
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies([]string{"existingPolicy", "well hello"}), nil, nil)
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePOST(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusBadRequest, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Contains(mappie["Title"], "not available")
}

func TestHandler_InvalidInput(t *testing.T) {
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader("invalidBody"))
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePOST(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusBadRequest, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]interface{}
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Equal(http.StatusText(http.StatusBadRequest), mappie["Title"])
}

func TestHandlerPost_Success(t *testing.T) {
	a, repo, client, policyClient, w, c := constructAllTestObjects(t)
	nextYear := time.Now().AddDate(1, 0, 0)
	d := date.YmdDate{Time: nextYear}
	q := int64(4)
	body := post.Post{
		Policies:     []string{"existingPolicy"},
		ActorID:      "1234",
		QuotaEndDate: &d,
		Quota:        &q,
	}
	jsonBody, err := json.Marshal(body)
	a.Nil(err)
	c.Request = httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(string(jsonBody)))
	policies := []string{"existingPolicy", "well hello"}
	tykResponse := tyk.ApiModifyKeySuccess{
		Key:     "12345",
		KeyHash: "123",
	}
	client.EXPECT().AddKey(gomock.Any(), gomock.Any()).Return(tykResponse, nil, nil)
	repo.EXPECT().StoreKey(gomock.Any()).Return(nil)
	policyClient.EXPECT().ListPolicies(gomock.Any()).Return(createTykPolicies(policies), nil, nil)
	handler := NewHandler(client, policyClient, repo)
	handler.HandlePOST(c)
	result, err := io.ReadAll(w.Result().Body)
	a.Equal(http.StatusCreated, w.Result().StatusCode)
	a.Nil(err)
	var mappie map[string]string
	err = json.Unmarshal(result, &mappie)
	a.Nil(err)
	a.Equal("12345", mappie["key"])
	a.Equal("123", mappie["hash"])
}

// solely for code coverage, yes this is a stupid test
func TestHandler_Constructor(t *testing.T) {
	a := assert.New(t)
	h := NewHandler(nil, nil, nil)
	a.NotNil(h)
}

func constructAllTestObjects(t *testing.T) (*assert.Assertions, *MockRepositoryInterface, *MockClientInterface,
	*policy.MockClientInterface, *httptest.ResponseRecorder, *gin.Context) {
	a := assert.New(t)
	repositoryCtrl := gomock.NewController(t)
	keyClientCtrl := gomock.NewController(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	client := NewMockClientInterface(keyClientCtrl)
	repo := NewMockRepositoryInterface(repositoryCtrl)
	polyCtrl := gomock.NewController(t)
	policyCli := policy.NewMockClientInterface(polyCtrl)
	return a, repo, client, policyCli, w, c
}
