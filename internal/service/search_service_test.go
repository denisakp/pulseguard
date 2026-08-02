package service

import (
	"context"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
)

type fakeSearchResources struct{ rows []*domain.Resource }

func (f *fakeSearchResources) ListResourcesByFilter(_ context.Context, _ dynquery.MonitorFilter, _, _ int) ([]*domain.Resource, int, error) {
	return f.rows, len(f.rows), nil
}

type fakeSearchIncidents struct{ rows []*domain.Incident }

func (f *fakeSearchIncidents) ListIncidentsByFilter(_ context.Context, _ dynquery.IncidentFilter, _, _ int) ([]*domain.Incident, int, error) {
	return f.rows, len(f.rows), nil
}

func res(id, name, target string, updated time.Time) *domain.Resource {
	r := &domain.Resource{Name: name, Target: target}
	r.ID = id
	r.UpdatedAt = updated
	return r
}

func inc(id, cause, resourceName string, updated time.Time) *domain.Incident {
	i := &domain.Incident{Cause: cause}
	i.ID = id
	i.UpdatedAt = updated
	i.Resource = domain.Resource{Name: resourceName}
	return i
}

func newSearch(rs []*domain.Resource, is []*domain.Incident) *SearchService {
	return NewSearchService(&fakeSearchResources{rows: rs}, &fakeSearchIncidents{rows: is})
}

func TestSearch_ScoringOrder(t *testing.T) {
	now := time.Now()
	svc := newSearch([]*domain.Resource{
		res("1", "my-api", "x", now),     // contained → 1
		res("2", "api", "x", now),        // exact → 3
		res("3", "api-gateway", "x", now), // prefix → 2
	}, nil)
	resp, err := svc.Search(context.Background(), "api", SearchOpts{Limit: 10, Categories: []string{"resource"}})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{resp.Results[0].Label, resp.Results[1].Label, resp.Results[2].Label}
	want := []string{"api", "api-gateway", "my-api"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rank %d = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestSearch_TiebreakByUpdatedAt(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	svc := newSearch([]*domain.Resource{
		res("old", "api-old", "x", older), // prefix, older
		res("new", "api-new", "x", newer), // prefix, newer
	}, nil)
	resp, _ := svc.Search(context.Background(), "api", SearchOpts{Limit: 10, Categories: []string{"resource"}})
	if resp.Results[0].Label != "api-new" {
		t.Fatalf("expected newer first, got %v", []string{resp.Results[0].Label, resp.Results[1].Label})
	}
}

func TestSearch_CategoryFilter(t *testing.T) {
	now := time.Now()
	svc := newSearch(
		[]*domain.Resource{res("1", "apiserver", "x", now)},
		[]*domain.Incident{inc("9", "api timeout", "apiserver", now)},
	)
	resp, _ := svc.Search(context.Background(), "api", SearchOpts{Limit: 10, Categories: []string{"incident"}})
	for _, r := range resp.Results {
		if r.Category != "incident" {
			t.Fatalf("category filter leaked %q", r.Category)
		}
	}
	if len(resp.Results) != 1 {
		t.Fatalf("want 1 incident, got %d", len(resp.Results))
	}
}

func TestSearch_UnknownOnlyFilterEmpty(t *testing.T) {
	now := time.Now()
	svc := newSearch([]*domain.Resource{res("1", "api", "x", now)}, nil)
	resp, _ := svc.Search(context.Background(), "api", SearchOpts{Limit: 10, Categories: []string{"nope"}})
	if resp.Total != 0 || len(resp.Results) != 0 {
		t.Fatalf("all-unknown filter should be empty, got %d", resp.Total)
	}
}

func TestSearch_PagesMatch(t *testing.T) {
	svc := newSearch(nil, nil)
	resp, _ := svc.Search(context.Background(), "incid", SearchOpts{Limit: 10}) // all categories
	found := false
	for _, r := range resp.Results {
		if r.Category == "page" && r.Label == "Incidents" && r.DeepLink == "/incidents" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Incidents page in results, got %+v", resp.Results)
	}
}

func TestSearch_LimitCap(t *testing.T) {
	now := time.Now()
	rows := make([]*domain.Resource, 5)
	for i := range rows {
		rows[i] = res(string(rune('a'+i)), "api", "x", now)
	}
	svc := newSearch(rows, nil)
	resp, _ := svc.Search(context.Background(), "api", SearchOpts{Limit: 2, Categories: []string{"resource"}})
	if resp.Total != 2 || len(resp.Results) != 2 {
		t.Fatalf("limit=2 → want 2, got %d", resp.Total)
	}
}

func TestSearch_IncidentLabelFallbackAndMeta(t *testing.T) {
	now := time.Now()
	svc := newSearch(nil, []*domain.Incident{inc("9", "", "apiserver", now)})
	resp, _ := svc.Search(context.Background(), "ap", SearchOpts{Limit: 10, Categories: []string{"incident"}})
	// empty cause → label "Incident"; meta from the (hydrated) resource name.
	if len(resp.Results) != 1 || resp.Results[0].Label != "Incident" || resp.Results[0].Meta != "apiserver" {
		t.Fatalf("unexpected incident result: %+v", resp.Results)
	}
	if resp.Results[0].DeepLink != "/incidents/9" {
		t.Fatalf("deep link = %q", resp.Results[0].DeepLink)
	}
}
