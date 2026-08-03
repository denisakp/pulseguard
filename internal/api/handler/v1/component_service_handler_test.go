package v1_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentHandler_Create_RequiresResourceIDs(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	// name present but no resource_ids → 422 with a field error.
	body := []byte(`{"name":"API"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/components", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.NotEmpty(t, out.Errors)
	assert.Equal(t, "resource_ids", out.Errors[0].Field)
	assert.Nil(t, svc.lastCreate, "service not called when validation fails")
}

func TestComponentHandler_Create_PassesResourceIDsAndGroupingWindow(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	body := []byte(`{"name":"API","resource_ids":["r1","r2"],"grouping_window_seconds":60}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/components", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, svc.lastCreate)
	assert.Equal(t, []string{"r1", "r2"}, svc.lastCreate.ResourceIDs)
	require.NotNil(t, svc.lastCreate.GroupingWindowSeconds)
	assert.Equal(t, 60, *svc.lastCreate.GroupingWindowSeconds)
}

func TestComponentHandler_Get_ReturnsRichShape(t *testing.T) {
	svc := &mockComponentService{one: &dto.ComponentResponse{
		ID:                    "c1",
		Name:                  "API",
		Status:                "degraded",
		Resources:             []dto.ComponentResourceSnapshot{{ID: "r1", Name: "web", Status: "up"}},
		ImpactedResources:     []dto.ComponentResourceSnapshot{},
		GroupingWindowSeconds: 30,
		CreatedAt:             "2026-01-01T00:00:00Z",
		UpdatedAt:             "2026-01-02T00:00:00Z",
	}}
	router := newComponentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/components/c1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "degraded", out.Data["status"], "derived status present")
	assert.EqualValues(t, 30, out.Data["grouping_window_seconds"])
	assert.Contains(t, out.Data, "resources")
	assert.Contains(t, out.Data, "impacted_resources")
	assert.Equal(t, "2026-01-02T00:00:00Z", out.Data["updated_at"], "timestamps present")
}

func TestComponentHandler_Delete_GuardReturns409(t *testing.T) {
	svc := &mockComponentService{deleteErr: service.ErrValidationFailed}
	router := newComponentRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/components/c1", nil)
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "COMPONENT_HAS_RESOURCES", out.Type)
}

func TestComponentHandler_BulkAssign_CapturesResourceIDs(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	body := []byte(`{"resource_ids":["r1","r2","r3"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/components/c1/resources/bulk-assign", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{"r1", "r2", "r3"}, svc.lastAssign)
}

func TestComponentHandler_BulkRemove_Works(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	body := []byte(`{"resource_ids":["r1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/components/resources/bulk-remove", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
