package v1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentStream_IngestsFrameOverRealWebSocket exercises the real WebSocket
// upgrade + host-credential auth + service ingestion path end-to-end (US1,
// SC-008). This is the only test that drives an actual socket.
func TestAgentStream_IngestsFrameOverRealWebSocket(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	host, raw, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)

	agentH := v1.NewAgentStreamHandler(deps.metricsSvc)

	r := chi.NewRouter()
	r.With(middleware.HostCredentialAuth(deps.credSvc)).Get("/api/v1/agent/stream", agentH.Stream)
	server := httptest.NewServer(r)
	defer server.Close()

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, "ws://"+server.Listener.Addr().String()+"/api/v1/agent/stream",
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + raw}}})
	require.NoError(t, err, "authenticated dial should succeed")

	frame := map[string]any{
		"os":            "Ubuntu 24.04",
		"agent_version": "0.1.0",
		"cpu_pct":       12.4,
		"mem_pct":       47.1,
		"net_in":        100,
		"net_out":       200,
		"disks": []any{
			map[string]any{"mount": "/", "used_pct": 23.0},
		},
	}
	require.NoError(t, wsjson.Write(dialCtx, conn, frame))
	// Close normally; ingestion happens asynchronously on the server read-loop.
	_ = conn.Close(websocket.StatusNormalClosure, "")

	// Poll: the host snapshot should reflect the pushed values and be online.
	require.Eventually(t, func() bool {
		h, err := deps.hostSvc.Get(ctx, host.ID)
		if err != nil || !h.Online {
			return false
		}
		return h.LastCPUPct != nil && h.LastDiskPct != nil
	}, 2*time.Second, 20*time.Millisecond, "host should come online with pushed metrics")

	h, err := deps.hostSvc.Get(ctx, host.ID)
	require.NoError(t, err)
	assert.True(t, h.Online)
	require.NotNil(t, h.LastCPUPct)
	assert.InDelta(t, 12.4, *h.LastCPUPct, 0.001)
	require.NotNil(t, h.LastDiskPct)
	assert.InDelta(t, 23.0, *h.LastDiskPct, 0.001)

	samples, err := deps.metricsSvc.History(ctx, host.ID, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.NotEmpty(t, samples, "an ingested metric sample should exist in history")
	assert.InDelta(t, 12.4, samples[len(samples)-1].CPUPct, 0.001)
}

// TestAgentStream_RejectsBadCredential asserts the stream cannot be established
// with a wrong bearer token (middleware returns 401, never upgrades).
func TestAgentStream_RejectsBadCredential(t *testing.T) {
	deps := newHostTestDeps()
	ctx := context.Background()

	_, _, _, err := deps.hostSvc.Register(ctx, "web-1")
	require.NoError(t, err)

	agentH := v1.NewAgentStreamHandler(deps.metricsSvc)
	r := chi.NewRouter()
	r.With(middleware.HostCredentialAuth(deps.credSvc)).Get("/api/v1/agent/stream", agentH.Stream)
	server := httptest.NewServer(r)
	defer server.Close()

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Wrong bearer.
	conn, _, err := websocket.Dial(dialCtx, "ws://"+server.Listener.Addr().String()+"/api/v1/agent/stream",
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer ag_live_bogusbogusbogus"}}})
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	require.Error(t, err, "dial with a bad credential must not establish the stream")

	// Absent bearer.
	conn2, _, err2 := websocket.Dial(dialCtx, "ws://"+server.Listener.Addr().String()+"/api/v1/agent/stream", nil)
	if conn2 != nil {
		_ = conn2.Close(websocket.StatusNormalClosure, "")
	}
	require.Error(t, err2, "dial without a credential must not establish the stream")
}

// TestAgentStream_VersionedAndLegacyFrames asserts the backend accepts BOTH a
// frame carrying schema_version:1 and a legacy frame with no schema_version
func TestAgentStream_VersionedAndLegacyFrames(t *testing.T) {
	cases := []struct {
		name  string
		frame map[string]any
	}{
		{"versioned", map[string]any{
			"schema_version": 1,
			"cpu_pct":        33.0, "mem_pct": 44.0, "net_in": 1, "net_out": 2,
			"disks": []any{map[string]any{"mount": "/", "used_pct": 55.0}},
		}},
		{"legacy_no_version", map[string]any{
			"cpu_pct": 33.0, "mem_pct": 44.0, "net_in": 1, "net_out": 2,
			"disks": []any{map[string]any{"mount": "/", "used_pct": 55.0}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newHostTestDeps()
			ctx := context.Background()
			host, raw, _, err := deps.hostSvc.Register(ctx, "h")
			require.NoError(t, err)

			r := chi.NewRouter()
			r.With(middleware.HostCredentialAuth(deps.credSvc)).
				Get("/api/v1/agent/stream", v1.NewAgentStreamHandler(deps.metricsSvc).Stream)
			server := httptest.NewServer(r)
			defer server.Close()

			dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(dialCtx, "ws://"+server.Listener.Addr().String()+"/api/v1/agent/stream",
				&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + raw}}})
			require.NoError(t, err)
			require.NoError(t, wsjson.Write(dialCtx, conn, tc.frame))
			_ = conn.Close(websocket.StatusNormalClosure, "")

			require.Eventually(t, func() bool {
				h, err := deps.hostSvc.Get(ctx, host.ID)
				return err == nil && h.Online && h.LastCPUPct != nil
			}, 2*time.Second, 20*time.Millisecond, "host should ingest the %s frame", tc.name)
		})
	}
}
