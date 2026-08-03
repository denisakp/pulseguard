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
	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock notification channel service ---

type mockChannelService struct {
	channels      []*domain.NotificationChannel
	channel       *domain.NotificationChannel
	listErr       error
	getErr        error
	createErr     error
	updateErr     error
	deleteErr     error
	testErr       error
	testConfigErr error
	stats         *service.NotificationStats
	statsErr      error
	lastUpdate    *dto.UpdateNotificationChannelPayload
}

func (m *mockChannelService) TestNotificationChannel(_ context.Context, _ string) error {
	return m.testErr
}
func (m *mockChannelService) ValidateAndTestChannelConfig(_ context.Context, _ domain.NotificationChannelType, _ json.RawMessage) error {
	return m.testConfigErr
}
func (m *mockChannelService) Stats(_ context.Context) (*service.NotificationStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.stats, nil
}

func (m *mockChannelService) ListNotificationChannels(_ context.Context, limit, offset int) ([]*domain.NotificationChannel, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	end := offset + limit
	if end > len(m.channels) {
		end = len(m.channels)
	}
	if offset > len(m.channels) {
		return []*domain.NotificationChannel{}, nil
	}
	return m.channels[offset:end], nil
}

func (m *mockChannelService) GetNotificationChannel(_ context.Context, _ string) (*domain.NotificationChannel, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.channel, nil
}

func (m *mockChannelService) CreateNotificationChannel(_ context.Context, payload *dto.CreateNotificationChannelPayload) (*domain.NotificationChannel, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &domain.NotificationChannel{
		Base: domain.Base{ID: "new-ch", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name: payload.Name,
		Type: payload.Type,
	}, nil
}

func (m *mockChannelService) UpdateNotificationChannel(_ context.Context, id string, payload *dto.UpdateNotificationChannelPayload) (*domain.NotificationChannel, error) {
	m.lastUpdate = payload
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	cfg := payload.Config
	if len(cfg) == 0 {
		cfg = []byte("{}")
	}
	return &domain.NotificationChannel{Base: domain.Base{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}, Config: cfg}, nil
}

func (m *mockChannelService) DeleteNotificationChannel(_ context.Context, _ string) error {
	return m.deleteErr
}

func newChannelRouter(svc v1.ChannelV1ServiceInterface) *chi.Mux {
	r := chi.NewRouter()
	h := v1.NewNotificationChannelHandler(svc)
	r.Get("/api/v1/notification-channels", h.List)
	r.With(middleware.RequireReadWrite).Post("/api/v1/notification-channels", h.Create)
	r.With(middleware.RequireReadWrite).Post("/api/v1/notification-channels/test-config", h.TestConfig)
	r.Get("/api/v1/notification-channels/{id}", h.Get)
	r.With(middleware.RequireReadWrite).Put("/api/v1/notification-channels/{id}", h.Update)
	r.With(middleware.RequireReadWrite).Patch("/api/v1/notification-channels/{id}", h.Patch)
	r.With(middleware.RequireReadWrite).Delete("/api/v1/notification-channels/{id}", h.Delete)
	r.With(middleware.RequireReadWrite).Post("/api/v1/notification-channels/{id}/test", h.Test)
	r.Get("/api/v1/notifications/stats", h.Stats)
	return r
}

// T025: Notification channel scope tests

func TestChannelHandler_ScopeEnforcement_ReadKey_Returns403OnWrite(t *testing.T) {
	svc := &mockChannelService{}
	router := newChannelRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/notification-channels", []byte(`{"name":"x","type":"smtp","config":{}}`)},
		{"PUT", "/api/v1/notification-channels/abc", []byte(`{}`)},
		{"DELETE", "/api/v1/notification-channels/abc", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *bytes.Reader
			if tc.body != nil {
				body = bytes.NewReader(tc.body)
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req = injectReadScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code, "%s %s with read scope should be 403", tc.method, tc.path)
		})
	}
}

