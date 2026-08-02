package safenet

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		// IPv4 loopback
		{"loopback 127.0.0.1", "127.0.0.1", true},
		{"loopback 127.255.255.255", "127.255.255.255", true},

		// IPv4 private
		{"private 10.0.0.1", "10.0.0.1", true},
		{"private 10.255.255.255", "10.255.255.255", true},
		{"private 172.16.0.1", "172.16.0.1", true},
		{"private 172.31.255.255", "172.31.255.255", true},
		{"private 192.168.0.1", "192.168.0.1", true},
		{"private 192.168.255.255", "192.168.255.255", true},

		// Not private 172.32.x.x
		{"not private 172.32.0.1", "172.32.0.1", false},

		// Link-local / metadata
		{"link-local 169.254.0.1", "169.254.0.1", true},
		{"metadata 169.254.169.254", "169.254.169.254", true},

		// IPv6 loopback
		{"ipv6 loopback", "::1", true},

		// IPv6 link-local
		{"ipv6 link-local", "fe80::1", true},

		// IPv6 ULA
		{"ipv6 ULA", "fd00::1", true},
		{"ipv6 ULA fc", "fc00::1", true},

		// Public IPs — should NOT be blocked
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
		{"public 203.0.113.1", "203.0.113.1", false},
		{"public ipv6", "2001:db8::1", false},

		// IPv4-mapped IPv6
		{"ipv4-mapped loopback", "::ffff:127.0.0.1", true},
		{"ipv4-mapped private", "::ffff:10.0.0.1", true},
		{"ipv4-mapped public", "::ffff:8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			got := IsBlockedIP(ip)
			if got != tt.blocked {
				t.Errorf("IsBlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestValidateAddress_HTTP(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		resType string
		wantErr bool
	}{
		{"valid public URL", "http://example.com", "http", false},
		{"valid HTTPS", "https://example.com/path", "http", false},
		{"loopback URL", "http://127.0.0.1:8080", "http", true},
		{"private 10.x", "http://10.0.0.1/test", "http", true},
		{"private 172.16.x", "http://172.16.0.1", "http", true},
		{"private 192.168.x", "http://192.168.1.1:3000", "http", true},
		{"metadata endpoint", "http://169.254.169.254/latest/meta-data/", "http", true},
		{"file scheme", "file:///etc/passwd", "http", true},
		{"ftp scheme", "ftp://internal.host/file", "http", true},
		{"gopher scheme", "gopher://internal:70", "http", true},
		{"ipv6 loopback", "http://[::1]:8080", "http", true},

		// Keyword type uses same validation
		{"keyword valid", "http://example.com", "keyword", false},
		{"keyword loopback", "http://127.0.0.1", "keyword", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.target, tt.resType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress(%q, %q) error = %v, wantErr %v", tt.target, tt.resType, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAddress_TCP(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{"valid public", "93.184.216.34:80", false},
		{"loopback", "127.0.0.1:6379", true},
		{"private 10.x", "10.0.0.1:5432", true},
		{"private 192.168.x", "192.168.1.1:3306", true},
		{"ipv6 loopback", "[::1]:27017", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.target, "tcp")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress(%q, tcp) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAddress_ICMP(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{"public IP", "8.8.8.8", false},
		{"loopback", "127.0.0.1", true},
		{"private", "10.0.0.1", true},
		{"metadata", "169.254.169.254", true},
		{"ipv6 loopback", "::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.target, "icmp")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress(%q, icmp) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestSafeDial_BlockedIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Direct IP — should be blocked immediately
	_, err := SafeDial(ctx, "tcp", "127.0.0.1:8080")
	if err == nil {
		t.Error("SafeDial should block connection to 127.0.0.1")
	}

	_, err = SafeDial(ctx, "tcp", "10.0.0.1:5432")
	if err == nil {
		t.Error("SafeDial should block connection to 10.0.0.1")
	}

	_, err = SafeDial(ctx, "tcp", "[::1]:8080")
	if err == nil {
		t.Error("SafeDial should block connection to ::1")
	}
}

func TestValidateResolvedIPs_DirectIP(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"public", "8.8.8.8", false},
		{"loopback", "127.0.0.1", true},
		{"private", "192.168.1.1", true},
		{"metadata", "169.254.169.254", true},
		{"ipv6 loopback", "::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResolvedIPs(tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResolvedIPs(%q) error = %v, wantErr %v", tt.host, err, tt.wantErr)
			}
		})
	}
}

func TestShouldBlock_RespectsAllowPrivate(t *testing.T) {
	priv := net.ParseIP("192.168.1.10")
	pub := net.ParseIP("8.8.8.8")

	// Default: private is blocked, public is not.
	SetAllowPrivate(false)
	if !ShouldBlock(priv) {
		t.Error("private IP should be blocked by default")
	}
	if ShouldBlock(pub) {
		t.Error("public IP should never be blocked")
	}

	// Opt-in: private is allowed; public still fine.
	SetAllowPrivate(true)
	if ShouldBlock(priv) {
		t.Error("private IP should be allowed when ALLOW_PRIVATE_TARGETS is set")
	}
	if ShouldBlock(pub) {
		t.Error("public IP should never be blocked")
	}

	// IsBlockedIP stays a pure range predicate regardless of the flag.
	if !IsBlockedIP(priv) {
		t.Error("IsBlockedIP must remain a pure predicate (192.168/16 is private)")
	}

	SetAllowPrivate(false) // restore default for other tests
}

func TestValidateResolvedIPs_AllowPrivate(t *testing.T) {
	SetAllowPrivate(true)
	defer SetAllowPrivate(false)
	if err := ValidateResolvedIPs("10.0.0.5"); err != nil {
		t.Errorf("private IP should validate when allowed: %v", err)
	}
}
