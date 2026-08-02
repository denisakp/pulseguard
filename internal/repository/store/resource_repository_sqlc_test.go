package store_test

import (
	"testing"

	"github.com/denisakp/ogoune/internal/repository/internaltest"
	"github.com/denisakp/ogoune/internal/repository/store"
)

// TestResourceRepository_SqlcContract drives the existing GORM contract suite
// against the sqlc-backed ResourceRepository. PR1 of US1 covers
// CRUD only; deferred methods (Update with M2M, FindByTag,
// UpdateMonitoringState, UpdateMetadata) intentionally fail when invoked, so
// this test currently runs the SAME runResourceContract body — which today
// only exercises Create, FindByID, FindByHeartbeatSlug, basic Update (no
// Tags), Delete, FindActive, FindMissedHeartbeats, UpdateLastPingAt.
func TestResourceRepository_SqlcContract(t *testing.T) {
	internaltest.ForEachDialect(t, func(t *testing.T, fx *internaltest.DialectFixture) {
		repo := store.NewResourceRepositorySQLC(fx.Runtime)
		runResourceContract(t, repo)
	})
}
