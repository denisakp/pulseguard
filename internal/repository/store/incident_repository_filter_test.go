package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/internal/repository/store"
)

func timePtr(t time.Time) *time.Time { return &t }

// TestIncidentRepository_ListByFilter_Q exercises the free-text `q` search over
// incident cause (spec 084) on both dialects: case-insensitive substring +
// metacharacter escape-safety.
func TestIncidentRepository_ListByFilter_Q(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		ctx := context.Background()
		repo := store.NewIncidentRepositorySQLC(fx.Runtime)
		resRepo := store.NewResourceRepositorySQLC(fx.Runtime)

		mon := "mon-q-" + fx.Dialect
		_, err := resRepo.Create(ctx, &domain.Resource{
			Base: domain.Base{ID: mon}, Name: mon, Type: domain.ResourceHTTP,
			Target: "https://example.com", IsActive: true, Interval: 60, Timeout: 10,
		})
		require.NoError(t, err)

		causes := []struct{ id, cause string }{
			{"c1", "Connection timeout"},
			{"c2", "DNS resolution failure"},
			{"c3", "TLS handshake error"},
			{"c4", "50% packet loss"}, // literal % — must be escaped, not a wildcard
		}
		now := time.Now()
		for i, c := range causes {
			_, err := repo.Create(ctx, &domain.Incident{
				Base:       domain.Base{ID: fx.Dialect + "-q-" + c.id, CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now},
				ResourceID: mon, Cause: c.cause, StartedAt: now,
			})
			require.NoError(t, err)
		}

		qf := func(s string) dynquery.IncidentFilter { return dynquery.IncidentFilter{Q: &s} }
		count := func(s string) int {
			items, total, err := repo.ListIncidentsByFilter(ctx, qf(s), 1, 50)
			require.NoError(t, err)
			assert.Equal(t, total, len(items))
			return total
		}

		assert.Equal(t, 1, count("timeout"), "substring on cause")
		assert.Equal(t, 1, count("TIMEOUT"), "case-insensitive")
		assert.Equal(t, 1, count("failure"))
		assert.Equal(t, 1, count("error"))
		assert.Equal(t, 0, count("nomatch"))
		// escape-safety: a literal '%' must match only the one cause containing '%',
		// NOT every row (which is what an unescaped LIKE '%' would do).
		assert.Equal(t, 1, count("50%"))
		assert.Equal(t, 1, count("%"))
	})
}

// TestIncidentRepository_ListByFilter exercises the dynamic-filter SQL path
// against both SQLite and Postgres. Seeds incidents across 3
// monitors with varied resolved states + timestamps.
func TestIncidentRepository_ListByFilter(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		ctx := context.Background()
		repo := store.NewIncidentRepositorySQLC(fx.Runtime)

		// Seed parent resources so FK references resolve.
		resRepo := store.NewResourceRepositorySQLC(fx.Runtime)
		mon1 := "mon-" + fx.Dialect + "-1"
		mon2 := "mon-" + fx.Dialect + "-2"
		mon3 := "mon-" + fx.Dialect + "-3"
		for _, mid := range []string{mon1, mon2, mon3} {
			_, err := resRepo.Create(ctx, &domain.Resource{
				Base:     domain.Base{ID: mid},
				Name:     mid,
				Type:     domain.ResourceHTTP,
				Target:   "https://example.com",
				IsActive: true,
				Interval: 60,
				Timeout:  10,
			})
			require.NoError(t, err)
		}

		// Window: 2026-05-01 .. 2026-05-31.
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		seed := []struct {
			id        string
			resID     string
			startedAt time.Time
			resolved  bool
		}{
			{"i1", mon1, base, false},                   // open, day 1
			{"i2", mon1, base.AddDate(0, 0, 5), true},   // resolved, day 6
			{"i3", mon2, base.AddDate(0, 0, 10), false}, // open, day 11
			{"i4", mon2, base.AddDate(0, 0, 15), true},  // resolved, day 16
			{"i5", mon3, base.AddDate(0, 0, 20), false}, // open, day 21
			{"i6", mon3, base.AddDate(0, 0, 25), true},  // resolved, day 26
			{"i7", mon1, base.AddDate(0, 0, -5), true},  // resolved, before window
			{"i8", mon2, base.AddDate(0, 1, 5), false},  // open, after window
		}
		now := time.Now()
		for i, s := range seed {
			inc := &domain.Incident{
				Base:       domain.Base{ID: fx.Dialect + "-" + s.id, CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now},
				ResourceID: s.resID,
				Cause:      "test",
				StartedAt:  s.startedAt,
			}
			if s.resolved {
				r := s.startedAt.Add(time.Hour)
				inc.ResolvedAt = &r
			}
			_, err := repo.Create(ctx, inc)
			require.NoError(t, err, "seed %s", s.id)
		}

		windowFrom := base
		windowTo := base.AddDate(0, 1, 0) // 2026-06-01

		cases := []struct {
			name      string
			f         dynquery.IncidentFilter
			wantTotal int
		}{
			{"no filter", dynquery.IncidentFilter{}, 8},
			{"status=open", dynquery.IncidentFilter{Status: strP("open")}, 4}, // i1, i3, i5, i8
			{"status=resolved", dynquery.IncidentFilter{Status: strP("resolved")}, 4},
			{"monitor_id=mon1", dynquery.IncidentFilter{MonitorID: &mon1}, 3},
			{"monitor_id=mon2", dynquery.IncidentFilter{MonitorID: &mon2}, 3},
			{"from window", dynquery.IncidentFilter{From: timePtr(windowFrom)}, 7}, // excludes i7
			{"to window", dynquery.IncidentFilter{To: timePtr(windowTo)}, 7},       // excludes i8
			{"from + to (full window)", dynquery.IncidentFilter{From: timePtr(windowFrom), To: timePtr(windowTo)}, 6},
			{"status=open + from", dynquery.IncidentFilter{Status: strP("open"), From: timePtr(windowFrom)}, 4}, // i1,i3,i5,i8 (no upper bound)
			{"monitor_id=mon1 + status=resolved", dynquery.IncidentFilter{MonitorID: &mon1, Status: strP("resolved")}, 2},
			{"no matches", dynquery.IncidentFilter{MonitorID: strP("no-such-monitor")}, 0},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				items, total, err := repo.ListIncidentsByFilter(ctx, c.f, 1, 50)
				require.NoError(t, err)
				assert.Equal(t, c.wantTotal, total, "items=%d", len(items))
				assert.Equal(t, c.wantTotal, len(items))
			})
		}

		t.Run("pagination", func(t *testing.T) {
			page1, total, err := repo.ListIncidentsByFilter(ctx, dynquery.IncidentFilter{}, 1, 3)
			require.NoError(t, err)
			assert.Equal(t, 8, total)
			assert.Len(t, page1, 3)
			page2, _, err := repo.ListIncidentsByFilter(ctx, dynquery.IncidentFilter{}, 2, 3)
			require.NoError(t, err)
			assert.Len(t, page2, 3)
			for _, a := range page1 {
				for _, b := range page2 {
					assert.NotEqual(t, a.ID, b.ID)
				}
			}
		})
	})
}
