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

func TestHostCredentialRepository_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		repo := store.NewHostCredentialRepositorySQLC(fx.Runtime)
		runHostCredentialContract(t, repo)
	})
}

func runHostCredentialContract(t *testing.T, repo port.HostCredentialRepository) {
	t.Helper()
	ctx := context.Background()

	t.Run("Create_FindActiveByHash", func(t *testing.T) {
		c := &domain.HostCredential{
			HostID:   "host-active",
			Hash:     "hash-active",
			Prefix:   "ag_live_act",
			IsActive: true,
		}
		require.NoError(t, repo.Create(ctx, c))
		assert.NotEmpty(t, c.ID)

		found, err := repo.FindActiveByHash(ctx, "hash-active")
		require.NoError(t, err)
		assert.Equal(t, c.ID, found.ID)
		assert.Equal(t, "host-active", found.HostID)
		assert.Equal(t, "ag_live_act", found.Prefix)
		assert.True(t, found.IsActive)
	})

	t.Run("FindActiveByHash_NotFound", func(t *testing.T) {
		_, err := repo.FindActiveByHash(ctx, "no-such-hash")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("DeactivateByID_HashNoLongerActive", func(t *testing.T) {
		c := &domain.HostCredential{
			HostID:   "host-deact",
			Hash:     "hash-deact",
			Prefix:   "ag_live_dea",
			IsActive: true,
		}
		require.NoError(t, repo.Create(ctx, c))

		require.NoError(t, repo.DeactivateByID(ctx, c.ID))

		_, err := repo.FindActiveByHash(ctx, "hash-deact")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("DeactivateByID_NotFound", func(t *testing.T) {
		assert.ErrorIs(t, repo.DeactivateByID(ctx, "nonexistent"), repository.ErrNotFound)
	})

	t.Run("DeactivateAllForHost", func(t *testing.T) {
		hostID := "host-deactall"
		for i := 0; i < 3; i++ {
			c := &domain.HostCredential{
				HostID:   hostID,
				Hash:     "hash-deactall-" + string(rune('a'+i)),
				Prefix:   "ag_live_all",
				IsActive: true,
			}
			require.NoError(t, repo.Create(ctx, c))
		}

		require.NoError(t, repo.DeactivateAllForHost(ctx, hostID))

		creds, err := repo.ListByHost(ctx, hostID)
		require.NoError(t, err)
		require.Len(t, creds, 3)
		for _, c := range creds {
			assert.False(t, c.IsActive, "credential %s should be inactive", c.ID)
		}
	})

	t.Run("ListByHost", func(t *testing.T) {
		hostID := "host-list"
		for i := 0; i < 2; i++ {
			c := &domain.HostCredential{
				HostID:   hostID,
				Hash:     "hash-list-" + string(rune('a'+i)),
				Prefix:   "ag_live_lst",
				IsActive: true,
			}
			require.NoError(t, repo.Create(ctx, c))
		}
		creds, err := repo.ListByHost(ctx, hostID)
		require.NoError(t, err)
		assert.Len(t, creds, 2)
	})

	t.Run("TouchLastUsed", func(t *testing.T) {
		c := &domain.HostCredential{
			HostID:   "host-touch",
			Hash:     "hash-touch",
			Prefix:   "ag_live_tch",
			IsActive: true,
		}
		require.NoError(t, repo.Create(ctx, c))
		assert.Nil(t, c.LastUsedAt)

		usedAt := time.Now().UTC().Truncate(time.Second)
		require.NoError(t, repo.TouchLastUsed(ctx, c.ID, usedAt))

		creds, err := repo.ListByHost(ctx, "host-touch")
		require.NoError(t, err)
		require.Len(t, creds, 1)
		require.NotNil(t, creds[0].LastUsedAt)
		assert.WithinDuration(t, usedAt, creds[0].LastUsedAt.UTC(), time.Second)
	})

	t.Run("DeleteByHost", func(t *testing.T) {
		hostID := "host-delete"
		for i := 0; i < 2; i++ {
			c := &domain.HostCredential{
				HostID:   hostID,
				Hash:     "hash-delete-" + string(rune('a'+i)),
				Prefix:   "ag_live_del",
				IsActive: true,
			}
			require.NoError(t, repo.Create(ctx, c))
		}

		require.NoError(t, repo.DeleteByHost(ctx, hostID))

		creds, err := repo.ListByHost(ctx, hostID)
		require.NoError(t, err)
		assert.Empty(t, creds)
	})
}
