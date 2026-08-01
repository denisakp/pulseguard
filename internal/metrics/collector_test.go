package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- function-pointer stub repositories ----

type stubResourceRepo struct {
	findActive func(ctx context.Context, limit, offset int) ([]*domain.Resource, error)
}

func (s *stubResourceRepo) FindActive(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	return s.findActive(ctx, limit, offset)
}
func (s *stubResourceRepo) Create(ctx context.Context, r *domain.Resource) (*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) SetResourceHostID(ctx context.Context, resourceID string, hostID *string) error {
	return nil
}
func (s *stubResourceRepo) ClearResourceHostIDByHost(ctx context.Context, hostID string) error {
	return nil
}
func (s *stubResourceRepo) FindByID(ctx context.Context, id string) (*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) FindByHeartbeatSlug(ctx context.Context, slug string) (*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) List(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) Update(ctx context.Context, r *domain.Resource) error { return nil }
func (s *stubResourceRepo) Delete(ctx context.Context, id string) error          { return nil }
func (s *stubResourceRepo) FindByTag(ctx context.Context, tagName string, limit, offset int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) FindByComponentID(ctx context.Context, componentID string) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) CountByComponentID(ctx context.Context, componentID string) (int64, error) {
	return 0, nil
}
func (s *stubResourceRepo) FindMissedHeartbeats(ctx context.Context, now time.Time, limit int) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) UpdateLastPingAt(ctx context.Context, id string, at time.Time) error {
	return nil
}
func (s *stubResourceRepo) UpdateStatus(ctx context.Context, id string, status domain.ResourceStatus) error {
	return nil
}
func (s *stubResourceRepo) UpdateMonitoringState(ctx context.Context, id string, req port.UpdateMonitoringStateRequest) error {
	return nil
}
func (s *stubResourceRepo) UpdateMetadata(ctx context.Context, id string, req port.UpdateMetadataRequest) error {
	return nil
}
func (s *stubResourceRepo) FindScheduledResources(ctx context.Context) ([]*domain.Resource, error) {
	return nil, nil
}
func (s *stubResourceRepo) ListResourcesByFilter(ctx context.Context, f dynquery.MonitorFilter, page, perPage int) ([]*domain.Resource, int, error) {
	return nil, 0, nil
}

type stubIncidentRepo struct {
	countByResourceID      func(ctx context.Context, resourceID string) (int64, error)
	findActiveByResourceID func(ctx context.Context, resourceID string) (*domain.Incident, error)
}

func (s *stubIncidentRepo) CountByResourceID(ctx context.Context, resourceID string) (int64, error) {
	return s.countByResourceID(ctx, resourceID)
}
func (s *stubIncidentRepo) FindActiveByResourceID(ctx context.Context, resourceID string) (*domain.Incident, error) {
	return s.findActiveByResourceID(ctx, resourceID)
}
func (s *stubIncidentRepo) Create(ctx context.Context, i *domain.Incident) (*domain.Incident, error) {
	return nil, nil
}
func (s *stubIncidentRepo) FindByID(ctx context.Context, id string) (*domain.Incident, error) {
	return nil, nil
}
func (s *stubIncidentRepo) List(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	return nil, nil
}
func (s *stubIncidentRepo) Update(ctx context.Context, i *domain.Incident) error { return nil }
func (s *stubIncidentRepo) Delete(ctx context.Context, id string) error          { return nil }
func (s *stubIncidentRepo) FindUnresolved(ctx context.Context, limit, offset int) ([]*domain.Incident, error) {
	return nil, nil
}
func (s *stubIncidentRepo) FindByResource(ctx context.Context, resourceID string, limit, offset int) ([]*domain.Incident, error) {
	return nil, nil
}
func (s *stubIncidentRepo) GetIncidentStats(ctx context.Context, hours int) (int, int, error) {
	return 0, 0, nil
}
func (s *stubIncidentRepo) HasActiveIncident(ctx context.Context) (bool, error) { return false, nil }
func (s *stubIncidentRepo) FindLastResolved(ctx context.Context) (*domain.Incident, error) {
	return nil, repository.ErrNotFound
}
func (s *stubIncidentRepo) ListIncidentsByFilter(ctx context.Context, f dynquery.IncidentFilter, page, perPage int) ([]*domain.Incident, int, error) {
	return nil, 0, nil
}

