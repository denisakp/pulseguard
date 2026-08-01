package main

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	v1 "github.com/denisakp/ogoune/internal/api/handler/v1"
	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/pkg/agentwire"
)

// backendFixture stands up the REAL spec 079 ingestion path (HostCredentialAuth
// + AgentStreamHandler over in-memory fakes) behind an httptest server.
type backendFixture struct {
	server     *httptest.Server
	hostSvc    *service.HostService
	metricsSvc *service.HostMetricsService
}

func newBackendFixture(t *testing.T) *backendFixture {
	t.Helper()
	credFake := fake.NewHostCredentialFake()
	hostFake := fake.NewHostFake()
	metricFake := fake.NewHostMetricFake()
	resFake := fake.NewResourceFake()

	credSvc := service.NewHostCredentialService(credFake)
	hostSvc := service.NewHostService(hostFake, credSvc, metricFake, resFake, 45*time.Second)
	metricsSvc := service.NewHostMetricsService(metricFake, hostFake, 48*time.Hour, 7*24*time.Hour)

	r := chi.NewRouter()
	r.With(middleware.HostCredentialAuth(credSvc)).
		Get("/api/v1/agent/stream", v1.NewAgentStreamHandler(metricsSvc).Stream)

	fx := &backendFixture{server: httptest.NewServer(r), hostSvc: hostSvc, metricsSvc: metricsSvc}
	t.Cleanup(fx.server.Close)
	return fx
}

func (fx *backendFixture) wsURL() string {
	return "ws://" + fx.server.Listener.Addr().String() + "/api/v1/agent/stream"
}

// TestAgent_StreamsToRealBackend runs the real agent Streamer (real dial) against
// the real 079 handler and asserts the host comes online with the pushed values
// (SC-001, SC-008). A deterministic fake collector is injected so assertions are
// exact.
func TestAgent_StreamsToRealBackend(t *testing.T) {
	fx := newBackendFixture(t)
	ctx := context.Background()
	host, raw, _, err := fx.hostSvc.Register(ctx, "web-1")
	if err != nil {
		t.Fatalf("register host: %v", err)
	}

	cfg := Config{BackendURL: fx.wsURL(), Credential: raw, Interval: time.Hour}
	col := &fakeCollector{frame: agentwire.Frame{
		OS: "Ubuntu 24.04", AgentVersion: "0.1.0",
		CPUPct: 12.4, MemPct: 47.1, NetIn: 100, NetOut: 200,
		Disks: []agentwire.DiskUsage{{Mount: "/", UsedPct: 23}},
	}}
	s := NewStreamer(cfg, col) // real dial + real clock

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { _ = s.Run(runCtx) }()

	waitFor(t, func() bool {
		h, err := fx.hostSvc.Get(ctx, host.ID)
		return err == nil && h.Online && h.LastCPUPct != nil && h.LastDiskPct != nil
	}, "host should come online with metrics pushed by the real agent")
	cancel()

	h, err := fx.hostSvc.Get(ctx, host.ID)
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	if !h.Online {
		t.Fatal("host not online")
	}
	if h.LastCPUPct == nil || *h.LastCPUPct < 12.39 || *h.LastCPUPct > 12.41 {
		t.Fatalf("LastCPUPct = %v, want ~12.4", h.LastCPUPct)
	}
	if h.LastDiskPct == nil || *h.LastDiskPct < 22.9 || *h.LastDiskPct > 23.1 {
		t.Fatalf("LastDiskPct = %v, want ~23", h.LastDiskPct)
	}
	if h.OS == nil || *h.OS != "Ubuntu 24.04" {
		t.Fatalf("OS = %v, want Ubuntu 24.04", h.OS)
	}

	samples, err := fx.metricsSvc.History(ctx, host.ID, time.Time{}, time.Time{})
	if err != nil || len(samples) == 0 {
		t.Fatalf("expected a stored sample, got %d (err %v)", len(samples), err)
	}
}

// TestAgent_RealDialRejectsBadCredential asserts the real dialer gets a 401 when
// the credential is unknown/revoked (the auth boundary the agent then slow-backs
// off on).
func TestAgent_RealDialRejectsBadCredential(t *testing.T) {
	fx := newBackendFixture(t)
	cfg := Config{BackendURL: fx.wsURL(), Credential: "ag_live_notarealcredential", Interval: time.Hour}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, status, err := realDial(cfg)(ctx)
	if err == nil {
		t.Fatal("dial with bad credential should fail")
	}
	if status != 401 {
		t.Fatalf("expected HTTP 401 on bad credential, got %d", status)
	}
}
