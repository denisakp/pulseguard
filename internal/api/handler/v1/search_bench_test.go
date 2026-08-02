package v1_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/store"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/go-chi/chi/v5"
)

func buildSearchBenchRouter(b *testing.B, fx *internaltest.DialectFixture) http.Handler {
	b.Helper()
	rr := store.NewResourceRepositorySQLC(fx.Runtime)
	ir := store.NewIncidentRepositorySQLC(fx.Runtime)
	h := v1.NewSearchHandler(service.NewSearchService(rr, ir))
	r := chi.NewRouter()
	r.Get("/api/v1/search", h.List)
	return r
}

// BenchmarkAPI_Search_p95 measures GET /api/v1/search latency (spec 084, SC-002:
// < 150 ms p95). It uses the shared 300-monitor / 100-incident bench fixture; the
// query "bench" matches both monitor names (`api-bench-res-*`) and incident causes
// (`bench_failure`), exercising the full resource + incident + pages path.
//
// To validate SC-002 at the target scale, bump apiBenchNumMonitors /
// apiBenchNumIncidents (bench_test.go) toward ~10k / ~50k and re-run `make bench-api`.
func BenchmarkAPI_Search_p95(b *testing.B) {
	fx := internaltest.SetupPostgres(b)
	if fx == nil {
		b.Skip("postgres backend unavailable")
		return
	}
	seedAPIBenchFixture(b, fx)
	router := buildSearchBenchRouter(b, fx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=bench", nil)
	const iterations = 1000
	durations := make([]time.Duration, 0, iterations)

	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("warm-up: unexpected status %d", w.Code)
		}
	}

	b.ResetTimer()
	for i := 0; i < iterations; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		durations = append(durations, time.Since(start))
		if w.Code != http.StatusOK {
			b.Fatalf("iter %d: unexpected status %d", i, w.Code)
		}
	}
	b.StopTimer()

	reportP95(b, "BenchmarkAPI_Search", durations)
}
