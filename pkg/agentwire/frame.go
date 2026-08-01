// Package agentwire defines the single, versioned wire contract for the host
// metrics frame exchanged between the Ogoune agent (cmd/agent) and the backend
// ingestion route (internal/api/handler/v1/agent_stream_handler.go). Both sides
// import this package so the frame cannot drift.
package agentwire

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// SchemaVersion is the current frame schema version. A frame decoded without a
// schema_version is treated as version 1 (backward compatibility with agents
// and tests that predate the field).
const SchemaVersion = 1

// ErrUnsupportedVersion is returned when a frame declares a schema_version
// greater than SchemaVersion (a newer agent talking to an older backend).
var ErrUnsupportedVersion = errors.New("agentwire: unsupported schema_version")

// ErrMissingField is returned when a frame omits a required numeric field
// (cpu_pct, mem_pct, net_in, net_out). Such a frame is malformed and must not
// be stored — the backend leaves last_seen_at unadvanced.
var ErrMissingField = errors.New("agentwire: frame missing required field")

// DiskUsage is a per-mount disk utilisation entry.
type DiskUsage struct {
	Mount   string  `json:"mount"`
	UsedPct float64 `json:"used_pct"`
}

// Frame is one point-in-time host metrics sample sent by the agent. The backend
// assigns the storage timestamp; the agent never sends one.
type Frame struct {
	SchemaVersion int         `json:"schema_version"`
	OS            string      `json:"os,omitempty"`
	AgentVersion  string      `json:"agent_version,omitempty"`
	CPUPct        float64     `json:"cpu_pct"`
	MemPct        float64     `json:"mem_pct"`
	NetIn         int64       `json:"net_in"`
	NetOut        int64       `json:"net_out"`
	Disks         []DiskUsage `json:"disks"`
}

// Encode serialises a frame to JSON, stamping the current SchemaVersion when the
// caller left it zero.
func Encode(f Frame) ([]byte, error) {
	if f.SchemaVersion == 0 {
		f.SchemaVersion = SchemaVersion
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("agentwire: encode: %w", err)
	}
	return b, nil
}

// Decode parses a frame from JSON. An absent/zero schema_version is normalised
// to 1; a version greater than SchemaVersion returns ErrUnsupportedVersion; a
// frame omitting a required numeric field returns ErrMissingField. Required
// fields are probed via pointers so an absent field is distinguishable from a
// legitimate zero value.
func Decode(b []byte) (Frame, error) {
	var probe struct {
		CPUPct *float64 `json:"cpu_pct"`
		MemPct *float64 `json:"mem_pct"`
		NetIn  *int64   `json:"net_in"`
		NetOut *int64   `json:"net_out"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return Frame{}, fmt.Errorf("agentwire: decode: %w", err)
	}
	if probe.CPUPct == nil || probe.MemPct == nil || probe.NetIn == nil || probe.NetOut == nil {
		return Frame{}, ErrMissingField
	}

	var f Frame
	if err := json.Unmarshal(b, &f); err != nil {
		return Frame{}, fmt.Errorf("agentwire: decode: %w", err)
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = 1
	}
	if f.SchemaVersion > SchemaVersion {
		return Frame{}, fmt.Errorf("%w: %d (max %d)", ErrUnsupportedVersion, f.SchemaVersion, SchemaVersion)
	}
	return f, nil
}

// Validate rejects non-finite numeric values. Range clamping to [0,100] is the
// backend's responsibility (it is the authority), so it is intentionally not
// done here.
func (f Frame) Validate() error {
	if !finite(f.CPUPct) || !finite(f.MemPct) {
		return errors.New("agentwire: cpu_pct/mem_pct must be finite")
	}
	for _, d := range f.Disks {
		if !finite(d.UsedPct) {
			return fmt.Errorf("agentwire: disk %q used_pct must be finite", d.Mount)
		}
	}
	return nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
