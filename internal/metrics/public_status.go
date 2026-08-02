package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// PublicStatusMetrics exposes the counters needed to measure the public
// status page caching behaviour.
//
// Vocabulary:
//   - "hit" — the request carried a conditional header (If-None-Match or
//     If-Modified-Since), meaning a downstream cache had a stored copy.
//   - "miss" — first-touch request, no conditional header.
//
// The 95% cache ratio claim in the spec is measurable as
//
//	hits / (hits + misses) on the public_status_cache_requests metric.
type PublicStatusMetrics struct {
	hits   prometheus.Counter
	misses prometheus.Counter
}

func NewPublicStatusMetrics(reg prometheus.Registerer) *PublicStatusMetrics {
	hits := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "public_status_cache_hits_total",
		Help: "Public status requests carrying a conditional header (downstream cache hit).",
	})
	misses := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "public_status_cache_misses_total",
		Help: "Public status requests without a conditional header (downstream cache miss).",
	})
	reg.MustRegister(hits, misses)
	return &PublicStatusMetrics{hits: hits, misses: misses}
}

func (m *PublicStatusMetrics) RecordHit()  { m.hits.Inc() }
func (m *PublicStatusMetrics) RecordMiss() { m.misses.Inc() }
