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

func TestNotificationEscalationStateRepository_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		repo := store.NewNotificationEscalationStateRepositorySQLC(fx.Runtime)
		ctx := context.Background()

		t.Run("get_absent_is_ErrNotFound", func(t *testing.T) {
			_, err := repo.Get(ctx)
			require.ErrorIs(t, err, repository.ErrNotFound)
		})

		wm := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
		dg := time.Date(2026, 8, 2, 11, 5, 0, 0, time.UTC)

		t.Run("upsert_insert_then_get", func(t *testing.T) {
			require.NoError(t, repo.Upsert(ctx, &domain.NotificationEscalationState{
				WatermarkOccurredAt: &wm, LastDigestAt: &dg,
			}))
			got, err := repo.Get(ctx)
			require.NoError(t, err)
			require.NotNil(t, got.WatermarkOccurredAt)
			assert.True(t, got.WatermarkOccurredAt.Equal(wm))
			require.NotNil(t, got.LastDigestAt)
			assert.True(t, got.LastDigestAt.Equal(dg))
		})

		t.Run("upsert_updates_single_row", func(t *testing.T) {
			wm2 := wm.Add(time.Hour)
			require.NoError(t, repo.Upsert(ctx, &domain.NotificationEscalationState{
				WatermarkOccurredAt: &wm2, LastDigestAt: &dg,
			}))
			got, err := repo.Get(ctx)
			require.NoError(t, err)
			assert.True(t, got.WatermarkOccurredAt.Equal(wm2))
		})
	})
}

func TestFeed_UnreadForEscalation_Contract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		feed := store.NewNotificationFeedRepositorySQLC(fx.Runtime)
		ctx := context.Background()
		now := time.Now().UTC()
		old := now.Add(-1 * time.Hour) // older than a 30m cutoff
		recent := now.Add(-5 * time.Minute)

		// Real user for the targeted-notification FK (notifications.user_id -> users.id).
		u, err := store.NewUserRepositorySQLC(fx.Runtime).Create(ctx, &domain.User{
			Email: "ops@example.com", Name: "Ops", HashedPassword: "x",
		})
		require.NoError(t, err)
		user := u.ID

		mk := func(cat, sev string, occurred time.Time, read *time.Time, userID *string) {
			_, err := feed.Create(ctx, &domain.FeedNotification{
				UserID: userID, Category: cat, Severity: sev,
				Title: cat + " event", OccurredAt: occurred, ReadAt: read,
			})
			require.NoError(t, err)
		}
		readAt := now.Add(-10 * time.Minute)

		mk(domain.NotificationCategoryIncident, domain.NotificationSeverityError, old, nil, nil)   // qualifies
		mk(domain.NotificationCategoryHost, domain.NotificationSeverityError, old, nil, nil)       // qualifies
		mk(domain.NotificationCategoryGeneral, domain.NotificationSeverityInfo, old, nil, nil)     // excluded: not actionable
		mk(domain.NotificationCategoryIncident, domain.NotificationSeverityError, old, &readAt, nil) // excluded: read
		mk(domain.NotificationCategoryIncident, domain.NotificationSeverityError, recent, nil, nil) // excluded: too recent
		mk(domain.NotificationCategoryIncident, domain.NotificationSeverityError, old, nil, &user)   // excluded: user-targeted

		cutoff := now.Add(-30 * time.Minute)
		items, err := feed.ListUnreadForEscalation(ctx, domain.EscalationCategories, cutoff, 10)
		require.NoError(t, err)
		assert.Len(t, items, 2, "only actionable, unread, instance-wide, older-than-cutoff qualify")

		count, err := feed.CountUnreadForEscalation(ctx, domain.EscalationCategories, cutoff)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}
