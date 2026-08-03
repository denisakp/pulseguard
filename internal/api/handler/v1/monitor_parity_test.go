package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitorParity_ListReturnsRichShape locks the spec-085 Phase-2a enrichment:
// GET /api/v1/monitors now returns the rich dto.ResourceResponse (uptime_7d,
// incident_count_30d, tags-as-objects) instead of the thin MonitorResponse.
func TestMonitorParity_ListReturnsRichShape(t *testing.T) {
	up := 99.5
	inc := 3
	res := &domain.Resource{
		Name: "api", Type: domain.ResourceHTTP, Target: "https://x",
		Uptime7d: &up, IncidentCount30d: &inc,
		Tags: []*domain.Tags{{Base: domain.Base{ID: "tag1"}, Name: "prod"}},
	}
	res.ID = "m1"
	router := newMonitorRouter(&mockMonitorService{resources: []*domain.Resource{res}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors?per_page=50", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	row := body.Data[0]
	assert.Equal(t, 99.5, row["uptime_7d"], "rich field uptime_7d present")
	assert.EqualValues(t, 3, row["incident_count_30d"], "rich field incident_count_30d present")
	tags, ok := row["tags"].([]any)
	require.True(t, ok && len(tags) == 1, "tags is an array of objects")
	assert.Equal(t, "tag1", tags[0].(map[string]any)["id"], "tag object carries id (not just name)")
}

type liveStub struct {
	resp *dto.LiveSnapshotResponse
	err  error
}

func (s *liveStub) GetLiveSnapshot(_ context.Context, _ string) (*dto.LiveSnapshotResponse, error) {
	return s.resp, s.err
}

type uptimeStub struct {
	rows []dto.UptimeStatResponse
	err  error
}

func (s *uptimeStub) GetUptimeStats(_ context.Context, _ string) ([]dto.UptimeStatResponse, error) {
	return s.rows, s.err
}

func newParityRouter(svc v1.MonitorV1ServiceInterface, live *liveStub, up *uptimeStub) *chi.Mux {
	h := v1.NewMonitorHandler(svc, live, up)
	r := chi.NewRouter()
	r.Get("/api/v1/monitors/{id}/live", h.GetLive)
	r.Get("/api/v1/monitors/{id}/uptime-stats", h.GetUptimeStats)
	r.Post("/api/v1/monitors/{id}/tags", h.AddTag)
	r.Delete("/api/v1/monitors/{id}/tags/{tagID}", h.RemoveTag)
	r.Patch("/api/v1/monitors/{id}", h.Patch)
	return r
}

func doReq(router *chi.Mux, method, path string, body []byte) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestMonitorParity_Live(t *testing.T) {
	router := newParityRouter(&mockMonitorService{}, &liveStub{resp: &dto.LiveSnapshotResponse{}}, &uptimeStub{})
	rr := doReq(router, http.MethodGet, "/api/v1/monitors/m1/live", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	router404 := newParityRouter(&mockMonitorService{}, &liveStub{err: service.ErrResourceNotFound}, &uptimeStub{})
	rr = doReq(router404, http.MethodGet, "/api/v1/monitors/missing/live", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestMonitorParity_UptimeStats(t *testing.T) {
	router := newParityRouter(&mockMonitorService{}, &liveStub{}, &uptimeStub{rows: []dto.UptimeStatResponse{
		{UptimePercent: 99.9, SuccessfulCount: 10, TotalCount: 10},
		{UptimePercent: 50, SuccessfulCount: 1, TotalCount: 2},
	}})
	rr := doReq(router, http.MethodGet, "/api/v1/monitors/m1/uptime-stats", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Data dtoV1.MonitorUptimeStatsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "m1", body.Data.ResourceID)
	assert.Len(t, body.Data.Stats, 2)
	assert.Equal(t, 99.9, body.Data.Stats[0].UptimePercent)
}

func TestMonitorParity_TagsAdd204AndCaptures(t *testing.T) {
	svc := &mockMonitorService{}
	router := newParityRouter(svc, &liveStub{}, &uptimeStub{})
	rr := doReq(router, http.MethodPost, "/api/v1/monitors/m1/tags", []byte(`{"tag_ids":["t1","t2"]}`))
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, []string{"t1", "t2"}, svc.lastTagIDs)
}

func TestMonitorParity_TagRemove204(t *testing.T) {
	router := newParityRouter(&mockMonitorService{}, &liveStub{}, &uptimeStub{})
	rr := doReq(router, http.MethodDelete, "/api/v1/monitors/m1/tags/t1", nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMonitorParity_PatchIsPartial(t *testing.T) {
	svc := &mockMonitorService{}
	router := newParityRouter(svc, &liveStub{}, &uptimeStub{})
	rr := doReq(router, http.MethodPatch, "/api/v1/monitors/m1", []byte(`{"name":"renamed only"}`))
	require.Equal(t, http.StatusOK, rr.Code)
	// only the sent field is set in the partial payload; others stay nil (unset ⇒ preserved).
	require.NotNil(t, svc.lastUpdate)
	require.NotNil(t, svc.lastUpdate.Name)
	assert.Equal(t, "renamed only", *svc.lastUpdate.Name)
	assert.Nil(t, svc.lastUpdate.Interval, "unsent field must be nil (preserved)")
	assert.Nil(t, svc.lastUpdate.Target)
}