type stubActivityRepo struct {
	getUptimeByWindow func(ctx context.Context, resourceID string, hours int) (*float64, error)
}

func (s *stubActivityRepo) GetUptimeByWindow(ctx context.Context, resourceID string, hours int) (*float64, error) {
	return s.getUptimeByWindow(ctx, resourceID, hours)
}
func (s *stubActivityRepo) Create(ctx context.Context, activity *domain.MonitoringActivity) error {
	return nil
}
func (s *stubActivityRepo) List(ctx context.Context, limit, offset int) ([]*domain.MonitoringActivity, error) {
	return nil, nil
}
func (s *stubActivityRepo) FindByResourceID(ctx context.Context, resourceID string, limit, offset int) ([]*domain.MonitoringActivity, error) {
	return nil, nil
}
func (s *stubActivityRepo) CountTransitionsInWindow(ctx context.Context, resourceID string, windowStart time.Time) (int, error) {
	return 0, nil
}
func (s *stubActivityRepo) GetUptimeStats(ctx context.Context, resourceID string) ([]domain.UptimeStat, error) {
	return nil, nil
}
func (s *stubActivityRepo) GetRecentResponseTimes(ctx context.Context, resourceID string, limit int) ([]domain.ResponseTimePoint, error) {
	return nil, nil
}
func (s *stubActivityRepo) GetGlobalUptimeStats(ctx context.Context, hours int) (float64, error) {
	return 0, nil
}
func (s *stubActivityRepo) GetAvgResponseTimeByWindow(ctx context.Context, resourceID string, hours int) (*int, error) {
	return nil, nil
}

// ---- helpers ----

func ptrFloat(v float64) *float64 { return &v }

func gatherMetrics(t *testing.T, c *OgouneCollector) map[string]*dto.MetricFamily {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	gathered, err := reg.Gather()
	require.NoError(t, err)
	result := make(map[string]*dto.MetricFamily)
	for _, mf := range gathered {
		result[mf.GetName()] = mf
	}
	return result
}

// ---- T019: comprehensive collector test ----

func TestOgouneCollector_Collect_AllMetricFamilies(t *testing.T) {
	resource := &domain.Resource{
		Base:   domain.Base{ID: "res-1"},
		Name:   "api-prod",
		Type:   domain.ResourceHTTP,
		Status: domain.StatusUp,
	}

	rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
		if offset == 0 {
			return []*domain.Resource{resource}, nil
		}
		return nil, nil
	}}
	ir := &stubIncidentRepo{
		countByResourceID:      func(ctx context.Context, resourceID string) (int64, error) { return 5, nil },
		findActiveByResourceID: func(ctx context.Context, resourceID string) (*domain.Incident, error) { return &domain.Incident{}, nil },
	}
	ar := &stubActivityRepo{getUptimeByWindow: func(ctx context.Context, resourceID string, hours int) (*float64, error) {
		return ptrFloat(0.998), nil
	}}

	c := NewOgouneCollector(rr, ir, ar)
	families := gatherMetrics(t, c)

	assert.Contains(t, families, "ogoune_resource_up")
	assert.Contains(t, families, "ogoune_resource_status")
	assert.Contains(t, families, "ogoune_incidents_total")
	assert.Contains(t, families, "ogoune_incidents_active")
	assert.Contains(t, families, "ogoune_uptime_ratio")
	// collector does NOT own check_duration or checks_total
	assert.NotContains(t, families, "ogoune_check_duration_seconds")
	assert.NotContains(t, families, "ogoune_checks_total")

	// resource_up=1 for StatusUp
	assertGaugeValue(t, families["ogoune_resource_up"], 1.0)

	// resource_status=1 for StatusUp
	assertGaugeValue(t, families["ogoune_resource_status"], 1.0)

	// incidents_total=5
	assertGaugeValue(t, families["ogoune_incidents_total"], 5.0)

	// incidents_active=1 (active incident exists)
	assertGaugeValue(t, families["ogoune_incidents_active"], 1.0)

	// uptime_ratio: 3 series for 3 windows
	require.Len(t, families["ogoune_uptime_ratio"].GetMetric(), 3)
	windows := map[string]bool{}
	for _, m := range families["ogoune_uptime_ratio"].GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "window" {
				windows[lp.GetValue()] = true
			}
		}
	}
	assert.True(t, windows["24h"], "window=24h must be present")
	assert.True(t, windows["7d"], "window=7d must be present")
	assert.True(t, windows["30d"], "window=30d must be present")

	// labels id, name, type on resource_up
	m := families["ogoune_resource_up"].GetMetric()[0]
	labelMap := labelPairs(m)
	assert.Equal(t, "res-1", labelMap["id"])
	assert.Equal(t, "api-prod", labelMap["name"])
	assert.Equal(t, "http", labelMap["type"])
}

