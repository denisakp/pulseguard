package safenet

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
)

// allowPrivate, when true, disables the private/loopback/link-local block so a
// trusted self-hosted instance can monitor its own LAN (opt-in via
// ALLOW_PRIVATE_TARGETS). Default false keeps public/SaaS instances SSRF-safe.
// Set once at startup via SetAllowPrivate.
var allowPrivate atomic.Bool

// SetAllowPrivate configures whether private-network targets are permitted.
func SetAllowPrivate(v bool) { allowPrivate.Store(v) }

// AllowPrivate reports the current policy.
func AllowPrivate() bool { return allowPrivate.Load() }

// ShouldBlock reports whether a resolved IP must be rejected: it is in a blocked
// range AND private targets are not permitted. This is the decision every SSRF
// guard should use (IsBlockedIP stays a pure range predicate).
func ShouldBlock(ip net.IP) bool {
	return !allowPrivate.Load() && IsBlockedIP(ip)
}

// blockedCIDRs contains all private, loopback, and link-local CIDR ranges
// that monitor targets must not resolve to.
var blockedCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918 Class A
		"172.16.0.0/12",  // RFC1918 Class B
		"192.168.0.0/16", // RFC1918 Class C
		"169.254.0.0/16", // Link-local / cloud metadata
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local (ULA)
	}
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("safenet: invalid CIDR %q: %v", cidr, err))
		}
		blockedCIDRs = append(blockedCIDRs, ipNet)
	}
}

// IsBlockedIP returns true if the given IP falls within any blocked CIDR range.
// For IPv4-mapped IPv6 addresses (::ffff:x.x.x.x), it checks the embedded IPv4.
func IsBlockedIP(ip net.IP) bool {
	// Normalize IPv4-mapped IPv6 to IPv4 for consistent matching
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateResolvedIPs resolves a hostname and validates all resulting IPs
// against the blocklist. Returns an error if any resolved IP is blocked.
// If host is already an IP address, it is validated directly.
func ValidateResolvedIPs(host string) error {
	// Check if host is already an IP
	if ip := net.ParseIP(host); ip != nil {
		if ShouldBlock(ip) {
			return fmt.Errorf("internal/private network addresses are not permitted")
		}
		return nil
	}

	// Resolve hostname
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %q: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip != nil && ShouldBlock(ip) {
			return fmt.Errorf("internal/private network addresses are not permitted")
		}
	}
	return nil
}

// ValidateAddress validates a monitor target address for SSRF safety.
// It checks scheme restrictions and resolves hostnames to verify they don't
// point to blocked IP ranges. Logs a [security] event on rejection.
func ValidateAddress(target string, resourceType string) error {
	switch resourceType {
	case "http", "keyword":
		return validateHTTPTarget(target)
	case "tcp", "protocol":
		return validateHostPort(target)
	case "icmp", "dns":
		return ValidateResolvedIPs(target)
	default:
		return nil
	}
}

func validateHTTPTarget(target string) error {
	u, err := url.ParseRequestURI(target)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid URL format")
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		logSecurityEvent("ssrf_block", "", target, "only http and https schemes are allowed")
		return fmt.Errorf("only http and https schemes are allowed")
	}

	host := u.Hostname()
	if err := ValidateResolvedIPs(host); err != nil {
		logSecurityEvent("ssrf_block", "", target, err.Error())
		return err
	}
	return nil
}

func validateHostPort(target string) error {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		// Target might be just a host without port
		host = target
	}
	if err := ValidateResolvedIPs(host); err != nil {
		logSecurityEvent("ssrf_block", "", target, err.Error())
		return err
	}
	return nil
}

func logSecurityEvent(eventType, sourceIP, target, reason string) {
	attrs := []slog.Attr{
		slog.String("event", eventType),
		slog.String("target", target),
		slog.String("reason", reason),
	}
	if sourceIP != "" {
		attrs = append(attrs, slog.String("source_ip", sourceIP))
	}
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	slog.Warn("security event", args...)
}
