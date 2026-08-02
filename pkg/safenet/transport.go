package safenet

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"
)

// SafeDial resolves the address, validates all IPs against the blocklist,
// and dials the first allowed IP. It derives timeout from context deadline.
// Used by TCP/Protocol strategies for execution-time SSRF defense.
func SafeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("safenet: invalid address %q: %w", addr, err)
	}

	// Resolve the hostname
	ips, err := resolveHost(host)
	if err != nil {
		return nil, err
	}

	// Validate all resolved IPs — block if ANY is in a blocked range
	if slices.ContainsFunc(ips, ShouldBlock) {
		logSecurityEvent("ssrf_block_exec", "", addr, "internal/private network addresses are not permitted")
		return nil, fmt.Errorf("safenet: connection to %s blocked: internal/private network addresses are not permitted", addr)
	}

	// Derive timeout from context deadline
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
		if timeout <= 0 {
			return nil, context.DeadlineExceeded
		}
	}

	// Dial the first resolved IP
	dialer := &net.Dialer{Timeout: timeout}
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)
		conn, err := dialer.DialContext(ctx, network, target)
		if err != nil {
			continue
		}
		return conn, nil
	}
	return nil, fmt.Errorf("safenet: failed to connect to any resolved IP for %s", addr)
}

// NewSafeTransport returns an http.Transport that validates resolved IPs
// before establishing connections, preventing DNS rebinding attacks.
func NewSafeTransport() *http.Transport {
	return &http.Transport{
		DialContext: SafeDial,
	}
}

// resolveHost resolves a hostname to a list of IPs. If host is already an IP, returns it directly.
func resolveHost(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("safenet: failed to resolve %q: %w", host, err)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("safenet: no valid IPs resolved for %q", host)
	}
	return ips, nil
}
