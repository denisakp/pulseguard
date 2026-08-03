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
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock component repository ---

type mockComponentRepo struct {
	components []*domain.Component
	component  *domain.Component
	createErr  error
	listErr    error
	findErr    error
	updateErr  error
	deleteErr  error
}

func (m *mockComponentRepo) Create(_ context.Context, c *domain.Component) (*domain.Component, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	c.Base = domain.Base{ID: "new-comp", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return c, nil
}

func (m *mockComponentRepo) List(_ context.Context, limit, offset int) ([]*domain.Component, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	end := offset + limit
	if end > len(m.components) {
		end = len(m.components)
	}
	if offset > len(m.components) {
		return []*domain.Component{}, nil
	}
	return m.components[offset:end], nil
}

func (m *mockComponentRepo) FindByID(_ context.Context, id string) (*domain.Component, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.component != nil {
		return m.component, nil
	}
	for _, c := range m.components {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockComponentRepo) Update(_ context.Context, _ *domain.Component) error {
	return m.updateErr
}

func (m *mockComponentRepo) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

func (m *mockComponentRepo) UpdateLastNotificationStatus(_ context.Context, _ string, _ domain.ComponentStatus) error {
	return nil
}

// --- mock tag service ---

type mockTagService struct {
	tags      []*domain.Tags
	tag       *domain.Tags
	createErr error
	listErr   error
	findErr   error
	updateErr error
	deleteErr error
}

func (m *mockTagService) CreateTag(_ context.Context, t *domain.Tags) error {
	if m.createErr != nil {
		return m.createErr
	}
	t.Base = domain.Base{ID: "new-tag", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return nil
}

func (m *mockTagService) ListTags(_ context.Context, limit, offset int) ([]*domain.Tags, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	end := offset + limit
	if end > len(m.tags) {
		end = len(m.tags)
	}
	if offset > len(m.tags) {
		return []*domain.Tags{}, nil
	}
	return m.tags[offset:end], nil
}

func (m *mockTagService) GetTagByID(_ context.Context, _ string) (*domain.Tags, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.tag, nil
}

func (m *mockTagService) UpdateTag(_ context.Context, _ string, _ string, _ *string, _ *string) (*domain.Tags, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.tag, nil
}

func (m *mockTagService) DeleteTag(_ context.Context, _ string) error {
	return m.deleteErr
}

// --- mock component service (v1 handler now goes through ComponentService) ---

type mockComponentService struct {
	list       []*dto.ComponentResponse
	one        *dto.ComponentResponse
	createErr  error
	updateErr  error
	deleteErr  error
	getErr     error
	listErr    error
	assignErr  error
	removeErr  error
	lastCreate *dto.CreateComponentPayload
	lastAssign []string
}

func (m *mockComponentService) CreateComponent(_ context.Context, p *dto.CreateComponentPayload) (*dto.ComponentResponse, error) {
	m.lastCreate = p
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &dto.ComponentResponse{ID: "new-comp", Name: p.Name}, nil
}
func (m *mockComponentService) UpdateComponent(_ context.Context, id string, _ *dto.UpdateComponentPayload) (*dto.ComponentResponse, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return &dto.ComponentResponse{ID: id}, nil
}
func (m *mockComponentService) DeleteComponent(_ context.Context, _ string) error { return m.deleteErr }
func (m *mockComponentService) GetComponent(_ context.Context, id string) (*dto.ComponentResponse, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.one != nil {
		return m.one, nil
	}
	return &dto.ComponentResponse{ID: id}, nil
}
func (m *mockComponentService) ListComponents(_ context.Context, _, _ int) ([]*dto.ComponentResponse, error) {
	return m.list, m.listErr
}
func (m *mockComponentService) BulkAssignToComponent(_ context.Context, _ string, p *dto.BulkAssignPayload) error {
	m.lastAssign = p.ResourceIDs
	return m.assignErr
}
func (m *mockComponentService) BulkRemoveFromComponent(_ context.Context, _ *dto.BulkRemovePayload) error {
	return m.removeErr
}

func newComponentRouter(svc v1.ComponentV1ServiceInterface) *chi.Mux {
	r := chi.NewRouter()
	h := v1.NewComponentHandler(svc)
	r.Get("/api/v1/components", h.List)
	r.With(middleware.RequireReadWrite).Post("/api/v1/components", h.Create)
	r.With(middleware.RequireReadWrite).Post("/api/v1/components/resources/bulk-remove", h.BulkRemove)
	r.Get("/api/v1/components/{id}", h.Get)
	r.With(middleware.RequireReadWrite).Put("/api/v1/components/{id}", h.Update)
	r.With(middleware.RequireReadWrite).Patch("/api/v1/components/{id}", h.Patch)
	r.With(middleware.RequireReadWrite).Delete("/api/v1/components/{id}", h.Delete)
	r.With(middleware.RequireReadWrite).Post("/api/v1/components/{id}/resources/bulk-assign", h.BulkAssign)
	return r
}

func newTagRouter(svc v1.TagV1ServiceInterface) *chi.Mux {
	r := chi.NewRouter()
	h := v1.NewTagHandler(svc)
	r.Get("/api/v1/tags", h.List)
	r.With(middleware.RequireReadWrite).Post("/api/v1/tags", h.Create)
	r.Get("/api/v1/tags/{id}", h.Get)
	r.With(middleware.RequireReadWrite).Put("/api/v1/tags/{id}", h.Update)
	r.With(middleware.RequireReadWrite).Delete("/api/v1/tags/{id}", h.Delete)
	return r
}

// T042: Scope enforcement tests for components and tags

func TestComponentHandler_ScopeEnforcement_ReadKey_Returns403OnWrite(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/components", []byte(`{"name":"comp"}`)},
		{"PUT", "/api/v1/components/abc", []byte(`{}`)},
		{"DELETE", "/api/v1/components/abc", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var b *bytes.Reader
			if tc.body != nil {
				b = bytes.NewReader(tc.body)
			} else {
				b = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, b)
			req = injectReadScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestComponentHandler_ScopeEnforcement_ReadWriteKey_NotForbiddenOnWrite(t *testing.T) {
	svc := &mockComponentService{one: &dto.ComponentResponse{ID: "c1", Name: "C1"}}
	router := newComponentRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/components", []byte(`{"name":"comp"}`)},
		{"PUT", "/api/v1/components/c1", []byte(`{}`)},
		{"DELETE", "/api/v1/components/c1", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var b *bytes.Reader
			if tc.body != nil {
				b = bytes.NewReader(tc.body)
			} else {
				b = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, b)
			req = injectReadWriteScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestComponentHandler_ScopeEnforcement_ReadKey_NotForbiddenOnGet(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/components", nil)
	req = injectReadScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusForbidden, rr.Code)
}

func TestTagHandler_ScopeEnforcement_ReadKey_Returns403OnWrite(t *testing.T) {
	svc := &mockTagService{}
	router := newTagRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/tags", []byte(`{"name":"tag"}`)},
		{"PUT", "/api/v1/tags/abc", []byte(`{}`)},
		{"DELETE", "/api/v1/tags/abc", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var b *bytes.Reader
			if tc.body != nil {
				b = bytes.NewReader(tc.body)
			} else {
				b = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, b)
			req = injectReadScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestTagHandler_ScopeEnforcement_ReadWriteKey_NotForbiddenOnWrite(t *testing.T) {
	tag := &domain.Tags{Base: domain.Base{ID: "t1", CreatedAt: time.Now(), UpdatedAt: time.Now()}, Name: "T1"}
	svc := &mockTagService{tags: []*domain.Tags{tag}, tag: tag}
	router := newTagRouter(svc)

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/api/v1/tags", []byte(`{"name":"tag"}`)},
		{"PUT", "/api/v1/tags/t1", []byte(`{}`)},
		{"DELETE", "/api/v1/tags/t1", nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var b *bytes.Reader
			if tc.body != nil {
				b = bytes.NewReader(tc.body)
			} else {
				b = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, b)
			req = injectReadWriteScope(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusForbidden, rr.Code)
		})
	}
}

func TestTagHandler_ScopeEnforcement_ReadKey_NotForbiddenOnGet(t *testing.T) {
	svc := &mockTagService{}
	router := newTagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	req = injectReadScope(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusForbidden, rr.Code)
}

// ============================================================
// Pagination validation tests
// ============================================================

func TestTagHandler_List_InvalidPagination_Returns422WithFields(t *testing.T) {
	svc := &mockTagService{}
	router := newTagRouter(svc)

	cases := []struct {
		name          string
		query         string
		expectedField string
	}{
		{"per_page=0", "?per_page=0", "per_page"},
		{"page=-1", "?page=-1", "page"},
		{"page=abc", "?page=abc", "page"},
		{"per_page=xyz", "?per_page=xyz", "per_page"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tags"+tc.query, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
			var out problemDetailResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
			assert.Equal(t, "VALIDATION_FAILED", out.Type)
			require.NotEmpty(t, out.Errors, "errors should be non-empty")
			assert.Equal(t, tc.expectedField, out.Errors[0].Field)
			assert.Equal(t, "must be a positive integer", out.Errors[0].Message)
		})
	}
}

func TestComponentHandler_List_InvalidPagination_Returns422WithFields(t *testing.T) {
	svc := &mockComponentService{}
	router := newComponentRouter(svc)

	cases := []struct {
		name          string
		query         string
		expectedField string
	}{
		{"per_page=0", "?per_page=0", "per_page"},
		{"page=-1", "?page=-1", "page"},
		{"page=abc", "?page=abc", "page"},
		{"per_page=xyz", "?per_page=xyz", "per_page"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/components"+tc.query, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
			var out problemDetailResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
			assert.Equal(t, "VALIDATION_FAILED", out.Type)
			require.NotEmpty(t, out.Errors, "errors should be non-empty")
			assert.Equal(t, tc.expectedField, out.Errors[0].Field)
			assert.Equal(t, "must be a positive integer", out.Errors[0].Message)
		})
	}
}
