package agentwire

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestEncodeDecode_RoundTrip(t *testing.T) {
	in := Frame{
		OS:           "Ubuntu 24.04",
		AgentVersion: "0.1.0",
		CPUPct:       12.4,
		MemPct:       47.1,
		NetIn:        10432,
		NetOut:       88123,
		Disks:        []DiskUsage{{Mount: "/", UsedPct: 23.0}, {Mount: "/data", UsedPct: 71.2}},
	}
	b, err := Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d (Encode should stamp it)", out.SchemaVersion, SchemaVersion)
	}
	if out.CPUPct != in.CPUPct || out.MemPct != in.MemPct || out.NetIn != in.NetIn || out.NetOut != in.NetOut {
		t.Fatalf("scalar mismatch: %+v vs %+v", out, in)
	}
	if len(out.Disks) != 2 || out.Disks[1].Mount != "/data" || out.Disks[1].UsedPct != 71.2 {
		t.Fatalf("disks mismatch: %+v", out.Disks)
	}
	if out.OS != in.OS || out.AgentVersion != in.AgentVersion {
		t.Fatalf("os/agent_version mismatch: %+v", out)
	}
}

func TestDecode_AbsentVersionIsV1(t *testing.T) {
	// A legacy frame (the exact shape spec 079 accepts) with NO schema_version.
	raw := []byte(`{"os":"Debian 12","cpu_pct":5,"mem_pct":10,"net_in":1,"net_out":2,"disks":[{"mount":"/","used_pct":3}]}`)
	f, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode legacy: %v", err)
	}
	if f.SchemaVersion != 1 {
		t.Fatalf("absent schema_version → %d, want 1", f.SchemaVersion)
	}
	if f.CPUPct != 5 || f.Disks[0].UsedPct != 3 {
		t.Fatalf("legacy fields not decoded: %+v", f)
	}
}

func TestDecode_UnknownVersionRejected(t *testing.T) {
	raw := []byte(`{"schema_version":999,"cpu_pct":1,"mem_pct":1,"net_in":0,"net_out":0,"disks":[]}`)
	_, err := Decode(raw)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestDecode_Malformed(t *testing.T) {
	if _, err := Decode([]byte("not json")); err == nil {
		t.Fatal("expected error decoding malformed JSON")
	}
}

func TestDecode_MissingRequiredField(t *testing.T) {
	// cpu_pct present, mem_pct absent → malformed.
	raw := []byte(`{"cpu_pct":5,"net_in":1,"net_out":2,"disks":[]}`)
	_, err := Decode(raw)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("expected ErrMissingField, got %v", err)
	}
	// A real zero value is NOT missing.
	ok := []byte(`{"cpu_pct":0,"mem_pct":0,"net_in":0,"net_out":0,"disks":[]}`)
	if _, err := Decode(ok); err != nil {
		t.Fatalf("zero-valued frame should decode, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	if err := (Frame{CPUPct: 12, MemPct: 30}).Validate(); err != nil {
		t.Fatalf("valid frame rejected: %v", err)
	}
	if err := (Frame{CPUPct: math.NaN()}).Validate(); err == nil {
		t.Fatal("NaN cpu_pct should be rejected")
	}
	if err := (Frame{CPUPct: math.Inf(1)}).Validate(); err == nil {
		t.Fatal("Inf cpu_pct should be rejected")
	}
	if err := (Frame{Disks: []DiskUsage{{Mount: "/", UsedPct: math.NaN()}}}).Validate(); err == nil {
		t.Fatal("NaN disk used_pct should be rejected")
	}
}

func TestEncode_OmitsEmptyOptionalStrings(t *testing.T) {
	b, err := Encode(Frame{CPUPct: 1, MemPct: 2})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["os"]; ok {
		t.Fatal("empty os should be omitted")
	}
	if m["schema_version"] != float64(SchemaVersion) {
		t.Fatalf("schema_version = %v, want %d", m["schema_version"], SchemaVersion)
	}
}
