package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/sqlc/dynquery"
	"github.com/denisakp/ogoune/internal/repository/store"
)

func strP(s string) *string { return &s }
func boolP(b bool) *bool    { return &b }

// TestResourceRepository_ListByFilter exercises the dynamic-filter SQL path
// against both SQLite and Postgres. Seeds a mix of monitors, then
// validates each filter alone and in combination.
func TestResourceRepository_ListByFilter(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		ctx := context.Background()
		repo := store.NewResourceRepositorySQLC(fx.Runtime)

		// Seed tags via the sqlc tags repo (sole impl post-decom).
		tagsRepo := store.NewTagsRepositorySQLC(fx.Runtime)
		tagProd := &domain.Tags{Base: domain.Base{ID: "tag-prod-" + fx.Dialect}, Name: "production"}
		tagStg := &domain.Tags{Base: domain.Base{ID: "tag-stg-" + fx.Dialect}, Name: "staging"}
		require.NoError(t, tagsRepo.Create(ctx, tagProd))
		require.NoError(t, tagsRepo.Create(ctx, tagStg))

		// Seed 8 resources covering all filter combinations.
		seed := []struct {
			id       string
			name     string
			typ      domain.ResourceType
			target   string
			isActive bool
			tag      *domain.Tags
		}{
			{"r1", "api-prod", domain.ResourceHTTP, "https://api.example.com", true, tagProd},
			{"r2", "api-stg", domain.ResourceHTTP, "https://stg.example.com", true, tagStg},
			{"r3", "db-prod", domain.ResourceTCP, "db.example.com:5432", true, tagProd},
			{"r4", "dns-check", domain.ResourceDNS, "example.com", true, nil},
			{"r5", "paused-monitor", domain.ResourceHTTP, "https://paused.com", false, nil},
			{"r6", "ping", domain.ResourceICMP, "1.1.1.1", true, tagProd},
			{"r7", "another-api", domain.ResourceHTTP, "https://other.com", true, nil},
			{"r8", "paused-api", domain.ResourceHTTP, "https://old.com", false, tagProd},
		}
		now := time.Now()
		for i, s := range seed {
			res := &domain.Resource{
				Base:     domain.Base{ID: fx.Dialect + "-" + s.id, CreatedAt: now.Add(time.Duration(i) * time.Second)},
				Name:     s.name,
				Type:     s.typ,
				Target:   s.target,
				IsActive: true, // GORM default:true overrides bool zero — set false via explicit Update below.
				Interval: 60,
				Timeout:  10,
			}
			if s.tag != nil {
				res.Tags = []*domain.Tags{s.tag}
			}
			created, err := repo.Create(ctx, res)
			require.NoError(t, err, "seed %s", s.id)
			if !s.isActive {
				// Drop is_active=false via a raw UPDATE (sqlc Resource Update
				// would require a full domain object; this is simpler for
				// post-create state adjustment in seed code).
				require.NoError(t, execRawUpdate(ctx, fx, "UPDATE resources SET is_active = ? WHERE id = ?", false, created.ID))
			}
		}

		cases := []struct {
			name      string
			f         dynquery.MonitorFilter
			wantTotal int
			wantIDs   []string // partial check — set if order-stable
		}{
			{
				name:      "no filter (default is_active=true)",
				f:         dynquery.MonitorFilter{},
				wantTotal: 6, // 8 - 2 paused
			},
			{
				name:      "is_active=false",
				f:         dynquery.MonitorFilter{IsActive: boolP(false)},
				wantTotal: 2,
			},
			{
				name:      "type=http",
				f:         dynquery.MonitorFilter{Type: strP("http")},
				wantTotal: 3, // r1, r2, r7 (r5 + r8 are paused)
			},
			{
				name:      "tag=production",
				f:         dynquery.MonitorFilter{Tag: strP("production")},
				wantTotal: 3, // r1, r3, r6 (r8 paused)
			},
			{
				name:      "type=http AND tag=production",
				f:         dynquery.MonitorFilter{Type: strP("http"), Tag: strP("production")},
				wantTotal: 1, // r1
			},
			{
				name:      "q=api",
				f:         dynquery.MonitorFilter{Q: strP("api")},
				wantTotal: 3, // r1, r2, r7 distinct rows (LIKE matches across name|target with OR — DISTINCT in COUNT prevents duplicates)
			},
			{
				name:      "q matches target only",
				f:         dynquery.MonitorFilter{Q: strP("1.1.1.1")},
				wantTotal: 1, // r6
			},
			{
				name:      "no matches",
				f:         dynquery.MonitorFilter{Tag: strP("nonexistent")},
				wantTotal: 0,
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				items, total, err := repo.ListResourcesByFilter(ctx, c.f, 1, 50)
				require.NoError(t, err)
				assert.Equal(t, c.wantTotal, total, "total mismatch (items=%d)", len(items))
				assert.Equal(t, c.wantTotal, len(items), "items length mismatch")
			})
		}

		t.Run("pagination", func(t *testing.T) {
			page1, total, err := repo.ListResourcesByFilter(ctx, dynquery.MonitorFilter{}, 1, 3)
			require.NoError(t, err)
			assert.Equal(t, 6, total)
			assert.Len(t, page1, 3)
			page2, _, err := repo.ListResourcesByFilter(ctx, dynquery.MonitorFilter{}, 2, 3)
			require.NoError(t, err)
			assert.Len(t, page2, 3)
			// No overlap.
			for _, a := range page1 {
				for _, b := range page2 {
					assert.NotEqual(t, a.ID, b.ID)
				}
			}
		})

		t.Run("q escapes LIKE wildcards", func(t *testing.T) {
			// Seed a resource whose name contains a literal '%' so we can
			// prove `q=%` does NOT match it as a wildcard.
			pct := &domain.Resource{
				Base:     domain.Base{ID: fx.Dialect + "-pct", CreatedAt: now.Add(100 * time.Second)},
				Name:     "100% uptime",
				Type:     domain.ResourceHTTP,
				Target:   "https://pct.example.com",
				IsActive: true,
				Interval: 60,
				Timeout:  10,
			}
			_, err := repo.Create(ctx, pct)
			require.NoError(t, err)

			// `q=100%` must match ONLY rows containing literal "100%".
			items, total, err := repo.ListResourcesByFilter(ctx, dynquery.MonitorFilter{Q: strP("100%")}, 1, 50)
			require.NoError(t, err)
			assert.Equal(t, 1, total, fmt.Sprintf("got items=%d", len(items)))
		})
	})
}

// execRawUpdate runs a parameterised UPDATE/DELETE statement via the raw
// driver-specific handle. Local helper used by filter tests for post-create
// state adjustments that aren't exposed via the sqlc Update wrappers.
func execRawUpdate(ctx context.Context, fx *internaltest.DialectFixture, query string, args ...any) error {
	if pool := fx.Runtime.PgxPool(); pool != nil {
		// pgx wants $1 placeholders; convert ? to $1, $2, ...
		q := convertQuestionMarksToDollar(query)
		_, err := pool.Exec(ctx, q, args...)
		return err
	}
	if db := fx.Runtime.SQLiteDB(); db != nil {
		_, err := db.ExecContext(ctx, query, args...)
		return err
	}
	return nil
}

func convertQuestionMarksToDollar(q string) string {
	var b []byte
	i := 1
	for _, c := range []byte(q) {
		if c == '?' {
			b = append(b, '$')
			b = append(b, []byte(fmt.Sprintf("%d", i))...)
			i++
			continue
		}
		b = append(b, c)
	}
	return string(b)
}
