package service

import (
	"context"
	"sort"
	"strings"
	"time"

	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
)

// searchPerCategoryCap bounds how many rows each category contributes before
// scoring/merge, so a broad query stays within the latency budget (spec 084).
const searchPerCategoryCap = 50

// Search categories.
const (
	searchCategoryResource = "resource"
	searchCategoryIncident = "incident"
	searchCategoryPage     = "page"
)

// searchResourceRepo / searchIncidentRepo are the repository subsets the search needs.
type searchResourceRepo interface {
	ListResourcesByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error)
}

type searchIncidentRepo interface {
	ListIncidentsByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error)
}

// searchPage is a static command-palette nav target (mirrors useSearchPalette STATIC_PAGES).
type searchPage struct{ Label, Meta, Path string }

var searchPages = []searchPage{
	{"Overview", "Dashboard", "/overview"},
	{"Resources", "All monitors", "/resources"},
	{"Incidents", "Incident list", "/incidents"},
	{"Components", "Status grouping", "/components"},
	{"Maintenance", "Planned windows", "/maintenance"},
	{"Notifications", "Channels", "/notifications"},
	{"Escalation", "Policies", "/escalation"},
	{"API keys", "Settings", "/api-keys"},
	{"Account", "Settings", "/settings/account"},
	{"Sessions", "Settings", "/settings/sessions"},
}

// SearchOpts carries the parsed, validated request options.
type SearchOpts struct {
	Limit      int      // already clamped by the handler
	Categories []string // empty ⇒ all supported categories; unknown values ignored
}

// SearchService aggregates command-palette search across monitors, incidents, and
// app pages (spec 084). Read-only; matching is case-insensitive substring via the
// dynquery repositories; ranking is done here.
type SearchService struct {
	resources searchResourceRepo
	incidents searchIncidentRepo
}

func NewSearchService(resources searchResourceRepo, incidents searchIncidentRepo) *SearchService {
	return &SearchService{resources: resources, incidents: incidents}
}

type scored struct {
	res       dtoV1.SearchResult
	score     int
	updatedAt time.Time
}

// Search runs the aggregated search. `q` is expected pre-validated (len ≥ 2).
func (s *SearchService) Search(ctx context.Context, q string, opts SearchOpts) (dtoV1.SearchResponse, error) {
	start := time.Now()
	want := wantedCategories(opts.Categories)
	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	}

	var out []scored

	if want[searchCategoryResource] {
		qv := q
		rows, _, err := s.resources.ListResourcesByFilter(ctx, dynquery.MonitorFilter{Q: &qv}, 1, searchPerCategoryCap)
		if err != nil {
			return dtoV1.SearchResponse{}, err
		}
		for _, r := range rows {
			out = append(out, scored{
				res: dtoV1.SearchResult{
					ID: "resource:" + r.ID, Category: searchCategoryResource,
					Label: r.Name, Meta: r.Target, DeepLink: "/resources/" + r.ID,
				},
				score:     maxScore(q, r.Name, r.Target),
				updatedAt: r.UpdatedAt,
			})
		}
	}

	if want[searchCategoryIncident] {
		qv := q
		rows, _, err := s.incidents.ListIncidentsByFilter(ctx, dynquery.IncidentFilter{Q: &qv}, 1, searchPerCategoryCap)
		if err != nil {
			return dtoV1.SearchResponse{}, err
		}
		for _, i := range rows {
			label := i.Cause
			if strings.TrimSpace(label) == "" {
				label = "Incident"
			}
			out = append(out, scored{
				res: dtoV1.SearchResult{
					ID: "incident:" + i.ID, Category: searchCategoryIncident,
					Label: label, Meta: i.Resource.Name, DeepLink: "/incidents/" + i.ID,
				},
				score:     maxScore(q, i.Cause),
				updatedAt: i.UpdatedAt,
			})
		}
	}

	if want[searchCategoryPage] {
		for _, p := range searchPages {
			sc := maxScore(q, p.Label, p.Meta)
			if sc == 0 {
				continue
			}
			out = append(out, scored{
				res: dtoV1.SearchResult{
					ID: "page:" + p.Label, Category: searchCategoryPage,
					Label: p.Label, Meta: p.Meta, DeepLink: p.Path,
				},
				score: sc, // pages carry no updated_at → zero time (sorts last within a score tier)
			})
		}
	}

	sort.SliceStable(out, func(a, b int) bool {
		if out[a].score != out[b].score {
			return out[a].score > out[b].score
		}
		return out[a].updatedAt.After(out[b].updatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}

	results := make([]dtoV1.SearchResult, len(out))
	for i, s := range out {
		results[i] = s.res
	}
	return dtoV1.SearchResponse{
		Results:         results,
		Total:           len(results),
		QueryDurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// wantedCategories returns the set to search: empty input ⇒ all; unknown ignored.
func wantedCategories(cats []string) map[string]bool {
	all := map[string]bool{searchCategoryResource: true, searchCategoryIncident: true, searchCategoryPage: true}
	if len(cats) == 0 {
		return all
	}
	// Non-empty filter: keep only known categories. If the caller asked ONLY for
	// unknown categories, the result is deterministically empty (they narrowed to
	// nothing valid) — the handler passes nil (not this) when no filter is given.
	want := map[string]bool{}
	for _, c := range cats {
		c = strings.TrimSpace(strings.ToLower(c))
		if all[c] {
			want[c] = true
		}
	}
	return want
}

// maxScore ranks the query against the best of the given fields:
// exact (3) > prefix (2) > contained (1) > none (0). Case-insensitive.
func maxScore(q string, fields ...string) int {
	ql := strings.ToLower(strings.TrimSpace(q))
	best := 0
	for _, f := range fields {
		fl := strings.ToLower(f)
		switch {
		case fl == ql:
			if 3 > best {
				best = 3
			}
		case strings.HasPrefix(fl, ql):
			if 2 > best {
				best = 2
			}
		case strings.Contains(fl, ql):
			if 1 > best {
				best = 1
			}
		}
	}
	return best
}
