package v1_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/internal/repository/store"
	"github.com/go-chi/chi/v5"
)

// Spec 052 — pre-decom API baseline benchmark.
//
// Measures p95 of GET /api/v1/monitors and GET /api/v1/incidents using the
// real sqlc-backed repository path against a Postgres testcontainer. Captures
// the user-facing latency that SC-006 (API hot-path) regression gate
// monitors. T002 captures the baseline pre-decom; T040 re-runs post-decom and
// compares. Tolerance: ≤ +10 % p95.
//
// Run via `make bench-api` (which forwards to `go test -bench=^BenchmarkAPI_
// -benchtime=5s -count=3 ./internal/api/handler/v1/...`).

// ----------------------------------------------------------------------------
// Bench harness — thin V1ServiceInterface adapters over the real repos.
// Only the List* methods are exercised by the bench; non-List methods panic
// to catch accidental cross-talk.
// ----------------------------------------------------------------------------

type benchMonitorService struct {
	repo port.ResourceRepository
}

func (s *benchMonitorService) ListByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error) {
	return s.repo.ListResourcesByFilter(ctx, f, page, perPage)
}
func (s *benchMonitorService) ListActiveResources(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	return s.repo.FindActive(ctx, limit, offset)
}
func (s *benchMonitorService) ListAll(ctx context.Context) ([]*domain.Resource, error) {
	return s.repo.List(ctx, 1000, 0)
}
func (s *benchMonitorService) GetResourceByID(ctx context.Context, id string) (*domain.Resource, error) {
	return s.repo.FindByID(ctx, id)
}
func (s *benchMonitorService) CreateResource(_ context.Context, _ *dto.CreateResourcePayload) (*domain.Resource, error) {
	panic("bench harness: CreateResource not used")
}
func (s *benchMonitorService) UpdateResource(_ context.Context, _ string, _ *dto.UpdateResourcePayload) (*domain.Resource, error) {
	panic("bench harness: UpdateResource not used")
}
func (s *benchMonitorService) DeleteResource(_ context.Context, _ string) error {
	panic("bench harness: DeleteResource not used")
}
func (s *benchMonitorService) PauseMonitoring(_ context.Context, _ string) error {
	panic("bench harness: PauseMonitoring not used")
}
func (s *benchMonitorService) AddTagsToResource(_ context.Context, _ string, _ []string) error {
	panic("bench harness: AddTagsToResource not used")
}
func (s *benchMonitorService) RemoveTagFromResource(_ context.Context, _, _ string) error {
	panic("bench harness: RemoveTagFromResource not used")
}
func (s *benchMonitorService) ResumeMonitoring(_ context.Context, _ string) error {
	panic("bench harness: ResumeMonitoring not used")
}

type benchIncidentService struct {
	repo port.IncidentRepository
}

func (s *benchIncidentService) ListAll(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	return s.repo.List(ctx, limit, offset)
}
func (s *benchIncidentService) ListByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	return s.repo.ListIncidentsByFilter(ctx, f, page, perPage)
}
func (s *benchIncidentService) GetIncidentByID(ctx context.Context, id string) (*domain.Incident, error) {
	return s.repo.FindByID(ctx, id)
}

// ----------------------------------------------------------------------------
// Fixture: 300 monitors + 100 incidents in PG.
// ----------------------------------------------------------------------------

const (
	apiBenchNumMonitors  = 300
	apiBenchNumIncidents = 100
)

