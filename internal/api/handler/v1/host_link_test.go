package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linkMonitorResponse decodes the {data:{...MonitorResponse}} envelope, focusing
// on the host_id link field.
type linkMonitorResponse struct {
	Data struct {
		ID     string  `json:"id"`
		HostID *string `json:"host_id"`
	} `json:"data"`
}

func TestHostHandler_LinkMonitor_SetsHostID(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	res, err := deps.resFake.Create(ctx, &domain.Resource{Name: "m", Type: domain.ResourceHTTP})
	require.NoError(t, err)
	host, _, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)

	body := []byte(`{"host_id":"` + host.ID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors/"+res.ID+"/host", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out linkMonitorResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, res.ID, out.Data.ID)
	require.NotNil(t, out.Data.HostID)
	assert.Equal(t, host.ID, *out.Data.HostID)

	// Repository truth: the monitor now points at the host.
	stored, err := deps.resFake.FindByID(ctx, res.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.HostID)
	assert.Equal(t, host.ID, *stored.HostID)
}

func TestHostHandler_UnlinkMonitor_ClearsHostID(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	res, err := deps.resFake.Create(ctx, &domain.Resource{Name: "m", Type: domain.ResourceHTTP})
	require.NoError(t, err)
	host, _, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)
	hid := host.ID
	require.NoError(t, deps.resFake.SetResourceHostID(ctx, res.ID, &hid))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/monitors/"+res.ID+"/host", nil)
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)

	// Regression: an unlinked monitor has host_id null.
	stored, err := deps.resFake.FindByID(ctx, res.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.HostID, "host_id must be nil after unlink")
}

func TestHostHandler_LinkMonitor_UnknownHost_Returns404(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	res, err := deps.resFake.Create(ctx, &domain.Resource{Name: "m", Type: domain.ResourceHTTP})
	require.NoError(t, err)

	body := []byte(`{"host_id":"unknown-host"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors/"+res.ID+"/host", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "RESOURCE_NOT_FOUND", out.Type)
}

func TestHostHandler_LinkMonitor_UnknownMonitor_Returns404(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	host, _, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)

	body := []byte(`{"host_id":"` + host.ID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors/unknown-monitor/host", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	deps.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "RESOURCE_NOT_FOUND", out.Type)
}

func TestHostHandler_LinkUnlink_ReadKey_Returns403(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	res, err := deps.resFake.Create(ctx, &domain.Resource{Name: "m", Type: domain.ResourceHTTP})
	require.NoError(t, err)
	host, _, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/monitors/" + res.ID + "/host", []byte(`{"host_id":"` + host.ID + `"}`)},
		{"DELETE", "/api/v1/monitors/" + res.ID + "/host", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			req = injectReadScope(req)
			rr := httptest.NewRecorder()
			deps.router.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}