func TestChannelHandler_ScopeEnforcement_ReadWriteKey_NotForbiddenOnWrite(t *testing.T) {
	ch := &domain.NotificationChannel{
		Base:   domain.Base{ID: "ch-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:   "Test",
		Type:   domain.NotificationChannelTypeSMTP,
		Config: []byte(`{}`),
	}
	svc := &mockChannelService{channels: []*domain.NotificationChannel{ch}, channel: ch}
	router := newChannelRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/notification-channels", []byte(`{"name":"x","type":"smtp","config":{}}`)},
		{"PUT", "/api/v1/notification-channels/ch-1", []byte(`{}`)},
		{"DELETE", "/api/v1/notification-channels/ch-1", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *bytes.Reader
			if tc.body != nil {
				body = bytes.NewReader(tc.body)
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req = injectReadWriteScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestChannelHandler_Response_DoesNotExposePassword(t *testing.T) {
	ch := &domain.NotificationChannel{
		Base:   domain.Base{ID: "ch-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:   "SMTP",
		Type:   domain.NotificationChannelTypeSMTP,
		Config: []byte(`{"host":"smtp.example.com","port":587,"username":"user","password":"secret"}`),
	}
	svc := &mockChannelService{channel: ch}
	router := newChannelRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notification-channels/ch-1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.NotContains(t, body, "secret", "response should not expose password")

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	data := out["data"].(map[string]interface{})
	config, ok := data["config"].(map[string]interface{})
	if ok {
		_, hasPassword := config["password"]
		assert.False(t, hasPassword, "config should not include password field")
	}
}

func TestChannelHandler_Get_ExposesRichFields(t *testing.T) {
	ch := &domain.NotificationChannel{
		Base:             domain.Base{ID: "ch-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:             "SMTP",
		Type:             domain.NotificationChannelTypeSMTP,
		Config:           []byte(`{"host":"smtp.example.com","password":"secret"}`),
		EnabledByDefault: true,
		Failures24h:      3,
	}
	svc := &mockChannelService{channel: ch}
	router := newChannelRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notification-channels/ch-1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "SMTP", out.Data["name"], "name is present (was dropped by the old thin DTO)")
	assert.Equal(t, true, out.Data["enabled_by_default"], "enabled_by_default present, not is_default/is_enabled")
	assert.EqualValues(t, 3, out.Data["failures_24h"], "telemetry present")
	_, hasIsEnabled := out.Data["is_enabled"]
	assert.False(t, hasIsEnabled, "bogus duplicate is_enabled removed")
}

// TestChannelHandler_Update_PreservesMaskedSecret locks the masking↔edit contract:
// because GET masks the password, an update that re-sends config WITHOUT the password
// must keep the stored one (not wipe it).
func TestChannelHandler_Update_PreservesMaskedSecret(t *testing.T) {
	ch := &domain.NotificationChannel{
		Base:   domain.Base{ID: "ch-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:   "SMTP",
		Type:   domain.NotificationChannelTypeSMTP,
		Config: []byte(`{"host":"old.example.com","password":"stored-secret"}`),
	}
	svc := &mockChannelService{channel: ch}
	router := newChannelRouter(svc)

	// New host, no password (it was masked out of the response the FE received).
	body := []byte(`{"config":{"host":"new.example.com"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notification-channels/ch-1", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, svc.lastUpdate)
	require.NotNil(t, svc.lastUpdate.Config)
	var merged map[string]interface{}
	require.NoError(t, json.Unmarshal(svc.lastUpdate.Config, &merged))
	assert.Equal(t, "new.example.com", merged["host"], "new host applied")
	assert.Equal(t, "stored-secret", merged["password"], "stored secret preserved")
}

func TestChannelHandler_Test_Sends(t *testing.T) {
	svc := &mockChannelService{}
	router := newChannelRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notification-channels/ch-1/test", bytes.NewReader([]byte(`{}`)))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	svc.testErr = service.ErrValidationFailed
	req = httptest.NewRequest(http.MethodPost, "/api/v1/notification-channels/ch-1/test", bytes.NewReader([]byte(`{}`)))
	req = injectReadWriteScope(req)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestChannelHandler_TestConfig_Validates(t *testing.T) {
	svc := &mockChannelService{}
	router := newChannelRouter(svc)
	body := []byte(`{"type":"smtp","config":{"host":"x"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notification-channels/test-config", bytes.NewReader(body))
	req = injectReadWriteScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestChannelHandler_Stats_ReturnsCounters(t *testing.T) {
	svc := &mockChannelService{stats: &service.NotificationStats{Sent30d: 12, Pending: 1, Failed24h: 2}}
	router := newChannelRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.EqualValues(t, 12, out.Data["sent_30d"])
	assert.EqualValues(t, 2, out.Data["failed_24h"])
}

func TestChannelHandler_Get_NotFound_Returns404(t *testing.T) {
	svc := &mockChannelService{getErr: service.ErrResourceNotFound}
	router := newChannelRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notification-channels/nonexistent-id", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	var out problemDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "RESOURCE_NOT_FOUND", out.Type)
	assert.Contains(t, out.Detail, "channel not found")
}
