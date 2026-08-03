package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock incident service ---

type mockIncidentService struct {
	incidents  []*domain.Incident
	incident   *domain.Incident
	eventSteps []domain.IncidentEventStep
	listErr    error
	getErr     error
	stepsErr   error
}

func (m *mockIncidentService) GetEventStepsForIncident(_ context.Context, _ string) ([]domain.IncidentEventStep, error) {
	if m.stepsErr != nil {
		return nil, m.stepsErr
	}
	return m.eventSteps, nil
}

func (m *mockIncidentService) ListAll(_ context.Context, limit, offset int) ([]*domain.Incident, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	end := offset + limit
	if end > len(m.incidents) {
		end = len(m.incidents)
	}
	if offset > len(m.incidents) {
		return []*domain.Incident{}, nil
	}
	return m.incidents[offset:end], nil
}

func (m *mockIncidentService) GetIncidentByID(_ context.Context, _ string) (*domain.Incident, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.incident, nil
}

func (m *mockIncidentService) ListByFilter(_ context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	// In-memory filter mimicking the SQL builder semantics.
	matched := make([]*domain.Incident, 0, len(m.incidents))
	for _, inc := range m.incidents {
		if f.Status != nil {
			isOpen := inc.ResolvedAt == nil
			switch *f.Status {
			case "open":
				if !isOpen {
					continue
				}
			case "resolved":
				if isOpen {
					continue
				}
			}
		}
		if f.MonitorID != nil && inc.ResourceID != *f.MonitorID {
			continue
		}
		if f.From != nil && inc.StartedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && inc.StartedAt.After(*f.To) {
			continue
		}
		matched = append(matched, inc)
	}
	total := len(matched)
	offset := (page - 1) * perPage
	if offset >= total {
		return []*domain.Incident{}, total, nil
	}
	end := offset + perPage
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}

func newIncidentRouter(svc v1.IncidentV1ServiceInterface) *chi.Mux {
	r := chi.NewRouter()
	h := v1.NewIncidentHandler(svc)
	r.Get("/api/v1/incidents", h.List)
	r.Get("/api/v1/incidents/{id}", h.Get)
	r.Get("/api/v1/incidents/{id}/event-steps", h.GetEventSteps)
	return r
}

func makeIncidents(n int, resolved bool) []*domain.Incident {
	incidents := make([]*domain.Incident, n)
	for i := range incidents {
		inc := &domain.Incident{
			Base:       domain.Base{ID: "inc-" + string(rune('0'+i)), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			ResourceID: "res-1",
			Cause:      "test failure",
			StartedAt:  time.Now().Add(-time.Hour),
		}
		if resolved {
			t := time.Now()
			inc.ResolvedAt = &t
		}
		incidents[i] = inc
	}
	return incidents
}

func TestIncidentHandler_GetEventSteps_ReturnsSteps(t *testing.T) {
	msg := "resource down"
	svc := &mockIncidentService{eventSteps: []domain.IncidentEventStep{
		{Base: domain.Base{ID: "step-1"}, IncidentID: "inc-1", Step: domain.IncidentEventStepDetected, Message: &msg},
	}}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-1/event-steps", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "step-1", out.Data[0]["id"])
}

func TestIncidentHandler_GetEventSteps_EmptyIsArray(t *testing.T) {
	router := newIncidentRouter(&mockIncidentService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-1/event-steps", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.NotNil(t, out.Data)
	assert.Len(t, out.Data, 0)
}

// T024: Incident filter tests

func TestIncidentHandler_StatusFilter_Open_ReturnsOnlyOpen(t *testing.T) {
	openInc := makeIncidents(2, false)
	resolvedInc := makeIncidents(1, true)
	all := append(openInc, resolvedInc...)
	svc := &mockIncidentService{incidents: all}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?status=open", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	data := out["data"].([]interface{})
	for _, item := range data {
		m := item.(map[string]interface{})
		assert.Equal(t, "open", m["status"], "all returned incidents should be open")
	}
}

func TestIncidentHandler_StatusFilter_Resolved_ReturnsOnlyResolved(t *testing.T) {
	openInc := makeIncidents(1, false)
	resolvedInc := makeIncidents(2, true)
	all := append(openInc, resolvedInc...)
	svc := &mockIncidentService{incidents: all}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?status=resolved", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	data := out["data"].([]interface{})
	for _, item := range data {
		m := item.(map[string]interface{})
		assert.Equal(t, "resolved", m["status"], "all returned incidents should be resolved")
	}
}

// Renamed from _Returns422 — spec 051 moves filter-validation errors from
// 422 to 400 per the new contract (pagination errors stay 422).
func TestIncidentHandler_StatusFilter_Invalid_Returns400(t *testing.T) {
	svc := &mockIncidentService{}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?status=invalid", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "VALIDATION_FAILED", out.Type)
	assert.NotEmpty(t, out.Errors)
}

func TestIncidentHandler_MonitorIDFilter_FiltersResult(t *testing.T) {
	inc1 := &domain.Incident{
		Base:       domain.Base{ID: "inc-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ResourceID: "res-1",
		StartedAt:  time.Now().Add(-time.Hour),
	}
	inc2 := &domain.Incident{
		Base:       domain.Base{ID: "inc-2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ResourceID: "res-2",
		StartedAt:  time.Now().Add(-time.Hour),
	}
	svc := &mockIncidentService{incidents: []*domain.Incident{inc1, inc2}}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?monitor_id=res-1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	data := out["data"].([]interface{})
	for _, item := range data {
		m := item.(map[string]interface{})
		assert.Equal(t, "res-1", m["monitor_id"])
	}
}

func TestIncidentHandler_Get_NotFound_Returns404(t *testing.T) {
	svc := &mockIncidentService{incident: nil, getErr: nil}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/nonexistent-id", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "RESOURCE_NOT_FOUND", out.Type)
	assert.Contains(t, out.Detail, "incident not found")
}

func TestIncidentHandler_CombinedFilter_AppliesBoth(t *testing.T) {
	inc1 := &domain.Incident{
		Base:       domain.Base{ID: "inc-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ResourceID: "res-1", StartedAt: time.Now().Add(-time.Hour),
	}
	t2 := time.Now()
	inc2 := &domain.Incident{
		Base:       domain.Base{ID: "inc-2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ResourceID: "res-1", StartedAt: time.Now().Add(-time.Hour), ResolvedAt: &t2,
	}
	svc := &mockIncidentService{incidents: []*domain.Incident{inc1, inc2}}
	router := newIncidentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?monitor_id=res-1&status=open", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	data := out["data"].([]interface{})
	for _, item := range data {
		m := item.(map[string]interface{})
		assert.Equal(t, "res-1", m["monitor_id"])
		assert.Equal(t, "open", m["status"])
	}
}
