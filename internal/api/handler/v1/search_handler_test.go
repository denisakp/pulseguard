package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSearchService struct {
	lastQ    string
	lastOpts service.SearchOpts
	resp     dtoV1.SearchResponse
	err      error
}

func (m *mockSearchService) Search(_ context.Context, q string, opts service.SearchOpts) (dtoV1.SearchResponse, error) {
	m.lastQ = q
	m.lastOpts = opts
	return m.resp, m.err
}

func doSearch(t *testing.T, mock *mockSearchService, url string) *httptest.ResponseRecorder {
	t.Helper()
	h := v1.NewSearchHandler(mock)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	h.List(w, req)
	return w
}

func TestSearchHandler_ShortQueryIs422(t *testing.T) {
	for _, u := range []string{"/api/v1/search?q=a", "/api/v1/search", "/api/v1/search?q=%20%20"} {
		w := doSearch(t, &mockSearchService{}, u)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, u)
	}
}

func TestSearchHandler_HappyPath(t *testing.T) {
	mock := &mockSearchService{resp: dtoV1.SearchResponse{
		Results:         []dtoV1.SearchResult{{ID: "resource:1", Category: "resource", Label: "api", DeepLink: "/resources/1"}},
		Total:           1,
		QueryDurationMs: 3,
	}}
	w := doSearch(t, mock, "/api/v1/search?q=api")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "api", mock.lastQ)

	var body struct {
		Data dtoV1.SearchResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Results, 1)
	assert.Equal(t, "/resources/1", body.Data.Results[0].DeepLink)
}

func TestSearchHandler_LimitClamp(t *testing.T) {
	cases := map[string]int{
		"/api/v1/search?q=api":            30,  // default
		"/api/v1/search?q=api&limit=5":    5,   // honored
		"/api/v1/search?q=api&limit=1000": 100, // clamped
		"/api/v1/search?q=api&limit=0":    30,  // invalid → default
		"/api/v1/search?q=api&limit=abc":  30,  // non-numeric → default
	}
	for url, want := range cases {
		mock := &mockSearchService{}
		doSearch(t, mock, url)
		assert.Equal(t, want, mock.lastOpts.Limit, url)
	}
}

func TestSearchHandler_CategoriesParsed(t *testing.T) {
	mock := &mockSearchService{}
	doSearch(t, mock, "/api/v1/search?q=api&categories=incident,%20foo%20,")
	// handler splits + trims + drops empties; service does the known-set filtering.
	assert.Equal(t, []string{"incident", "foo"}, mock.lastOpts.Categories)

	mock2 := &mockSearchService{}
	doSearch(t, mock2, "/api/v1/search?q=api")
	assert.Nil(t, mock2.lastOpts.Categories, "no categories param ⇒ nil (all)")
}

func TestSearchHandler_MetacharsLiteral(t *testing.T) {
	mock := &mockSearchService{}
	w := doSearch(t, mock, "/api/v1/search?q=%25%25") // "%%"
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "%%", mock.lastQ, "query passed through literally")
}

func TestSearchHandler_ServiceErrorIs500(t *testing.T) {
	w := doSearch(t, &mockSearchService{err: context.DeadlineExceeded}, "/api/v1/search?q=api")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
