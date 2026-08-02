package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	pgsqlc "github.com/denisakp/ogoune/internal/repository/sqlc/pg"
	sqlitesqlc "github.com/denisakp/ogoune/internal/repository/sqlc/sqlite"
	"github.com/denisakp/ogoune/internal/repository/store"
	"github.com/denisakp/ogoune/pkg/crypto"
)

const roundtripTestKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// roundtripFactory wires a sqlc resource repo with a query-counting DBTX.
// Returns the repo and the counter sharing the same instance.
func roundtripFactory(t *testing.T, fx *internaltest.DialectFixture) (port.ResourceRepository, *internaltest.Counter) {
	t.Helper()
	rt := fx.Runtime
	switch fx.Dialect {
	case "postgres":
		c, dbtx := internaltest.NewPGCounter(rt.PgxPool())
		q := pgsqlc.New(dbtx)
		return store.NewResourceRepositorySQLCForTest(q, nil, rt.PgxPool(), nil), c
	case "sqlite":
		c, dbtx := internaltest.NewSQLiteCounter(rt.SQLiteDB())
		q := sqlitesqlc.New(dbtx)
		return store.NewResourceRepositorySQLCForTest(nil, q, nil, rt.SQLiteDB()), c
	default:
		t.Fatalf("unknown dialect %q", fx.Dialect)
		return nil, nil
	}
}

// TestResourceRepository_ListPreloads_RoundTripBound enforces the round-trip
// bound for Resource.List.
//
// As of spec 050 the list-view path uses attachTagsOnly: 1 principal SELECT
// + 1 Tags preload = 2 round-trips. Channels / Component / Credential
// preloads were dropped from the list path after audit confirmed no caller
// of List consumes those fields. Full preloads remain on FindByID /
// FindActive / FindByHeartbeatSlug for callers that need them (workers,
// detail view).
//
// Bound is invariant in N (controlled-N+1).
func TestResourceRepository_ListPreloads_RoundTripBound(t *testing.T) {
	t.Setenv("APP_SECRET_KEY", roundtripTestKey)
	crypto.SetGlobalProvider(&crypto.EnvKeyProvider{})

	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		ctx := context.Background()

		// Use the sqlc repo (without the counter) to seed — avoids GORM
		// upsert quirks when attaching tag/channel stubs to resources.
		// We use a separate sqlc repo instance (no counter) so the seed
		// round-trips don't pollute the test's counter.
		sqlcSeedRepo := store.NewResourceRepositorySQLC(fx.Runtime)

		// Seed 3 tags, 2 channels, 1 component.
		tagsRepo := store.NewTagsRepositorySQLC(fx.Runtime)
		chRepo := store.NewNotificationChannelRepositorySQLC(fx.Runtime)
		compRepo := store.NewComponentRepositorySQLC(fx.Runtime)

		comp := &domain.Component{
			Base: domain.Base{ID: "rt-comp", CreatedAt: time.Now()},
			Name: "rt-comp",
		}
		_, err := compRepo.Create(ctx, comp)
		require.NoError(t, err)

		tagStubs := []*domain.Tags{}
		for i := 0; i < 3; i++ {
			id := fmt.Sprintf("rt-tag-%d", i)
			require.NoError(t, tagsRepo.Create(ctx, &domain.Tags{Base: domain.Base{ID: id, CreatedAt: time.Now()}, Name: id}))
			tagStubs = append(tagStubs, &domain.Tags{Base: domain.Base{ID: id}})
		}
		chStubs := []*domain.NotificationChannel{}
		for i := 0; i < 2; i++ {
			id := fmt.Sprintf("rt-ch-%d", i)
			require.NoError(t, chRepo.Create(ctx, &domain.NotificationChannel{
				Base:   domain.Base{ID: id, CreatedAt: time.Now()},
				Name:   id,
				Type:   domain.NotificationChannelTypeSlack,
				Config: []byte(`{"webhook":"https://example.com"}`),
			}))
			chStubs = append(chStubs, &domain.NotificationChannel{Base: domain.Base{ID: id}})
		}

		// Wipe resources between cases.
		wipeResources := func() {
			require.NoError(t, execRawDelete(ctx, fx, "DELETE FROM resource_tags"))
			require.NoError(t, execRawDelete(ctx, fx, "DELETE FROM resource_notification_channels"))
			require.NoError(t, execRawDelete(ctx, fx, "DELETE FROM resources"))
		}
		compID := comp.ID
		seedN := func(n int, withTags, withCh, withComp bool) {
			for i := 0; i < n; i++ {
				res := &domain.Resource{
					Base:     domain.Base{ID: fmt.Sprintf("rt-res-%04d", i), CreatedAt: time.Now()},
					Name:     fmt.Sprintf("rt-res-%04d", i),
					Type:     domain.ResourceHTTP,
					Target:   "https://example.com",
					IsActive: true,
				}
				if withTags {
					res.Tags = tagStubs
				}
				if withCh {
					res.NotificationChannels = chStubs
				}
				if withComp {
					res.ComponentID = &compID
				}
				_, err := sqlcSeedRepo.Create(ctx, res)
				require.NoError(t, err)
			}
		}

		// Cases. As of spec 050 the bound was 1 + 1 = 2 round-trips for List
		// (principal + Tags). Spec 067 adds the IncidentCount30d preload
		// (1 GROUP BY query). Spec 068 adds the Uptime7d + Uptime30d +
		// ResponseTimeAvg preload (3 bulk GROUP BY queries), bringing the
		// bound to 6. Channels / Component / Credential presence on seeded
		// rows is irrelevant — those preloads are not issued on the list
		// path. The test runs for two N values to prove invariance.
		type tc struct {
			name              string
			withTags          bool
			withCh            bool
			withComp          bool
			expectedRoundTrip int64
		}
		cases := []tc{
			{name: "no_relations", expectedRoundTrip: 6},
			{name: "tags_channels_only", withTags: true, withCh: true, expectedRoundTrip: 6},
			{name: "with_component", withTags: true, withCh: true, withComp: true, expectedRoundTrip: 6},
			{name: "component_only", withComp: true, expectedRoundTrip: 6},
		}

		for _, n := range []int{10, 100} {
			for _, c := range cases {
				name := fmt.Sprintf("N%d_%s", n, c.name)
				t.Run(name, func(t *testing.T) {
					wipeResources()
					seedN(n, c.withTags, c.withCh, c.withComp)

					sqlcRepo, counter := roundtripFactory(t, fx)
					counter.Reset()
					out, err := sqlcRepo.List(ctx, n, 0)
					require.NoError(t, err)
					assert.Len(t, out, n)

					got := counter.Snapshot()
					assert.Equalf(t, c.expectedRoundTrip, got,
						"round-trip count for N=%d preloads=%s: expected=%d got=%d (1 principal + per-relation IN queries)",
						n, c.name, c.expectedRoundTrip, got)
				})
			}
		}
	})
}
