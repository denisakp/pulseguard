package hostcred

import (
	"strings"
	"testing"
)

func TestGenerate_RoundTrip(t *testing.T) {
	raw, hash, prefix, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(raw, "ag_live_") {
		t.Fatalf("raw missing prefix: %q", raw)
	}
	if hash != Hash(raw) {
		t.Fatalf("returned hash %q != Hash(raw) %q", hash, Hash(raw))
	}
	if prefix != raw[:12] {
		t.Fatalf("prefix %q != raw[:12] %q", prefix, raw[:12])
	}
	if !IsFormat(raw) {
		t.Fatalf("IsFormat(raw) = false, want true")
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		raw, _, _, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[raw]; dup {
			t.Fatalf("duplicate credential generated: %q", raw)
		}
		seen[raw] = struct{}{}
	}
}

func TestIsFormat(t *testing.T) {
	cases := []struct {
		token string
		want  bool
	}{
		{"ag_live_abcdef0123456789", true},
		{"pk_live_abcdef0123456789", false},
		{"ag_live_", false}, // prefix only, no entropy
		{"", false},
		{"nonsense", false},
	}
	for _, tc := range cases {
		if got := IsFormat(tc.token); got != tc.want {
			t.Errorf("IsFormat(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

func TestHash_Deterministic(t *testing.T) {
	const raw = "ag_live_deadbeefdeadbeef"
	if Hash(raw) != Hash(raw) {
		t.Fatal("Hash not deterministic")
	}
	if Hash(raw) == Hash(raw+"x") {
		t.Fatal("Hash collision on distinct inputs")
	}
}
