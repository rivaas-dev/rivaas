package customers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"go.uber.org/mock/gomock"
)

// --- Mocks and helpers ---

func strptr(s string) *string { return &s }

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

func newHandlerForTests(kc keycloak.Client, oc *client.Client) *Handler {
	return &Handler{
		keycloakClient:  kc,
		keycloakConfig:  config.KeyCloakConfig{BrifRepresentation: true},
		omaClient:       oc,
		customerService: customers.New(nil, nil, nil),
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

	// Mock keycloak responses
	attrs := map[string][]string{}
	groups := []*keycloak.Group{
		{ID: strptr("g1"), Name: strptr("Alpha"), Attributes: &attrs},
		{ID: strptr("g2"), Name: strptr("Beta"), Attributes: &attrs},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupsCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(2, nil)
	kc.EXPECT().GetGroupsPaginated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(groups, nil)

	// OPA returns authorized
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)

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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": false}`))
	})
	h := newHandlerForTests(kc, oc)

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
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupsCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, assertErr("count error"))
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)

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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)

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
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupsCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(2, nil)
	kc.EXPECT().GetGroupsPaginated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assertErr("page error"))
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)

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

	attrs := map[string][]string{}
	group := &keycloak.Group{ID: strptr("cust-1"), Name: strptr("Acme"), Attributes: &attrs}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupByID(gomock.Any(), "cust-1").Return(group, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupByID(gomock.Any(), "cust-1").Return(nil, assertErr("group not found"))

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)

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
	kc := keycloak.NewMockClient(ctrl)
	// OPA returns 500 to trigger OMA client error
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	h := newHandlerForTests(kc, oc)
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

type strPtrValMatcher struct{ want string }

func (m strPtrValMatcher) Matches(x any) bool {
	p, ok := x.(*string)
	return ok && p != nil && *p == m.want
}
func (m strPtrValMatcher) String() string { return "is *string with wanted value" }

func TestLIST_Admin_WithNameFilter_PropagatesSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	attrs := map[string][]string{}
	groups := []*keycloak.Group{
		{ID: strptr("g1"), Name: strptr("Beta"), Attributes: &attrs},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupsCount(gomock.Any(), strPtrValMatcher{want: "Beta"}, gomock.Any()).Return(1, nil)
	kc.EXPECT().GetGroupsPaginated(gomock.Any(), strPtrValMatcher{want: "Beta"}, gomock.Any(), gomock.Any(), gomock.Any()).Return(groups, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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

	attrs := map[string][]string{}
	group := &keycloak.Group{
		ID: strptr("g1"), Name: strptr("Beta"), Attributes: &attrs,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupByID(gomock.Any(), "cust-1").Return(group, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)
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
	kc := keycloak.NewMockClient(ctrl)
	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})
	h := newHandlerForTests(kc, oc)
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
	attrs := map[string][]string{}
	groups := []*keycloak.Group{
		{ID: strptr("g1"), Name: strptr("OnlyOne"), Attributes: &attrs},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	kc := keycloak.NewMockClient(ctrl)
	kc.EXPECT().GetGroupsCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(2, nil)
	kc.EXPECT().GetGroupsPaginated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(groups, nil)

	oc := makeOMAClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": true}`))
	})

	h := newHandlerForTests(kc, oc)
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
