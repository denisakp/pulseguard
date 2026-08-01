package v1

import (
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/service"
	"github.com/denisakp/ogoune/pkg/agentwire"
)

// AgentStreamHandler upgrades the agent ingestion route to a WebSocket and
// reads periodic metric frames, delegating validation + persistence to
// HostMetricsService.Ingest. The socket is kept off the unit-test path by
// keeping all logic in the service; this handler is a thin read-loop. Frames
// are decoded through the shared pkg/agentwire contract so the agent and the
// backend cannot drift.
type AgentStreamHandler struct {
	metrics *service.HostMetricsService
}

func NewAgentStreamHandler(metrics *service.HostMetricsService) *AgentStreamHandler {
	return &AgentStreamHandler{metrics: metrics}
}

// Stream handles GET /api/v1/agent/stream (WebSocket upgrade, host-credential auth).
func (h *AgentStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	hostID, ok := middleware.HostIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Auth is by bearer host credential, not Origin; agents are non-browser.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// Client disconnected or context cancelled — end the read loop.
			break
		}
		frame, err := agentwire.Decode(data)
		if err != nil {
			// Malformed / missing-field / unsupported-version frame — do not
			// store, do not advance last_seen.
			slog.Debug("agent stream: rejected frame", "host_id", hostID, "error", err)
			continue
		}
		if err := h.metrics.Ingest(ctx, hostID, frameToSample(frame)); err != nil {
			slog.Warn("agent stream: ingest failed", "host_id", hostID, "error", err)
			// Keep the connection open; a transient bad frame must not drop the agent.
		}
	}
	conn.Close(websocket.StatusNormalClosure, "")
}

// frameToSample maps the shared wire frame to the ingestion sample. Optional
// string fields become nil pointers when empty so the host snapshot's
// COALESCE keeps prior values.
func frameToSample(f agentwire.Frame) service.IngestSample {
	s := service.IngestSample{
		CPUPct: f.CPUPct,
		MemPct: f.MemPct,
		NetIn:  f.NetIn,
		NetOut: f.NetOut,
	}
	if f.OS != "" {
		os := f.OS
		s.OS = &os
	}
	if f.AgentVersion != "" {
		av := f.AgentVersion
		s.AgentVersion = &av
	}
	for _, d := range f.Disks {
		s.Disks = append(s.Disks, domain.DiskUsage{Mount: d.Mount, UsedPct: d.UsedPct})
	}
	return s
}
