package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/store"
)

func TestHostMetricRepository_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		repo := store.NewHostMetricRepositorySQLC(fx.Runtime)
		runHostMetricContract(t, repo)
	})
}

func newSample(hostID string, at time.Time) *domain.HostMetricSample {
	return &domain.HostMetricSample{
		HostID:    hostID,
		SampledAt: at,
		CPUPct:    10.0,
		MemPct:    20.0,
		NetIn:     100,
		NetOut:    200,
		Disks:     []domain.DiskUsage{{Mount: "/", UsedPct: 42.0}},
	}
}

func runHostMetricContract(t *testing.T, repo port.HostMetricsRepository) {
	t.Helper()
	ctx := context.Background()

	t.Run("Insert_ListInRange_Chronological", func(t *testing.T) {
		hostID := "host-range"
		now := time.Now().UTC().Truncate(time.Second)
		t1 := now.Add(-30 * time.Minute)
		t2 := now.Add(-20 * time.Minute)
		t3 := now.Add(-10 * time.Minute)
		outBefore := now.Add(-90 * time.Minute)
		outAfter := now.Add(10 * time.Minute)

		// Insert deliberately out of chronological order.
		for _, at := range []time.Time{t2, outBefore, t3, t1, outAfter} {
			require.NoError(t, repo.Insert(ctx, newSample(hostID, at)))
		}

		got, err := repo.ListInRange(ctx, hostID, now.Add(-40*time.Minute), now)
		require.NoError(t, err)
		require.Len(t, got, 3, "window excludes out-of-range samples")

		// Chronological ASC.
		assert.WithinDuration(t, t1, got[0].SampledAt.UTC(), time.Second)
		assert.WithinDuration(t, t2, got[1].SampledAt.UTC(), time.Second)
		assert.WithinDuration(t, t3, got[2].SampledAt.UTC(), time.Second)

		// Payload roundtrip on first row.
		assert.InDelta(t, 10.0, got[0].CPUPct, 0.001)
		assert.InDelta(t, 20.0, got[0].MemPct, 0.001)
		assert.EqualValues(t, 100, got[0].NetIn)
		assert.EqualValues(t, 200, got[0].NetOut)
		require.Len(t, got[0].Disks, 1)
		assert.Equal(t, "/", got[0].Disks[0].Mount)
		assert.InDelta(t, 42.0, got[0].Disks[0].UsedPct, 0.001)
	})

	t.Run("DeleteOlderThan_PurgesStrictlyBeforeCutoff", func(t *testing.T) {
		hostID := "host-purge"
		now := time.Now().UTC().Truncate(time.Second)
		old1 := now.Add(-3 * time.Hour)
		old2 := now.Add(-2 * time.Hour)
		recent := now.Add(-10 * time.Minute)
		require.NoError(t, repo.Insert(ctx, newSample(hostID, old1)))
		require.NoError(t, repo.Insert(ctx, newSample(hostID, old2)))
		require.NoError(t, repo.Insert(ctx, newSample(hostID, recent)))

		cutoff := now.Add(-1 * time.Hour)
		n, err := repo.DeleteOlderThan(ctx, cutoff)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(2), "at least the two old samples for this host purged")

		remaining, err := repo.ListInRange(ctx, hostID, now.Add(-4*time.Hour), now)
		require.NoError(t, err)
		require.Len(t, remaining, 1, "only the sample at/after cutoff survives")
		assert.WithinDuration(t, recent, remaining[0].SampledAt.UTC(), time.Second)
	})

	t.Run("DeleteByHost", func(t *testing.T) {
		hostID := "host-delete-metrics"
		now := time.Now().UTC().Truncate(time.Second)
		require.NoError(t, repo.Insert(ctx, newSample(hostID, now.Add(-5*time.Minute))))
		require.NoError(t, repo.Insert(ctx, newSample(hostID, now.Add(-1*time.Minute))))

		require.NoError(t, repo.DeleteByHost(ctx, hostID))

		remaining, err := repo.ListInRange(ctx, hostID, now.Add(-1*time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		assert.Empty(t, remaining)
	})

	t.Run("Decimate_ReducesDensityBeforeCutoff", func(t *testing.T) {
		hostID := "host-decimate"
		// Three samples inside the SAME minute, all older than the cutoff.
		bucket := time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
		require.NoError(t, repo.Insert(ctx, newSample(hostID, bucket.Add(10*time.Second))))
		require.NoError(t, repo.Insert(ctx, newSample(hostID, bucket.Add(20*time.Second))))
		require.NoError(t, repo.Insert(ctx, newSample(hostID, bucket.Add(30*time.Second))))
		// A recent sample AFTER the cutoff — must be left untouched.
		recent := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		require.NoError(t, repo.Insert(ctx, newSample(hostID, recent)))

		cutoff := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
		_, err := repo.Decimate(ctx, cutoff)
		require.NoError(t, err)

		got, err := repo.ListInRange(ctx, hostID,
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC))
		require.NoError(t, err)
		require.Len(t, got, 2, "one survivor per minute before cutoff + the post-cutoff sample")

		// Exactly one survivor from the pre-cutoff minute bucket (which one is
		// nondeterministic — MIN(id) over ULIDs minted in the same millisecond).
		survivor := got[0].SampledAt.UTC()
		assert.False(t, survivor.Before(bucket), "survivor within the 10:30 bucket")
		assert.True(t, survivor.Before(bucket.Add(time.Minute)), "survivor within the 10:30 bucket")
		// The post-cutoff sample is untouched.
		assert.WithinDuration(t, recent, got[1].SampledAt.UTC(), time.Second)
	})
}
