package v1

import (
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/denisakp/ogoune/internal/api/middleware"
	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/service"
)

// AgentStreamHandler upgrades the agent ingestion route to a WebSocket and
// reads periodic JSON metric frames, delegating validation + persistence to
// HostMetricsService.Ingest. The socket is kept off the unit-test path by
// keeping all logic in the service; this handler is a thin read-loop.
type AgentStreamHandler struct {
	metrics *service.HostMetricsService
}

func NewAgentStreamHandler(metrics *service.HostMetricsService) *AgentStreamHandler {
	return &AgentStreamHandler{metrics: metrics}
}

// agentFrame is the wire frame. Required numeric fields are pointers so an
// absent field (malformed frame) is distinguishable from a zero value.
type agentFrame struct {
	OS           *string            `json:"os"`
	AgentVersion *string            `json:"agent_version"`
	CPUPct       *float64           `json:"cpu_pct"`
	MemPct       *float64           `json:"mem_pct"`
	NetIn        *int64             `json:"net_in"`
	NetOut       *int64             `json:"net_out"`
	Disks        []domain.DiskUsage `json:"disks"`
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
		var f agentFrame
		if err := wsjson.Read(ctx, conn, &f); err != nil {
			// Client disconnected or context cancelled — end the read loop.
			break
		}
		if f.CPUPct == nil || f.MemPct == nil || f.NetIn == nil || f.NetOut == nil {
			// Malformed frame — do not store, do not advance last_seen.
			slog.Debug("agent stream: malformed frame", "host_id", hostID)
			continue
		}
		sample := service.IngestSample{
			OS:           f.OS,
			AgentVersion: f.AgentVersion,
			CPUPct:       *f.CPUPct,
			MemPct:       *f.MemPct,
			NetIn:        *f.NetIn,
			NetOut:       *f.NetOut,
			Disks:        f.Disks,
		}
		if err := h.metrics.Ingest(ctx, hostID, sample); err != nil {
			slog.Warn("agent stream: ingest failed", "host_id", hostID, "error", err)
			// Keep the connection open; a transient bad frame must not drop the agent.
		}
	}
	conn.Close(websocket.StatusNormalClosure, "")
}
