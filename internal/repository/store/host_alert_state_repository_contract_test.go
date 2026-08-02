package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/store"
)

func TestHostAlertStateRepository_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		hosts := store.NewHostRepositorySQLC(fx.Runtime)
		repo := store.NewHostAlertStateRepositorySQLC(fx.Runtime)
		ctx := context.Background()

		// Parent host (FK host_alert_state.host_id -> hosts.id).
		h := &domain.Host{Name: "web-1"}
		require.NoError(t, hosts.Create(ctx, h))
		require.NotEmpty(t, h.ID)

		t.Run("get_absent_is_ErrNotFound", func(t *testing.T) {
			_, err := repo.Get(ctx, h.ID)
			require.ErrorIs(t, err, repository.ErrNotFound)
		})

		offlineSince := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

		t.Run("upsert_insert_then_get", func(t *testing.T) {
			err := repo.Upsert(ctx, &domain.HostAlertState{
				HostID:       h.ID,
				State:        domain.HostAlertStateOffline,
				OfflineSince: &offlineSince,
				Alerted:      false,
			})
			require.NoError(t, err)

			got, err := repo.Get(ctx, h.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.HostAlertStateOffline, got.State)
			assert.False(t, got.Alerted)
			require.NotNil(t, got.OfflineSince)
			assert.True(t, got.OfflineSince.Equal(offlineSince))
		})

		t.Run("upsert_update_same_row", func(t *testing.T) {
			err := repo.Upsert(ctx, &domain.HostAlertState{
				HostID:       h.ID,
				State:        domain.HostAlertStateOffline,
				OfflineSince: &offlineSince,
				Alerted:      true, // now alerted
			})
			require.NoError(t, err)

			got, err := repo.Get(ctx, h.ID)
			require.NoError(t, err)
			assert.True(t, got.Alerted)
		})

		t.Run("upsert_online_clears_offline_since", func(t *testing.T) {
			err := repo.Upsert(ctx, &domain.HostAlertState{
				HostID:       h.ID,
				State:        domain.HostAlertStateOnline,
				OfflineSince: nil,
				Alerted:      false,
			})
			require.NoError(t, err)

			got, err := repo.Get(ctx, h.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.HostAlertStateOnline, got.State)
			assert.Nil(t, got.OfflineSince)
			assert.False(t, got.Alerted)
		})
	})
}
