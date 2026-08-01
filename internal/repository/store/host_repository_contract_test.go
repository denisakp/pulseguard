package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/store"
)

// ---- local pointer helpers ----

func strptr(s string) *string    { return &s }
func f64ptr(f float64) *float64  { return &f }
func i64ptr(i int64) *int64      { return &i }
func timeptr(t time.Time) *time.Time { return &t }

func TestHostRepository_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		repo := store.NewHostRepositorySQLC(fx.Runtime)
		runHostContract(t, repo)
	})
}

func runHostContract(t *testing.T, repo port.HostRepository) {
	t.Helper()
	ctx := context.Background()

	t.Run("Create_Find_Roundtrip", func(t *testing.T) {
		seen := time.Now().UTC().Truncate(time.Second)
		h := &domain.Host{
			Name:         "web-01",
			OS:           strptr("linux"),
			AgentVersion: strptr("1.2.3"),
			LastSeenAt:   timeptr(seen),
			LastCPUPct:   f64ptr(12.5),
			LastMemPct:   f64ptr(48.0),
			LastDiskPct:  f64ptr(70.25),
			LastNetIn:    i64ptr(1024),
			LastNetOut:   i64ptr(2048),
			LastDisks: []domain.DiskUsage{
				{Mount: "/", UsedPct: 70.25},
				{Mount: "/data", UsedPct: 33.3},
			},
		}
		require.NoError(t, repo.Create(ctx, h))
		assert.NotEmpty(t, h.ID)

		found, err := repo.FindByID(ctx, h.ID)
		require.NoError(t, err)
		assert.Equal(t, h.ID, found.ID)
		assert.Equal(t, "web-01", found.Name)
		require.NotNil(t, found.OS)
		assert.Equal(t, "linux", *found.OS)
		require.NotNil(t, found.AgentVersion)
		assert.Equal(t, "1.2.3", *found.AgentVersion)
		require.NotNil(t, found.LastSeenAt)
		assert.WithinDuration(t, seen, found.LastSeenAt.UTC(), time.Second)
		require.NotNil(t, found.LastCPUPct)
		assert.InDelta(t, 12.5, *found.LastCPUPct, 0.001)
		require.NotNil(t, found.LastMemPct)
		assert.InDelta(t, 48.0, *found.LastMemPct, 0.001)
		require.NotNil(t, found.LastDiskPct)
		assert.InDelta(t, 70.25, *found.LastDiskPct, 0.001)
		require.NotNil(t, found.LastNetIn)
		assert.EqualValues(t, 1024, *found.LastNetIn)
		require.NotNil(t, found.LastNetOut)
		assert.EqualValues(t, 2048, *found.LastNetOut)
		require.Len(t, found.LastDisks, 2)
		assert.Equal(t, "/", found.LastDisks[0].Mount)
		assert.InDelta(t, 70.25, found.LastDisks[0].UsedPct, 0.001)
		assert.Equal(t, "/data", found.LastDisks[1].Mount)
		assert.InDelta(t, 33.3, found.LastDisks[1].UsedPct, 0.001)
	})

	t.Run("Create_NullableFields", func(t *testing.T) {
		h := &domain.Host{Name: "minimal"}
		require.NoError(t, repo.Create(ctx, h))

		found, err := repo.FindByID(ctx, h.ID)
		require.NoError(t, err)
		assert.Equal(t, "minimal", found.Name)
		assert.Nil(t, found.OS)
		assert.Nil(t, found.AgentVersion)
		assert.Nil(t, found.LastSeenAt)
		assert.Nil(t, found.LastCPUPct)
		assert.Nil(t, found.LastMemPct)
		assert.Nil(t, found.LastDiskPct)
		assert.Nil(t, found.LastNetIn)
		assert.Nil(t, found.LastNetOut)
		assert.Empty(t, found.LastDisks)
	})

	t.Run("FindByID_NotFound", func(t *testing.T) {
		_, err := repo.FindByID(ctx, "nonexistent")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("List_OrderedByCreatedAtDESC_AndCount", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Second)
		oldest := &domain.Host{Name: "order-oldest", Base: domain.Base{CreatedAt: base.Add(-3 * time.Hour)}}
		middle := &domain.Host{Name: "order-middle", Base: domain.Base{CreatedAt: base.Add(-2 * time.Hour)}}
		newest := &domain.Host{Name: "order-newest", Base: domain.Base{CreatedAt: base.Add(-1 * time.Hour)}}
		require.NoError(t, repo.Create(ctx, oldest))
		require.NoError(t, repo.Create(ctx, middle))
		require.NoError(t, repo.Create(ctx, newest))

		hosts, err := repo.List(ctx, 100, 0)
		require.NoError(t, err)

		// Filter to the three we created and assert relative ordering (DESC).
		var order []string
		want := map[string]bool{oldest.ID: true, middle.ID: true, newest.ID: true}
		for _, h := range hosts {
			if want[h.ID] {
				order = append(order, h.ID)
			}
		}
		require.Equal(t, []string{newest.ID, middle.ID, oldest.ID}, order)

		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(3))
	})

	t.Run("Delete_Success", func(t *testing.T) {
		h := &domain.Host{Name: "to-delete"}
		require.NoError(t, repo.Create(ctx, h))
		require.NoError(t, repo.Delete(ctx, h.ID))
		_, err := repo.FindByID(ctx, h.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		assert.ErrorIs(t, repo.Delete(ctx, "nonexistent"), repository.ErrNotFound)
	})

	t.Run("UpdateSnapshot_SetsValues", func(t *testing.T) {
		h := &domain.Host{Name: "snap"}
		require.NoError(t, repo.Create(ctx, h))

		seen := time.Now().UTC().Truncate(time.Second)
		h.LastSeenAt = timeptr(seen)
		h.LastCPUPct = f64ptr(55.5)
		h.LastMemPct = f64ptr(66.6)
		h.LastDiskPct = f64ptr(77.7)
		h.LastNetIn = i64ptr(4096)
		h.LastNetOut = i64ptr(8192)
		h.LastDisks = []domain.DiskUsage{{Mount: "/", UsedPct: 77.7}}
		h.OS = strptr("darwin")
		h.AgentVersion = strptr("2.0.0")
		require.NoError(t, repo.UpdateSnapshot(ctx, h))

		found, err := repo.FindByID(ctx, h.ID)
		require.NoError(t, err)
		require.NotNil(t, found.LastSeenAt)
		assert.WithinDuration(t, seen, found.LastSeenAt.UTC(), time.Second)
		require.NotNil(t, found.LastCPUPct)
		assert.InDelta(t, 55.5, *found.LastCPUPct, 0.001)
		require.NotNil(t, found.LastNetIn)
		assert.EqualValues(t, 4096, *found.LastNetIn)
		require.Len(t, found.LastDisks, 1)
		assert.Equal(t, "/", found.LastDisks[0].Mount)
		require.NotNil(t, found.OS)
		assert.Equal(t, "darwin", *found.OS)
		require.NotNil(t, found.AgentVersion)
		assert.Equal(t, "2.0.0", *found.AgentVersion)
	})

	t.Run("UpdateSnapshot_NilOSPreservesExisting", func(t *testing.T) {
		h := &domain.Host{
			Name:         "preserve",
			OS:           strptr("linux"),
			AgentVersion: strptr("1.0.0"),
		}
		require.NoError(t, repo.Create(ctx, h))

		// Update metrics but pass nil OS / AgentVersion — COALESCE keeps old.
		h.OS = nil
		h.AgentVersion = nil
		h.LastCPUPct = f64ptr(99.9)
		h.LastSeenAt = timeptr(time.Now().UTC().Truncate(time.Second))
		require.NoError(t, repo.UpdateSnapshot(ctx, h))

		found, err := repo.FindByID(ctx, h.ID)
		require.NoError(t, err)
		require.NotNil(t, found.OS)
		assert.Equal(t, "linux", *found.OS)
		require.NotNil(t, found.AgentVersion)
		assert.Equal(t, "1.0.0", *found.AgentVersion)
		require.NotNil(t, found.LastCPUPct)
		assert.InDelta(t, 99.9, *found.LastCPUPct, 0.001)
	})
}
