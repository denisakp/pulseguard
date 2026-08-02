package resourceimport

import (
	"context"
	"os"
	"testing"
	"time"

	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
)

// TestImport_GoldenScale imports a 100-resource manifest in a single request and
// asserts it completes well within an interactive budget.
// Heartbeat monitors are used so creation performs no network I/O.
func TestImport_GoldenScale(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden-100.yaml")
	if err != nil {
		t.Fatalf("read golden manifest: %v", err)
	}

	f := newImportFixture(t)
	start := time.Now()
	report, err := f.svc.Import(context.Background(), raw, dtoV1.ImportOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("golden import failed: %v", err)
	}
	if report.Created != 100 {
		t.Fatalf("created = %d, want 100", report.Created)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("import took %s, want < 5s", elapsed)
	}
}
