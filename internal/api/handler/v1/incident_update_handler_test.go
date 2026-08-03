package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock incident-update provider ---

type mockIncidentUpdateProvider struct {
	rows       []*domain.IncidentUpdate
	created    *domain.IncidentUpdate
	updated    *domain.IncidentUpdate
	updateErr  error
	deleteErr  error
	lastPosted string
	lastStatus domain.IncidentUpdateStatus
}

func (m *mockIncidentUpdateProvider) ListByIncident(_ context.Context, _ string) ([]*domain.IncidentUpdate, error) {
	return m.rows, nil
}
func (m *mockIncidentUpdateProvider) Create(_ context.Context, incidentID string, status domain.IncidentUpdateStatus, message, postedBy string) (*domain.IncidentUpdate, error) {
	m.lastPosted = postedBy
	m.lastStatus = status
	u := &domain.IncidentUpdate{
		Base: domain.Base{ID: "upd-new", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		IncidentID: incidentID, Status: status, Message: message, PostedBy: postedBy, PostedAt: time.Now(),
	}
	m.created = u
	return u, nil
}
func (m *mockIncidentUpdateProvider) Update(_ context.Context, id string, status domain.IncidentUpdateStatus, message string) (*domain.IncidentUpdate, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	u := &domain.IncidentUpdate{Base: domain.Base{ID: id, UpdatedAt: time.Now()}, Status: status, Message: message, PostedAt: time.Now()}
	m.updated = u
	return u, nil
}
func (m *mockIncidentUpdateProvider) Delete(_ context.Context, _ string) error { return m.deleteErr }

func newIncidentUpdateRouter(svc v1.IncidentUpdateProvider) *chi.Mux {
	h := v1.NewIncidentUpdateHandler(svc)
	r := chi.NewRouter()
	r.Get("/api/v1/incidents/{id}/updates", h.List)
	r.Post("/api/v1/incidents/{id}/updates", h.Create)
	r.Patch("/api/v1/incidents/{id}/updates/{updateID}", h.Update)
	r.Delete("/api/v1/incidents/{id}/updates/{updateID}", h.Delete)
	return r
}

func TestIncidentUpdate_List_WrapsInDataEnvelope(t *testing.T) {
	svc := &mockIncidentUpdateProvider{rows: []*domain.IncidentUpdate{
		{Base: domain.Base{ID: "u1", CreatedAt: time.Now(), UpdatedAt: time.Now()}, IncidentID: "inc-1", Status: domain.IncidentUpdateInvestigating, Message: "looking", PostedAt: time.Now()},
	}}
	router := newIncidentUpdateRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-1/updates", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "u1", out.Data[0]["id"])
	assert.Equal(t, "investigating", out.Data[0]["status"])
}

func TestIncidentUpdate_Create_201AndCapturesPostedBy(t *testing.T) {
	svc := &mockIncidentUpdateProvider{}
	router := newIncidentUpdateRouter(svc)

	body := []byte(`{"status":"identified","message":"root cause found"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/inc-1/updates", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "user_id", "user-42")) //nolint:staticcheck // existing key
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "user-42", svc.lastPosted, "actor id from context threads to Create")
	assert.Equal(t, domain.IncidentUpdateIdentified, svc.lastStatus)
	var out struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "root cause found", out.Data["message"])
}

func TestIncidentUpdate_Update_NotFoundMaps404(t *testing.T) {
	svc := &mockIncidentUpdateProvider{updateErr: repository.ErrNotFound}
	router := newIncidentUpdateRouter(svc)

	body := []byte(`{"status":"resolved","message":"done"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incidents/inc-1/updates/missing", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIncidentUpdate_Delete_204(t *testing.T) {
	router := newIncidentUpdateRouter(&mockIncidentUpdateProvider{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/incidents/inc-1/updates/u1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}