func TestOgouneCollector_ResourceUp_NonUpStatuses(t *testing.T) {
	for _, status := range []domain.ResourceStatus{
		domain.StatusDown, domain.StatusPaused, domain.StatusError, domain.StatusPending,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			resource := &domain.Resource{Base: domain.Base{ID: "r"}, Name: "x", Type: domain.ResourceHTTP, Status: status}
			rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
				if offset == 0 {
					return []*domain.Resource{resource}, nil
				}
				return nil, nil
			}}
			ir := &stubIncidentRepo{
				countByResourceID:      func(_ context.Context, _ string) (int64, error) { return 0, nil },
				findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
			}
			ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return nil, nil }}
			c := NewOgouneCollector(rr, ir, ar)
			families := gatherMetrics(t, c)
			assertGaugeValue(t, families["ogoune_resource_up"], 0.0)
		})
	}
}

func TestOgouneCollector_ResourceStatus_FullMapping(t *testing.T) {
	cases := []struct {
		status domain.ResourceStatus
		want   float64
	}{
		{domain.StatusUp, 1},
		{domain.StatusDown, 2},
		{domain.StatusError, 2},
		{domain.StatusWarn, 2},
		{domain.StatusFlapping, 2},
		{domain.StatusPaused, 3},
		{domain.StatusUnknown, 0},
		{domain.StatusPending, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			resource := &domain.Resource{Base: domain.Base{ID: "r"}, Name: "x", Type: domain.ResourceHTTP, Status: tc.status}
			rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
				if offset == 0 {
					return []*domain.Resource{resource}, nil
				}
				return nil, nil
			}}
			ir := &stubIncidentRepo{
				countByResourceID:      func(_ context.Context, _ string) (int64, error) { return 0, nil },
				findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
			}
			ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return nil, nil }}
			c := NewOgouneCollector(rr, ir, ar)
			families := gatherMetrics(t, c)
			assertGaugeValue(t, families["ogoune_resource_status"], tc.want)
		})
	}
}

func TestOgouneCollector_IncidentsActive_ErrNotFound(t *testing.T) {
	resource := &domain.Resource{Base: domain.Base{ID: "r"}, Name: "x", Type: domain.ResourceHTTP, Status: domain.StatusUp}
	rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
		if offset == 0 {
			return []*domain.Resource{resource}, nil
		}
		return nil, nil
	}}
	ir := &stubIncidentRepo{
		countByResourceID:      func(_ context.Context, _ string) (int64, error) { return 0, nil },
		findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
	}
	ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return nil, nil }}
	c := NewOgouneCollector(rr, ir, ar)
	families := gatherMetrics(t, c)
	assertGaugeValue(t, families["ogoune_incidents_active"], 0.0)
}