func seedAPIBenchFixture(b *testing.B, fx *internaltest.DialectFixture) {
	b.Helper()
	ctx := context.Background()
	resourceRepo := store.NewResourceRepositorySQLC(fx.Runtime)

	// Seed 300 monitors via the sqlc create path (sole impl post-decom).
	for i := 0; i < apiBenchNumMonitors; i++ {
		res := &domain.Resource{
			Base: domain.Base{
				ID:        fmt.Sprintf("api-bench-res-%04d", i),
				CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
			},
			Name:     fmt.Sprintf("api-bench-res-%04d", i),
			Type:     domain.ResourceHTTP,
			Target:   "https://example.com",
			IsActive: true,
			Interval: 60,
			Timeout:  10,
		}
		if _, err := resourceRepo.Create(ctx, res); err != nil {
			b.Fatalf("seed monitor %d: %v", i, err)
		}
	}

	// Seed 100 incidents distributed across the first 30 monitors.
	incidentRepo := store.NewIncidentRepositorySQLC(fx.Runtime)
	now := time.Now()
	for i := 0; i < apiBenchNumIncidents; i++ {
		resourceID := fmt.Sprintf("api-bench-res-%04d", i%30)
		startedAt := now.Add(-time.Duration(i) * time.Minute)
		inc := &domain.Incident{
			Base: domain.Base{
				ID:        fmt.Sprintf("api-bench-inc-%04d", i),
				CreatedAt: startedAt,
				UpdatedAt: startedAt,
			},
			ResourceID: resourceID,
			Cause:      "bench_failure",
			StartedAt:  startedAt,
		}
		// Half resolved, half open.
		if i%2 == 0 {
			r := startedAt.Add(5 * time.Minute)
			inc.ResolvedAt = &r
		}
		if _, err := incidentRepo.Create(ctx, inc); err != nil {
			b.Fatalf("seed incident %d: %v", i, err)
		}
	}
}

func buildBenchRouter(b *testing.B, fx *internaltest.DialectFixture) http.Handler {
	b.Helper()
	resourceRepo := store.NewResourceRepositorySQLC(fx.Runtime)
	incidentRepo := store.NewIncidentRepositorySQLC(fx.Runtime)
	monitorHandler := v1.NewMonitorHandler(&benchMonitorService{repo: resourceRepo}, nil, nil)
	incidentHandler := v1.NewIncidentHandler(&benchIncidentService{repo: incidentRepo})

	r := chi.NewRouter()
	r.Get("/api/v1/monitors", monitorHandler.List)
	r.Get("/api/v1/incidents", incidentHandler.List)
	return r
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkAPI_ListMonitors_p95(b *testing.B) {
	fx := internaltest.SetupPostgres(b)
	if fx == nil {
		b.Skip("postgres backend unavailable")
		return
	}
	seedAPIBenchFixture(b, fx)
	router := buildBenchRouter(b, fx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors?per_page=50", nil)
	const iterations = 1000
	durations := make([]time.Duration, 0, iterations)

	// Warm-up.
	for i := 0; i < 50; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("warm-up: unexpected status %d", rr.Code)
		}
	}

	b.ResetTimer()
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		durations = append(durations, time.Since(start))
		if rr.Code != http.StatusOK {
			b.Fatalf("iter %d: unexpected status %d", i, rr.Code)
		}
	}
	b.StopTimer()

	reportP95(b, "BenchmarkAPI_ListMonitors", durations)
}

func BenchmarkAPI_ListIncidents_p95(b *testing.B) {
	fx := internaltest.SetupPostgres(b)
	if fx == nil {
		b.Skip("postgres backend unavailable")
		return
	}
	seedAPIBenchFixture(b, fx)
	router := buildBenchRouter(b, fx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?per_page=50", nil)
	const iterations = 1000
	durations := make([]time.Duration, 0, iterations)

	for i := 0; i < 50; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("warm-up: unexpected status %d", rr.Code)
		}
	}

	b.ResetTimer()
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		durations = append(durations, time.Since(start))
		if rr.Code != http.StatusOK {
			b.Fatalf("iter %d: unexpected status %d", i, rr.Code)
		}
	}
	b.StopTimer()

	reportP95(b, "BenchmarkAPI_ListIncidents", durations)
}

// ----------------------------------------------------------------------------
// p95 reporter — emits a line testbench parsers + humans can both read.
// ----------------------------------------------------------------------------

func reportP95(b *testing.B, name string, durations []time.Duration) {
	b.Helper()
	if len(durations) == 0 {
		b.Fatalf("no durations captured")
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[len(durations)*50/100]
	p95 := durations[len(durations)*95/100]
	p99 := durations[len(durations)*99/100]
	line := fmt.Sprintf("api_bench name=%s iterations=%d p50_us=%d p95_us=%d p99_us=%d",
		name, len(durations), p50.Microseconds(), p95.Microseconds(), p99.Microseconds())
	b.Log(line)
	// Also write to stdout for `make bench-api` grep.
	fmt.Println(line)
}
