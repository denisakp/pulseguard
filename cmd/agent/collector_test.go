package main

import (
	"context"
	"math"
	"testing"

	"github.com/denisakp/ogoune/pkg/agentwire"
)

// fakeCollector returns a fixed frame; used by the stream tests.
type fakeCollector struct {
	frame agentwire.Frame
	err   error
	calls int
}

func (f *fakeCollector) Collect(context.Context) (agentwire.Frame, error) {
	f.calls++
	return f.frame, f.err
}

// TestGopsutilCollector_Smoke verifies the real collector returns finite,
// in-range values on this host. CPU % on the first call may be ~0 (delta since
// process start), so it is only checked for finiteness/non-negativity.
func TestGopsutilCollector_Smoke(t *testing.T) {
	c := newGopsutilCollector("test-0.0.0")
	f, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if f.AgentVersion != "test-0.0.0" {
		t.Errorf("AgentVersion = %q", f.AgentVersion)
	}
	if math.IsNaN(f.CPUPct) || math.IsInf(f.CPUPct, 0) || f.CPUPct < 0 {
		t.Errorf("CPUPct not finite/non-negative: %v", f.CPUPct)
	}
	if f.MemPct < 0 || f.MemPct > 100 {
		t.Errorf("MemPct out of range: %v", f.MemPct)
	}
	if f.NetIn < 0 || f.NetOut < 0 {
		t.Errorf("net counters negative: in=%d out=%d", f.NetIn, f.NetOut)
	}
	for _, d := range f.Disks {
		if d.UsedPct < 0 || d.UsedPct > 100 {
			t.Errorf("disk %q used_pct out of range: %v", d.Mount, d.UsedPct)
		}
	}
	// The frame must satisfy the shared contract's validation.
	if err := f.Validate(); err != nil {
		t.Errorf("collected frame fails agentwire.Validate: %v", err)
	}
}