func TestOgouneCollector_UptimeRatio_NilReturnsZero(t *testing.T) {
	resource := &domain.Resource{Base: domain.Base{ID: "r"}, Name: "x", Type: domain.ResourceHTTP, Status: domain.StatusUp}
	rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
		if offset == 0 {
			return []*domain.Resource{resource}, nil
		}
		return nil, nil
	}}
	ir := &stubIncidentRepo{
		countByResourceID:      func(_ context.Context, _ string) (int64, error) { return 0, nil },
		findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
	}
	ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return nil, nil }}
	c := NewOgouneCollector(rr, ir, ar)
	families := gatherMetrics(t, c)
	for _, m := range families["ogoune_uptime_ratio"].GetMetric() {
		assert.EqualValues(t, 0.0, m.GetGauge().GetValue(), "nil uptime ratio must emit 0.0")
	}
}

// T020: zero-resources — no ogoune_* metrics emitted.
func TestOgouneCollector_ZeroResources(t *testing.T) {
	rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
		return nil, nil
	}}
	ir := &stubIncidentRepo{
		countByResourceID:      func(_ context.Context, _ string) (int64, error) { return 0, nil },
		findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
	}
	ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return nil, nil }}
	c := NewOgouneCollector(rr, ir, ar)
	families := gatherMetrics(t, c)
	assert.Empty(t, families, "no metrics should be emitted for zero resources")
}

// T021: per-resource error resilience — failing resource is skipped, others still emitted.
func TestOgouneCollector_PartialErrorResilience(t *testing.T) {
	res1 := &domain.Resource{Base: domain.Base{ID: "res-1"}, Name: "good", Type: domain.ResourceHTTP, Status: domain.StatusUp}
	res2 := &domain.Resource{Base: domain.Base{ID: "res-2"}, Name: "bad", Type: domain.ResourceHTTP, Status: domain.StatusUp}

	rr := &stubResourceRepo{findActive: func(ctx context.Context, limit, offset int) ([]*domain.Resource, error) {
		if offset == 0 {
			return []*domain.Resource{res1, res2}, nil
		}
		return nil, nil
	}}
	ir := &stubIncidentRepo{
		countByResourceID: func(_ context.Context, resourceID string) (int64, error) {
			if resourceID == "res-2" {
				return 0, fmt.Errorf("db error for res-2")
			}
			return 3, nil
		},
		findActiveByResourceID: func(_ context.Context, _ string) (*domain.Incident, error) { return nil, repository.ErrNotFound },
	}
	ar := &stubActivityRepo{getUptimeByWindow: func(_ context.Context, _ string, _ int) (*float64, error) { return ptrFloat(1.0), nil }}

	c := NewOgouneCollector(rr, ir, ar)
	families := gatherMetrics(t, c)

	// res-1 must have incidents_total (CountByResourceID succeeded)
	totalIDs := idsFromFamily(families["ogoune_incidents_total"])
	assert.True(t, totalIDs["res-1"], "res-1 must have ogoune_incidents_total")
	// res-2 is skipped after CountByResourceID error — no incidents_total for it
	assert.False(t, totalIDs["res-2"], "res-2 must not have ogoune_incidents_total after error")
}

// ---- assert helpers ----

func assertGaugeValue(t *testing.T, mf *dto.MetricFamily, want float64) {
	t.Helper()
	require.NotNil(t, mf)
	require.NotEmpty(t, mf.GetMetric())
	assert.EqualValues(t, want, mf.GetMetric()[0].GetGauge().GetValue())
}

func idsFromFamily(mf *dto.MetricFamily) map[string]bool {
	result := make(map[string]bool)
	if mf == nil {
		return result
	}
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "id" {
				result[lp.GetValue()] = true
			}
		}
	}
	return result
}

func labelPairs(m *dto.Metric) map[string]string {
	result := make(map[string]string)
	for _, lp := range m.GetLabel() {
		result[lp.GetName()] = lp.GetValue()
	}
	return result
}
