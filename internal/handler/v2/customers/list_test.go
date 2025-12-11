package customers

import (
	"encoding/json"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"go.uber.org/mock/gomock"
)

// --- Mocks and helpers ---

func makeOMAClient(t *testing.T, opaHandler http.HandlerFunc) *client.Client {
	t.Helper()
	// OMA base is not used in these tests, but must be non-nil
	omaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(omaSrv.Close)

	opaSrv := httptest.NewServer(opaHandler)
	t.Cleanup(opaSrv.Close)

	baseOMA, _ := url.Parse(omaSrv.URL)
	baseOPA, _ := url.Parse(opaSrv.URL)
	// force http/1.1 client to work with httptest servers
	http1 := &http.Client{}
	c, err := client.New(baseOMA, baseOPA, client.WithHTTPClient(http1))
	if err != nil {
		t.Fatalf("failed to create oma client: %v", err)
	}
	return c
}

func newHandlerForTests(oc *client.Client, cs customers.ServiceInterface) *Handler {
	return &Handler{
		keycloakConfig:  config.KeyCloakConfig{BrifRepresentation: true},
		omaClient:       oc,
		customerService: cs,
		defaultPageSize: 1,
		maxPageSize:     10,
	}
}

func performRequest(r http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// --- Tests ---

func TestLIST_HappyPath_Admin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock customers service
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomersCount(gomock.Any(), gomock.Any()).Return(2, nil)

	// Create expected customer resources
	customerResources := []*customers.CustomerResource{
		{ID: "g1", Name: "Alpha"},
		{ID: "g2", Name: "Beta"},
	}
	cs.EXPECT().GetCustomersPaginated(gomock.Any(), gomock.Any()).Return(customerResources, nil)

	// OPA returns authorized
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[number]=1&page[size]=1", headersMap)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Validate meta in response
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta in response: %v", resp)
	}
	if tr, ok := meta["totalResults"].(float64); !ok || tr != 2 {
		t.Fatalf("expected totalResults=2, got %v", meta["totalResults"])
	}
	if tp, ok := meta["totalPages"].(float64); !ok || tp != 2 {
		t.Fatalf("expected totalPages=2, got %v", meta["totalPages"])
	}
}

func TestLIST_InvalidPagination_BadPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[size]=0", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_Forbidden_WhenOPAReturnsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": false}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InternalError_WhenKeycloakFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock customers service to return error
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomersCount(gomock.Any(), gomock.Any()).Return(0, assertErr("count error"))

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// assertErr is a small helper to create a simple error type with a fixed message

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestLIST_InvalidPagination_BadPageNumber(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[number]=0", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InternalError_WhenKeycloakPaginationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock customers service to return success for count but error for pagination
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomersCount(gomock.Any(), gomock.Any()).Return(2, nil)
	cs.EXPECT().GetCustomersPaginated(gomock.Any(), gomock.Any()).Return(nil, assertErr("page error"))

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_NonAdmin_Success_GetByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group := &customers.CustomerResource{
		ID:           "cust-1",
		Name:         "Acme",
		SalesforceID: "",
		Contacts:     nil,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomer(gomock.Any(), "cust-1").Return(group, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		// Non-admin: no administrator role
		headers.CIRoleHeader:     "user",
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[number]=1&page[size]=1", headersMap)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta in response: %v", resp)
	}
	if tr, ok := meta["totalResults"].(float64); !ok || tr != 1 {
		t.Fatalf("expected totalResults=1, got %v", meta["totalResults"])
	}
}

func TestLIST_NonAdmin_Error_GetByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomer(gomock.Any(), "cust-1").Return(nil, assertErr("group not found"))

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		// Non-admin
		headers.CIRoleHeader:     "user",
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InvalidAuthorizationHeader_BadCustomerID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)

	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader: headers.CIRoleAdministrator,
		// Bad customer id should make headers.GetAuthorization fail
		headers.CustomerIDHeader: "not-a-urn",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_AuthorizationCheck_ErrorOPA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	// OPA returns 500 to trigger OMA client error
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// custom matcher to verify *string value in gomock expectations

func TestLIST_Admin_WithNameFilter_PropagatesSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groups := []*customers.CustomerResource{{
		ID:           "g1",
		Name:         "Beta",
		SalesforceID: "",
		Contacts:     nil,
	},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	searchValue := "Beta"
	cs.EXPECT().GetCustomersCount(gomock.Any(), customer.ListParams{Search: &searchValue}).Return(1, nil)
	cs.EXPECT().GetCustomersPaginated(gomock.Any(), customer.ListParams{Search: &searchValue, First: 0, Max: 1}).Return(groups, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?match[name]=Beta", headersMap)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_NonAdmin_WithNameFilter_PropagatesSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group := &customers.CustomerResource{
		ID:           "cust-1",
		Name:         "Beta",
		SalesforceID: "",
		Contacts:     nil,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomer(gomock.Any(), "cust-1").Return(group, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     "user",
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?match[name]=Beta", headersMap)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InvalidPagination_SizeTooBig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[size]=11", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InvalidPagination_NonIntegerNumber(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[number]=abc", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_InvalidPagination_NonIntegerSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers?page[size]=abc", headersMap)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestLIST_DefaultPagination_Succeeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groups := []*customers.CustomerResource{{
		ID:           "g1",
		Name:         "OnlyOne",
		SalesforceID: "",
		Contacts:     nil,
	},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := customers.NewMockServiceInterface(ctrl)
	cs.EXPECT().GetCustomersCount(gomock.Any(), customer.ListParams{}).Return(2, nil)
	cs.EXPECT().GetCustomersPaginated(gomock.Any(), customer.ListParams{First: 0, Max: 1}).Return(groups, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(oc, cs)
	r := gin.New()
	r.GET("/customers", h.LIST)

	headersMap := map[string]string{
		headers.CIRoleHeader:     headers.CIRoleAdministrator,
		headers.CustomerIDHeader: "urn:online:user:cust-1:user-1",
	}
	rec := performRequest(r, http.MethodGet, "/customers", headersMap)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}
