package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/denisakp/ogoune/pkg/agentwire"
)

// fakeConn captures frames written to it.
type fakeConn struct {
	mu     sync.Mutex
	frames [][]byte
}

func (c *fakeConn) Write(_ context.Context, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, append([]byte(nil), data...))
	return nil
}

func (c *fakeConn) Close(websocket.StatusCode, string) error { return nil }

func (c *fakeConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}

func (c *fakeConn) last() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.frames) == 0 {
		return nil
	}
	return c.frames[len(c.frames)-1]
}

func newTestStreamer(cfg Config, collector Collector, dial dialFunc, sleep func(context.Context, time.Duration) bool) *Streamer {
	return &Streamer{
		cfg:       cfg,
		collector: collector,
		dial:      dial,
		sleep:     sleep,
		transient: backoff{base: 1 * time.Second, max: 30 * time.Second},
		auth:      backoff{base: 30 * time.Second, max: 5 * time.Minute},
	}
}

func testCfg() Config {
	return Config{BackendURL: "wss://h/api/v1/agent/stream", Credential: "ag_live_x", Interval: time.Hour}
}

// TestStreamer_SendsFrame covers the send path: collect → encode → write. The
// interval is huge so only the immediate send fires before we cancel.
func TestStreamer_SendsFrame(t *testing.T) {
	conn := &fakeConn{}
	col := &fakeCollector{frame: agentwire.Frame{CPUPct: 12.4, MemPct: 47.1, NetIn: 1, NetOut: 2, Disks: []agentwire.DiskUsage{{Mount: "/", UsedPct: 23}}}}
	dial := func(context.Context) (wsConn, int, error) { return conn, 101, nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newTestStreamer(testCfg(), col, dial, func(context.Context, time.Duration) bool { return true })

	go func() { _ = s.Run(ctx) }()

	waitFor(t, func() bool { return conn.count() >= 1 }, "frame should be sent after connect")
	cancel()

	f, err := agentwire.Decode(conn.last())
	if err != nil {
		t.Fatalf("decode sent frame: %v", err)
	}
	if f.CPUPct != 12.4 || f.Disks[0].UsedPct != 23 {
		t.Fatalf("sent frame mismatch: %+v", f)
	}
	if f.SchemaVersion != agentwire.SchemaVersion {
		t.Fatalf("sent frame not version-stamped: %d", f.SchemaVersion)
	}
}

// TestStreamer_TransientBackoff asserts a failing (non-auth) dial retries with
// the fast exponential regime (1s, 2s, 4s …).
func TestStreamer_TransientBackoff(t *testing.T) {
	dialErr := errors.New("connection refused")
	dial := func(context.Context) (wsConn, int, error) { return nil, 0, dialErr }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var sleeps []time.Duration
	sleep := func(_ context.Context, d time.Duration) bool {
		mu.Lock()
		sleeps = append(sleeps, d)
		n := len(sleeps)
		mu.Unlock()
		if n >= 3 {
			cancel()
			return false
		}
		return true
	}
	s := newTestStreamer(testCfg(), &fakeCollector{}, dial, sleep)
	_ = s.Run(ctx)

	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	mu.Lock()
	defer mu.Unlock()
	if len(sleeps) < 3 {
		t.Fatalf("expected ≥3 backoffs, got %v", sleeps)
	}
	for i, w := range want {
		if sleeps[i] != w {
			t.Errorf("transient backoff[%d] = %s, want %s (seq %v)", i, sleeps[i], w, sleeps)
		}
	}
}

// TestStreamer_StartupRetry: backend unreachable at startup ⇒ retry, not exit.
func TestStreamer_StartupRetry(t *testing.T) {
	var calls int
	var mu sync.Mutex
	conn := &fakeConn{}
	dial := func(context.Context) (wsConn, int, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n < 3 {
			return nil, 0, errors.New("dial: no route to host")
		}
		return conn, 101, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newTestStreamer(testCfg(), &fakeCollector{frame: agentwire.Frame{CPUPct: 1, MemPct: 2}}, dial, instantSleep)
	go func() { _ = s.Run(ctx) }()

	waitFor(t, func() bool { return conn.count() >= 1 }, "agent should connect after startup retries, not exit")
	cancel()
}

// TestStreamer_AuthRejectionSlowBackoffAndSelfHeal asserts a persistent 401 uses
// the slow capped regime (30s, 60s, 120s …), never exits, and self-heals once
// the credential is re-validated (dial later succeeds).
func TestStreamer_AuthRejectionSlowBackoffAndSelfHeal(t *testing.T) {
	var mu sync.Mutex
	var calls int
	var sleeps []time.Duration
	conn := &fakeConn{}
	dial := func(context.Context) (wsConn, int, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n <= 3 {
			return nil, 401, errors.New("expected handshake status 401")
		}
		return conn, 101, nil // credential re-validated
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sleep := func(_ context.Context, d time.Duration) bool {
		mu.Lock()
		sleeps = append(sleeps, d)
		mu.Unlock()
		return true // never cancel via sleep; the test cancels after self-heal
	}
	s := newTestStreamer(testCfg(), &fakeCollector{frame: agentwire.Frame{CPUPct: 1, MemPct: 2}}, dial, sleep)
	go func() { _ = s.Run(ctx) }()

	waitFor(t, func() bool { return conn.count() >= 1 }, "agent should self-heal and stream after credential re-validation")
	cancel()

	mu.Lock()
	defer mu.Unlock()
	// First three backoffs are the slow auth regime, not the 1s transient one.
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second}
	if len(sleeps) < 3 {
		t.Fatalf("expected ≥3 auth backoffs, got %v", sleeps)
	}
	for i, w := range want {
		if sleeps[i] != w {
			t.Errorf("auth backoff[%d] = %s, want %s (seq %v)", i, sleeps[i], w, sleeps)
		}
	}
}

func TestBackoff_Sequence(t *testing.T) {
	b := backoff{base: time.Second, max: 8 * time.Second}
	got := []time.Duration{b.next(), b.next(), b.next(), b.next(), b.next()}
	want := []time.Duration{1, 2, 4, 8, 8}
	for i := range want {
		if got[i] != want[i]*time.Second {
			t.Errorf("next[%d] = %s, want %ds (%v)", i, got[i], want[i], got)
		}
	}
	b.reset()
	if b.next() != time.Second {
		t.Errorf("after reset, next = %s, want 1s", b.next())
	}
}

func instantSleep(context.Context, time.Duration) bool { return true }

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting: %s", msg)
}
