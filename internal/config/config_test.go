package config

import (
	"os"
	"testing"
	"time"
)

func TestStaticDirDefault(t *testing.T) {
	os.Unsetenv("STATIC_DIR")

	cfg := Load()

	if cfg.StaticDir != "web/dist" {
		t.Errorf("expected StaticDir default %q, got %q", "web/dist", cfg.StaticDir)
	}
}

func TestStaticDirOverride(t *testing.T) {
	os.Setenv("STATIC_DIR", "/app/static")
	defer os.Unsetenv("STATIC_DIR")

	cfg := Load()

	if cfg.StaticDir != "/app/static" {
		t.Errorf("expected StaticDir %q, got %q", "/app/static", cfg.StaticDir)
	}
}

func TestAgentDownDefaults(t *testing.T) {
	for _, k := range []string{"AGENT_DOWN_ALERTS_ENABLED", "AGENT_DOWN_SCAN_INTERVAL", "AGENT_DOWN_EXTERNAL_DELIVERY"} {
		os.Unsetenv(k)
	}
	cfg := Load()
	if !cfg.AgentDownAlertsEnabled {
		t.Error("AgentDownAlertsEnabled should default true")
	}
	if cfg.AgentDownScanInterval != 20*time.Second {
		t.Errorf("AgentDownScanInterval default = %s, want 20s", cfg.AgentDownScanInterval)
	}
	if cfg.AgentDownExternalDelivery {
		t.Error("AgentDownExternalDelivery should default false")
	}
}

func TestAgentDownOverrides(t *testing.T) {
	os.Setenv("AGENT_DOWN_ALERTS_ENABLED", "false")
	os.Setenv("AGENT_DOWN_SCAN_INTERVAL", "5s")
	os.Setenv("AGENT_DOWN_EXTERNAL_DELIVERY", "true")
	defer func() {
		os.Unsetenv("AGENT_DOWN_ALERTS_ENABLED")
		os.Unsetenv("AGENT_DOWN_SCAN_INTERVAL")
		os.Unsetenv("AGENT_DOWN_EXTERNAL_DELIVERY")
	}()
	cfg := Load()
	if cfg.AgentDownAlertsEnabled {
		t.Error("AgentDownAlertsEnabled should be false")
	}
	if cfg.AgentDownScanInterval != 5*time.Second {
		t.Errorf("AgentDownScanInterval = %s, want 5s", cfg.AgentDownScanInterval)
	}
	if !cfg.AgentDownExternalDelivery {
		t.Error("AgentDownExternalDelivery should be true")
	}
}

func TestEnableICMPDefaultFalse(t *testing.T) {
	os.Unsetenv("ENABLE_ICMP")

	cfg := Load()

	if cfg.EnableICMP {
		t.Error("expected EnableICMP default to be false")
	}
}

func TestEnableICMPSetTrue(t *testing.T) {
	os.Setenv("ENABLE_ICMP", "true")
	defer os.Unsetenv("ENABLE_ICMP")

	cfg := Load()

	if !cfg.EnableICMP {
		t.Error("expected EnableICMP to be true when ENABLE_ICMP=true")
	}
}

func TestEnableICMPSetFalseExplicit(t *testing.T) {
	os.Setenv("ENABLE_ICMP", "false")
	defer os.Unsetenv("ENABLE_ICMP")

	cfg := Load()

	if cfg.EnableICMP {
		t.Error("expected EnableICMP to be false when ENABLE_ICMP=false")
	}
}

func TestMetricsEnabledDefault(t *testing.T) {
	os.Unsetenv("ENABLE_METRICS")

	cfg := Load()

	if cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled default to be false")
	}
}

func TestMetricsEnabledSetTrue(t *testing.T) {
	os.Setenv("ENABLE_METRICS", "true")
	defer os.Unsetenv("ENABLE_METRICS")

	cfg := Load()

	if !cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled to be true when ENABLE_METRICS=true")
	}
}

func TestMetricsEnabledInvalidValueFalse(t *testing.T) {
	os.Setenv("ENABLE_METRICS", "notabool")
	defer os.Unsetenv("ENABLE_METRICS")

	cfg := Load()

	if cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled to be false for invalid value (fail-closed)")
	}
}

func TestMetricsTokenDefault(t *testing.T) {
	os.Unsetenv("METRICS_TOKEN")

	cfg := Load()

	if cfg.MetricsToken != "" {
		t.Errorf("expected MetricsToken default to be empty, got %q", cfg.MetricsToken)
	}
}

func TestMetricsTokenSet(t *testing.T) {
	os.Setenv("METRICS_TOKEN", "test-secret")
	defer os.Unsetenv("METRICS_TOKEN")

	cfg := Load()

	if cfg.MetricsToken != "test-secret" {
		t.Errorf("expected MetricsToken %q, got %q", "test-secret", cfg.MetricsToken)
	}
}

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		defaultCount  int
		defaultWindow time.Duration
		wantCount     int
		wantWindow    time.Duration
	}{
		{"valid 10/1m", "10/1m", 5, 30 * time.Second, 10, 1 * time.Minute},
		{"valid 20/2m", "20/2m", 5, 30 * time.Second, 20, 2 * time.Minute},
		{"valid 100/60s", "100/60s", 5, 30 * time.Second, 100, 60 * time.Second},
		{"missing slash", "101m", 5, 30 * time.Second, 5, 30 * time.Second},
		{"empty string", "", 5, 30 * time.Second, 5, 30 * time.Second},
		{"non-numeric count", "abc/1m", 5, 30 * time.Second, 5, 30 * time.Second},
		{"invalid duration", "10/xyz", 5, 30 * time.Second, 5, 30 * time.Second},
		{"zero count", "0/1m", 5, 30 * time.Second, 5, 30 * time.Second},
		{"negative count", "-1/1m", 5, 30 * time.Second, 5, 30 * time.Second},
		{"spaces around", " 10 / 1m ", 5, 30 * time.Second, 10, 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, window := parseRateLimit(tt.input, tt.defaultCount, tt.defaultWindow)
			if count != tt.wantCount {
				t.Errorf("parseRateLimit(%q) count = %d, want %d", tt.input, count, tt.wantCount)
			}
			if window != tt.wantWindow {
				t.Errorf("parseRateLimit(%q) window = %v, want %v", tt.input, window, tt.wantWindow)
			}
		})
	}
}

func TestParseCORSOrigins(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of origins, -1 for nil
	}{
		{"empty string", "", -1},
		{"single origin", "https://example.com", 1},
		{"two origins", "https://a.com,https://b.com", 2},
		{"with spaces", " https://a.com , https://b.com ", 2},
		{"trailing comma", "https://a.com,", 1},
		{"only commas", ",,,", -1},
		{"whitespace only entries", " , , ", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCORSOrigins(tt.input)
			if tt.want == -1 {
				if got != nil {
					t.Errorf("parseCORSOrigins(%q) = %v, want nil", tt.input, got)
				}
			} else {
				if len(got) != tt.want {
					t.Errorf("parseCORSOrigins(%q) len = %d, want %d", tt.input, len(got), tt.want)
				}
			}
		})
	}
}
