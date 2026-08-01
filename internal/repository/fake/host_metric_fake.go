package fake

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// HostMetricFake is an in-memory HostMetricsRepository for tests.
type HostMetricFake struct {
	mu      sync.RWMutex
	samples []*domain.HostMetricSample
}

func NewHostMetricFake() *HostMetricFake {
	return &HostMetricFake{}
}

func (r *HostMetricFake) Insert(ctx context.Context, s *domain.HostMetricSample) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s.EnsureID()
	if s.SampledAt.IsZero() {
		s.SampledAt = time.Now()
	}
	cp := *s
	r.samples = append(r.samples, &cp)
	return nil
}

func (r *HostMetricFake) ListInRange(ctx context.Context, hostID string, from, to time.Time) ([]*domain.HostMetricSample, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*domain.HostMetricSample, 0)
	for _, s := range r.samples {
		if s.HostID != hostID {
			continue
		}
		if s.SampledAt.Before(from) || s.SampledAt.After(to) {
			continue
		}
		cp := *s
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SampledAt.Before(out[j].SampledAt) })
	return out, nil
}

func (r *HostMetricFake) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	kept := r.samples[:0]
	var deleted int64
	for _, s := range r.samples {
		if s.SampledAt.Before(cutoff) {
			deleted++
			continue
		}
		kept = append(kept, s)
	}
	r.samples = kept
	return deleted, nil
}

func (r *HostMetricFake) DeleteByHost(ctx context.Context, hostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	kept := r.samples[:0]
	for _, s := range r.samples {
		if s.HostID == hostID {
			continue
		}
		kept = append(kept, s)
	}
	r.samples = kept
	return nil
}

func (r *HostMetricFake) Decimate(ctx context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Keep the earliest sample per (host, minute) bucket for rows older than
	// cutoff; delete the rest. Rows at/after cutoff are always kept.
	type key struct {
		host   string
		minute int64
	}
	seen := make(map[key]time.Time)
	// First pass: find the earliest sampled_at per bucket (older-than-cutoff only).
	for _, s := range r.samples {
		if !s.SampledAt.Before(cutoff) {
			continue
		}
		k := key{host: s.HostID, minute: s.SampledAt.Unix() / 60}
		if existing, ok := seen[k]; !ok || s.SampledAt.Before(existing) {
			seen[k] = s.SampledAt
		}
	}
	kept := r.samples[:0]
	var deleted int64
	for _, s := range r.samples {
		if !s.SampledAt.Before(cutoff) {
			kept = append(kept, s)
			continue
		}
		k := key{host: s.HostID, minute: s.SampledAt.Unix() / 60}
		if earliest, ok := seen[k]; ok && s.SampledAt.Equal(earliest) {
			kept = append(kept, s)
			// Mark this bucket consumed so duplicates at the same instant are pruned.
			seen[k] = time.Time{}
			continue
		}
		deleted++
	}
	r.samples = kept
	return deleted, nil
}
