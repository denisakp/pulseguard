package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hostTestDeps bundles the fakes + real services + handler used by the host
// HTTP-layer tests. The host handler takes concrete *service types, so we wire
// real services over in-memory repository fakes.
type hostTestDeps struct {
	credFake   *fake.HostCredentialFake
	hostFake   *fake.HostFake
	metricFake *fake.HostMetricFake
	resFake    *fake.ResourceFake
	credSvc    *service.HostCredentialService
	hostSvc    *service.HostService
	metricsSvc *service.HostMetricsService
	handler    *v1.HostHandler
	router     *chi.Mux
}

func newHostTestDeps() *hostTestDeps {
	credFake := fake.NewHostCredentialFake()
	hostFake := fake.NewHostFake()
	metricFake := fake.NewHostMetricFake()
	resFake := fake.NewResourceFake()

	credSvc := service.NewHostCredentialService(credFake)
	hostSvc := service.NewHostService(hostFake, credSvc, metricFake, resFake, 45*time.Second)
	metricsSvc := service.NewHostMetricsService(metricFake, hostFake, 48*time.Hour, 7*24*time.Hour)

	h := v1.NewHostHandler(hostSvc, metricsSvc)

	return &hostTestDeps{
		credFake:   credFake,
		hostFake:   hostFake,
		metricFake: metricFake,
		resFake:    resFake,
		credSvc:    credSvc,
		hostSvc:    hostSvc,
		metricsSvc: metricsSvc,
		handler:    h,
		router:     newHostRouter(h),
	}
}

// newHostRouter mirrors newComponentRouter / newMonitorRouter: mounts the host
// routes with RequireReadWrite wrapping all write endpoints.
func newHostRouter(h *v1.HostHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/hosts", h.List)
	r.With(middleware.RequireReadWrite).Post("/api/v1/hosts", h.Register)
	r.Get("/api/v1/hosts/{id}", h.Get)
	r.With(middleware.RequireReadWrite).Delete("/api/v1/hosts/{id}", h.Delete)
	r.Get("/api/v1/hosts/{id}/metrics", h.Metrics)
	r.With(middleware.RequireReadWrite).Post("/api/v1/hosts/{id}/credential/rotate", h.RotateCredential)
	r.With(middleware.RequireReadWrite).Post("/api/v1/hosts/{id}/credential/revoke", h.RevokeCredential)
	r.With(middleware.RequireReadWrite).Post("/api/v1/monitors/{id}/host", h.LinkMonitor)
	r.With(middleware.RequireReadWrite).Delete("/api/v1/monitors/{id}/host", h.UnlinkMonitor)
	return r
}

// registerHostResponse decodes the {data:{host,credential,prefix}} envelope.
type registerHostResponse struct {
	Data struct {
		Host struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Online bool   `json:"online"`
		} `json:"host"`
		Credential string `json:"credential"`
		Prefix     string `json:"prefix"`
	} `json:"data"`
}

// ============================================================
// Register: raw credential returned exactly once
// ============================================================

func TestHostHandler_Register_Returns201WithRawCredentialOnce(t *testing.T) {
	deps := newHostTestDeps()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts", bytes.NewReader([]byte(`{"name":"web-1"}`)))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var out registerHostResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "web-1", out.Data.Host.Name)
	assert.NotEmpty(t, out.Data.Host.ID)
	assert.NotEmpty(t, out.Data.Credential, "raw credential must be returned on registration")
	assert.True(t, strings.HasPrefix(out.Data.Credential, "ag_live_"), "credential should have ag_live_ prefix, got %q", out.Data.Credential)
	assert.NotEmpty(t, out.Data.Prefix)

	// A subsequent GET of the host must NOT expose any raw credential field.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/"+out.Data.Host.ID, nil)
	getRR := httptest.NewRecorder()
	deps.router.ServeHTTP(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &raw))
	data, ok := raw["data"].(map[string]any)
	require.True(t, ok, "GET should have a data object")
	assert.NotContains(t, data, "credential", "GET host must not expose a raw credential")
	assert.NotContains(t, data, "raw_credential")

	// Same guarantee for the list representation.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	listRR := httptest.NewRecorder()
	deps.router.ServeHTTP(listRR, listReq)
	require.Equal(t, http.StatusOK, listRR.Code)
	assert.NotContains(t, listRR.Body.String(), "ag_live_", "list must not leak the raw credential")
}

// ============================================================
// Scope enforcement (FR-019): read-only key → 403 on writes
// ============================================================

func TestHostHandler_ScopeEnforcement_ReadKey_Returns403OnWrites(t *testing.T) {
	deps := newHostTestDeps()

	// Seed a host so paths that reference an id are valid.
	host, _, _, err := deps.hostSvc.Register(context.Background(), "seed-host")
	require.NoError(t, err)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/hosts", []byte(`{"name":"x"}`)},
		{"DELETE", "/api/v1/hosts/" + host.ID, nil},
		{"POST", "/api/v1/hosts/" + host.ID + "/credential/rotate", nil},
		{"POST", "/api/v1/hosts/" + host.ID + "/credential/revoke", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			req = injectReadScope(req)
			rr := httptest.NewRecorder()
			deps.router.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestHostHandler_ScopeEnforcement_ReadWriteKey_NotForbiddenOnWrites(t *testing.T) {
	deps := newHostTestDeps()

	host, _, _, err := deps.hostSvc.Register(context.Background(), "seed-host")
	require.NoError(t, err)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/hosts", []byte(`{"name":"x"}`)},
		{"POST", "/api/v1/hosts/" + host.ID + "/credential/rotate", nil},
		{"POST", "/api/v1/hosts/" + host.ID + "/credential/revoke", nil},
		{"DELETE", "/api/v1/hosts/" + host.ID, nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			req = injectReadWriteScope(req)
			rr := httptest.NewRecorder()
			deps.router.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusForbidden, rr.Code)
		})
	}
}

// ============================================================
// Get / List / validation
// ============================================================

func TestHostHandler_Get_UnknownID_Returns404(t *testing.T) {
	deps := newHostTestDeps()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/does-not-exist", nil)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "RESOURCE_NOT_FOUND", out.Type)
}

func TestHostHandler_List_RegisteredHost_OnlineFalse(t *testing.T) {
	deps := newHostTestDeps()

	_, _, _, err := deps.hostSvc.Register(context.Background(), "web-1")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data []struct {
			Name   string `json:"name"`
			Online bool   `json:"online"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "web-1", out.Data[0].Name)
	assert.False(t, out.Data[0].Online, "a freshly registered host with no metrics is offline")
}

func TestHostHandler_Register_BlankName_Returns422(t *testing.T) {
	deps := newHostTestDeps()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts", bytes.NewReader([]byte(`{"name":"   "}`)))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "VALIDATION_FAILED", out.Type)
}
