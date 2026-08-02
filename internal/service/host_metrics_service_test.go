package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHostMetricsFixture builds a metrics service backed by fakes and creates a
// single host, returning the service, the host repo, and the host ID.
func newHostMetricsFixture(t *testing.T) (*HostMetricsService, *fake.HostFake, string) {
	t.Helper()
	ctx := context.Background()
	hosts := fake.NewHostFake()
	host := &domain.Host{Name: "h"}
	require.NoError(t, hosts.Create(ctx, host))

	svc := NewHostMetricsService(fake.NewHostMetricFake(), hosts, 48*time.Hour, 7*24*time.Hour)
	return svc, hosts, host.ID
}

func TestHostMetricsService_Ingest_StoresSampleAndSnapshot(t *testing.T) {
	ctx := context.Background()
	svc, hosts, hostID := newHostMetricsFixture(t)

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return now })

	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{
		OS:           strPtr("linux"),
		AgentVersion: strPtr("1.2.3"),
		CPUPct:       42,
		MemPct:       55,
		NetIn:        100,
		NetOut:       200,
		Disks: []domain.DiskUsage{
			{Mount: "/", UsedPct: 60},
			{Mount: "/data", UsedPct: 80},
		},
	}))

	// Sample stored — verify via History.
	samples, err := svc.History(ctx, hostID, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, samples, 1)
	assert.Equal(t, hostID, samples[0].HostID)
	assert.Equal(t, 42.0, samples[0].CPUPct)
	assert.Equal(t, 55.0, samples[0].MemPct)
	assert.Equal(t, now, samples[0].SampledAt)

	// Snapshot updated on the host row.
	host, err := hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	require.NotNil(t, host.LastSeenAt)
	assert.Equal(t, now, *host.LastSeenAt)
	require.NotNil(t, host.LastCPUPct)
	assert.Equal(t, 42.0, *host.LastCPUPct)
	require.NotNil(t, host.LastMemPct)
	assert.Equal(t, 55.0, *host.LastMemPct)
	require.NotNil(t, host.LastDiskPct)
	assert.Equal(t, 80.0, *host.LastDiskPct, "LastDiskPct must be the worst (max) disk used_pct")
	require.NotNil(t, host.OS)
	assert.Equal(t, "linux", *host.OS)
	require.NotNil(t, host.AgentVersion)
	assert.Equal(t, "1.2.3", *host.AgentVersion)
}

func TestHostMetricsService_OnlineOffline(t *testing.T) {
	ctx := context.Background()
	svc, hosts, hostID := newHostMetricsFixture(t)

	const threshold = 45 * time.Second
	base := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	// Fresh ingest at base.
	svc.SetNow(func() time.Time { return base })
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{CPUPct: 10, MemPct: 10}))

	host, err := hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	assert.True(t, host.IsOnline(base, threshold), "host should be online right after a fresh ingest")

	// Advance now beyond the threshold → offline.
	later := base.Add(threshold + time.Second)
	assert.False(t, host.IsOnline(later, threshold), "host should be offline once now advances past the threshold")

	// A new ingest at `later` flips it back to online.
	svc.SetNow(func() time.Time { return later })
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{CPUPct: 20, MemPct: 20}))

	host, err = hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	assert.True(t, host.IsOnline(later, threshold), "host should be online again after a fresh ingest")
}

func TestHostMetricsService_Ingest_ClampsPercentages(t *testing.T) {
	ctx := context.Background()
	svc, hosts, hostID := newHostMetricsFixture(t)

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return now })

	// Over-range clamps to 100.
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{
		CPUPct: 150,
		MemPct: 200,
		Disks:  []domain.DiskUsage{{Mount: "/", UsedPct: 130}},
	}))

	host, err := hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	require.NotNil(t, host.LastCPUPct)
	assert.Equal(t, 100.0, *host.LastCPUPct)
	require.NotNil(t, host.LastMemPct)
	assert.Equal(t, 100.0, *host.LastMemPct)
	require.NotNil(t, host.LastDiskPct)
	assert.Equal(t, 100.0, *host.LastDiskPct)

	// Under-range clamps to 0.
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{CPUPct: -5, MemPct: -1}))
	host, err = hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	require.NotNil(t, host.LastCPUPct)
	assert.Equal(t, 0.0, *host.LastCPUPct)

	// And it's clamped in the stored sample too.
	samples, err := svc.History(ctx, hostID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, samples, 2)
	assert.Equal(t, 100.0, samples[0].CPUPct)
	assert.Equal(t, 0.0, samples[1].CPUPct)
}

func TestHostMetricsService_Ingest_RejectsMalformed(t *testing.T) {
	ctx := context.Background()
	svc, hosts, hostID := newHostMetricsFixture(t)

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return now })

	err := svc.Ingest(ctx, hostID, IngestSample{CPUPct: math.NaN()})
	assert.ErrorIs(t, err, ErrValidationFailed)

	// last_seen_at must NOT have been advanced — this was the first call, so nil.
	host, err := hosts.FindByID(ctx, hostID)
	require.NoError(t, err)
	assert.Nil(t, host.LastSeenAt, "a rejected sample must not advance last_seen_at")

	// No sample should have been stored either.
	samples, err := svc.History(ctx, hostID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, samples)
}

func TestHostMetricsService_History_DefaultsToLastHour(t *testing.T) {
	ctx := context.Background()
	svc, _, hostID := newHostMetricsFixture(t)

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	// One sample inside the last hour, one well outside it.
	svc.SetNow(func() time.Time { return now.Add(-30 * time.Minute) })
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{CPUPct: 10, MemPct: 10}))

	svc.SetNow(func() time.Time { return now.Add(-3 * time.Hour) })
	require.NoError(t, svc.Ingest(ctx, hostID, IngestSample{CPUPct: 20, MemPct: 20}))

	// Now anchor the clock at `now`; zero from/to → last hour only.
	svc.SetNow(func() time.Time { return now })
	samples, err := svc.History(ctx, hostID, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, samples, 1, "zero from/to should default to the last hour")
	assert.Equal(t, 10.0, samples[0].CPUPct)
}
