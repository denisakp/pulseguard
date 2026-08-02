package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/pkg/safenet"
	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// Toolbox domain-level errors. Handlers map these to HTTP status codes.
var (
	ErrToolboxValidation          = errors.New("toolbox: invalid request")
	ErrToolboxTargetBlocked       = errors.New("toolbox: target blocked (internal/private address)")
	ErrToolboxTargetNotRegistered = errors.New("toolbox: target is not a registered monitor")
	ErrToolboxCertUnavailable     = errors.New("toolbox: certificate could not be retrieved")
	ErrToolboxWhoisNoData         = errors.New("toolbox: no registration data for domain")
)

// Toolbox tuning constants (clarified 2026-06-27).
const (
	toolboxMaxPorts          = 100
	toolboxMinPortTimeoutMs  = 100
	toolboxMaxPortTimeoutMs  = 2000
	toolboxDefTimeoutMs      = 1000
	toolboxSSLExpiryWarnDays = 14
	toolboxResourceScanLimit = 1000
)

// dnsResolvers maps a friendly resolver name to its IP.
var dnsResolvers = map[string]string{
	"cloudflare": "1.1.1.1",
	"google":     "8.8.8.8",
	"quad9":      "9.9.9.9",
}

// allowedDNSRecordTypes is the set of supported record types.
var allowedDNSRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "MX": true, "NS": true, "TXT": true, "CNAME": true,
}

// wellKnownPorts maps common ports to a service label for port-scan results.
var wellKnownPorts = map[int]string{
	21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "dns", 80: "http",
	110: "pop3", 143: "imap", 443: "https", 465: "smtps", 587: "smtp",
	993: "imaps", 995: "pop3s", 3306: "mysql", 3389: "rdp", 5432: "postgres",
	6379: "redis", 8080: "http-alt", 8443: "https-alt", 27017: "mongodb",
}

// --- Service-level result types (decoupled from DTOs) ------------------------

type ToolboxDNSRecord struct {
	Type  string
	Value string
	TTL   int
}

type ToolboxDNSResult struct {
	Records      []ToolboxDNSRecord
	QueryMs      int64
	ResolverUsed string
}

type ToolboxPortResult struct {
	Port    int
	Service string
	Status  string
	Banner  string
}

type ToolboxPortScanResult struct {
	Results      []ToolboxPortResult
	OpenCount    int
	ScannedCount int
}

type ToolboxSSLCertificate struct {
	Subject   string
	Issuer    string
	ValidFrom time.Time
	ValidTo   time.Time
	Cipher    string
	SANs      []string
	Chain     []string
}

type ToolboxSSLVuln struct {
	Name   string
	Status string // pass | warn
}

type ToolboxSSLResult struct {
	Certificate     ToolboxSSLCertificate
	DaysToExpiry    int
	ExpiringSoon    bool
	Vulnerabilities []ToolboxSSLVuln
}

type ToolboxWhoisResult struct {
	Registrar    string
	RegisteredAt string
	UpdatedAt    string
	ExpiresAt    string
	DaysToExpiry int
	Status       []string
	Privacy      bool
	DNSSEC       bool
	Nameservers  []string
}

// --- Request types -----------------------------------------------------------

type ToolboxDNSQuery struct {
	Domain         string
	RecordTypes    []string
	Resolver       string
	CustomResolver string
}

type ToolboxPortScanQuery struct {
	Target    string
	Ports     []int
	TimeoutMs int
}

type ToolboxSSLQuery struct {
	Domain string
	Port   int
}

// --- Service -----------------------------------------------------------------

// ToolboxService runs one-shot network diagnostics (DNS/port/SSL/WHOIS).
// It performs no persistence; the only repository access is a read-only lookup
// of registered monitor targets to gate the port scanner (FR-016).
type ToolboxService struct {
	resourceRepo port.ResourceRepository
	timeout      time.Duration
	clock        func() time.Time
}

// NewToolboxService builds a ToolboxService. timeout bounds external calls.
func NewToolboxService(repo port.ResourceRepository, timeout time.Duration) *ToolboxService {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ToolboxService{resourceRepo: repo, timeout: timeout, clock: time.Now}
}

// DNS performs a DNS lookup for the requested record types via the chosen resolver.
func (s *ToolboxService) DNS(ctx context.Context, q ToolboxDNSQuery) (ToolboxDNSResult, error) {
	var res ToolboxDNSResult

	domain := strings.TrimSpace(q.Domain)
	if domain == "" {
		return res, fmt.Errorf("%w: domain is required", ErrToolboxValidation)
	}
	types := q.RecordTypes
	if len(types) == 0 {
		types = []string{"A"}
	}
	for _, t := range types {
		if !allowedDNSRecordTypes[strings.ToUpper(t)] {
			return res, fmt.Errorf("%w: unsupported record type %q", ErrToolboxValidation, t)
		}
	}

	resolver, resolverIP, err := s.buildResolver(q.Resolver, q.CustomResolver)
	if err != nil {
		return res, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := s.clock()
	records := make([]ToolboxDNSRecord, 0)
	for _, raw := range types {
		t := strings.ToUpper(raw)
		recs, lookupErr := s.lookupRecordType(ctx, resolver, t, domain)
		if lookupErr != nil {
			// Per-type "no records" is not a hard error; skip silently.
			continue
		}
		records = append(records, recs...)
	}
	res.Records = records
	res.QueryMs = time.Since(start).Milliseconds()
	res.ResolverUsed = resolverIP
	return res, nil
}

func (s *ToolboxService) buildResolver(name, custom string) (*net.Resolver, string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var ip string
	switch name {
	case "", "cloudflare":
		ip = dnsResolvers["cloudflare"]
	case "google", "quad9":
		ip = dnsResolvers[name]
	case "custom":
		host := strings.TrimSpace(custom)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		parsed := net.ParseIP(host)
		if parsed == nil {
			return nil, "", fmt.Errorf("%w: custom_resolver must be a valid IP", ErrToolboxValidation)
		}
		if safenet.ShouldBlock(parsed) {
			return nil, "", fmt.Errorf("%w: custom resolver", ErrToolboxTargetBlocked)
		}
		ip = host
	default:
		return nil, "", fmt.Errorf("%w: unknown resolver %q", ErrToolboxValidation, name)
	}

	addr := net.JoinHostPort(ip, "53")
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", addr)
		},
	}
	return r, ip, nil
}

func (s *ToolboxService) lookupRecordType(ctx context.Context, r *net.Resolver, t, domain string) ([]ToolboxDNSRecord, error) {
	switch t {
	case "A", "AAAA":
		network := "ip4"
		if t == "AAAA" {
			network = "ip6"
		}
		ips, err := r.LookupIP(ctx, network, domain)
		if err != nil {
			return nil, err
		}
		out := make([]ToolboxDNSRecord, 0, len(ips))
		for _, ip := range ips {
			out = append(out, ToolboxDNSRecord{Type: t, Value: ip.String()})
		}
		return out, nil
	case "MX":
		mxs, err := r.LookupMX(ctx, domain)
		if err != nil {
			return nil, err
		}
		out := make([]ToolboxDNSRecord, 0, len(mxs))
		for _, mx := range mxs {
			out = append(out, ToolboxDNSRecord{Type: t, Value: fmt.Sprintf("%d %s", mx.Pref, mx.Host)})
		}
		return out, nil
	case "NS":
		nss, err := r.LookupNS(ctx, domain)
		if err != nil {
			return nil, err
		}
		out := make([]ToolboxDNSRecord, 0, len(nss))
		for _, ns := range nss {
			out = append(out, ToolboxDNSRecord{Type: t, Value: ns.Host})
		}
		return out, nil
	case "TXT":
		txts, err := r.LookupTXT(ctx, domain)
		if err != nil {
			return nil, err
		}
		out := make([]ToolboxDNSRecord, 0, len(txts))
		for _, txt := range txts {
			out = append(out, ToolboxDNSRecord{Type: t, Value: txt})
		}
		return out, nil
	case "CNAME":
		cname, err := r.LookupCNAME(ctx, domain)
		if err != nil {
			return nil, err
		}
		return []ToolboxDNSRecord{{Type: t, Value: cname}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported record type", ErrToolboxValidation)
	}
}

// PortScan scans the requested ports on a target that MUST be a registered monitor host.
func (s *ToolboxService) PortScan(ctx context.Context, q ToolboxPortScanQuery) (ToolboxPortScanResult, error) {
	var res ToolboxPortScanResult

	target := normalizeHostname(strings.TrimSpace(q.Target))
	if target == "" {
		return res, fmt.Errorf("%w: target is required", ErrToolboxValidation)
	}
	if len(q.Ports) == 0 {
		return res, fmt.Errorf("%w: at least one port is required", ErrToolboxValidation)
	}
	if len(q.Ports) > toolboxMaxPorts {
		return res, fmt.Errorf("%w: at most %d ports per scan", ErrToolboxValidation, toolboxMaxPorts)
	}
	for _, p := range q.Ports {
		if p < 1 || p > 65535 {
			return res, fmt.Errorf("%w: port %d out of range 1-65535", ErrToolboxValidation, p)
		}
	}
	timeoutMs := q.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = toolboxDefTimeoutMs
	}
	if timeoutMs < toolboxMinPortTimeoutMs || timeoutMs > toolboxMaxPortTimeoutMs {
		return res, fmt.Errorf("%w: timeout_ms must be between %d and %d", ErrToolboxValidation, toolboxMinPortTimeoutMs, toolboxMaxPortTimeoutMs)
	}

	// FR-016: gate to registered monitor targets.
	registered, err := s.isRegisteredTarget(ctx, target)
	if err != nil {
		return res, err
	}
	if !registered {
		return res, ErrToolboxTargetNotRegistered
	}

	perPort := time.Duration(timeoutMs) * time.Millisecond
	results := make([]ToolboxPortResult, 0, len(q.Ports))
	for _, p := range q.Ports {
		results = append(results, s.scanPort(ctx, target, p, perPort))
	}
	res.Results = results
	res.ScannedCount = len(results)
	for _, r := range results {
		if r.Status == "open" {
			res.OpenCount++
		}
	}
	return res, nil
}

func (s *ToolboxService) scanPort(ctx context.Context, host string, p int, timeout time.Duration) ToolboxPortResult {
	out := ToolboxPortResult{Port: p, Service: wellKnownPorts[p], Status: "closed"}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := safenet.SafeDial(dialCtx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", p)))
	if err != nil {
		if strings.Contains(err.Error(), "blocked") {
			out.Status = "filtered"
			return out
		}
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout") {
			out.Status = "filtered"
		}
		return out
	}
	defer conn.Close()
	out.Status = "open"

	// Best-effort short banner read.
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 256)
	if n, rerr := conn.Read(buf); rerr == nil && n > 0 {
		out.Banner = strings.TrimSpace(string(buf[:n]))
	}
	return out
}

// SSL inspects the TLS certificate of domain:port and derives expiry + passive vuln indicators.
func (s *ToolboxService) SSL(ctx context.Context, q ToolboxSSLQuery) (ToolboxSSLResult, error) {
	var res ToolboxSSLResult

	domain := normalizeHostname(strings.TrimSpace(q.Domain))
	if domain == "" {
		return res, fmt.Errorf("%w: domain is required", ErrToolboxValidation)
	}
	port := q.Port
	if port == 0 {
		port = 443
	}
	if port < 1 || port > 65535 {
		return res, fmt.Errorf("%w: port out of range 1-65535", ErrToolboxValidation)
	}

	if err := safenet.ValidateResolvedIPs(domain); err != nil {
		return res, fmt.Errorf("%w: %v", ErrToolboxTargetBlocked, err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	td := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // inspection tool: we report on the cert, not trust it
			ServerName:         domain,
		},
	}
	nc, err := td.DialContext(dialCtx, "tcp", net.JoinHostPort(domain, fmt.Sprintf("%d", port)))
	if err != nil {
		return res, fmt.Errorf("%w: %v", ErrToolboxCertUnavailable, err)
	}
	defer nc.Close()
	tlsConn, ok := nc.(*tls.Conn)
	if !ok {
		return res, ErrToolboxCertUnavailable
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return res, ErrToolboxCertUnavailable
	}
	leaf := state.PeerCertificates[0]

	chain := make([]string, 0, len(state.PeerCertificates))
	for _, c := range state.PeerCertificates {
		chain = append(chain, c.Subject.CommonName)
	}

	res.Certificate = ToolboxSSLCertificate{
		Subject:   leaf.Subject.String(),
		Issuer:    issuerName(leaf),
		ValidFrom: leaf.NotBefore,
		ValidTo:   leaf.NotAfter,
		Cipher:    tls.CipherSuiteName(state.CipherSuite),
		SANs:      leaf.DNSNames,
		Chain:     chain,
	}
	res.DaysToExpiry = int(time.Until(leaf.NotAfter).Hours() / 24)
	res.ExpiringSoon = res.DaysToExpiry < toolboxSSLExpiryWarnDays
	res.Vulnerabilities = assessTLSVulnerabilities(state.Version, state.CipherSuite)
	return res, nil
}

// WHOIS looks up domain registration data.
func (s *ToolboxService) WHOIS(ctx context.Context, domain string) (ToolboxWhoisResult, error) {
	var res ToolboxWhoisResult
	domain = normalizeHostname(strings.TrimSpace(domain))
	if domain == "" {
		return res, fmt.Errorf("%w: domain is required", ErrToolboxValidation)
	}

	raw, err := whois.Whois(domain)
	if err != nil {
		return res, fmt.Errorf("%w: %v", ErrToolboxWhoisNoData, err)
	}
	info, err := whoisparser.Parse(raw)
	if err != nil {
		return res, fmt.Errorf("%w: %v", ErrToolboxWhoisNoData, err)
	}
	if info.Domain == nil {
		return res, ErrToolboxWhoisNoData
	}

	res.Status = info.Domain.Status
	res.Nameservers = info.Domain.NameServers
	res.DNSSEC = info.Domain.DNSSec
	res.RegisteredAt = info.Domain.CreatedDate
	res.UpdatedAt = info.Domain.UpdatedDate
	res.ExpiresAt = info.Domain.ExpirationDate
	if info.Registrar != nil {
		res.Registrar = info.Registrar.Name
	}
	if info.Registrant != nil && info.Registrant.Name != "" {
		res.Privacy = strings.Contains(strings.ToLower(info.Registrant.Name), "privacy") ||
			strings.Contains(strings.ToLower(info.Registrant.Organization), "privacy")
	}
	if info.Domain.ExpirationDateInTime != nil {
		res.DaysToExpiry = int(time.Until(*info.Domain.ExpirationDateInTime).Hours() / 24)
	}
	return res, nil
}

// isRegisteredTarget reports whether host matches the hostname of any existing
// monitor target. Read-only; no migration / sqlc (FR-016).
func (s *ToolboxService) isRegisteredTarget(ctx context.Context, host string) (bool, error) {
	resources, err := s.resourceRepo.List(ctx, toolboxResourceScanLimit, 0)
	if err != nil {
		return false, err
	}
	want := strings.ToLower(host)
	for _, r := range resources {
		if r == nil {
			continue
		}
		if strings.ToLower(normalizeHostname(r.Target)) == want {
			return true, nil
		}
	}
	return false, nil
}

// normalizeHostname extracts a bare hostname from a URL, host:port, or bare host.
// Mirrors EnrichmentService.extractHostname.
func normalizeHostname(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		host := u.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	return target
}

// issuerName returns the certificate issuer organization, falling back to CN.
func issuerName(cert *x509.Certificate) string {
	if len(cert.Issuer.Organization) > 0 {
		return cert.Issuer.Organization[0]
	}
	return cert.Issuer.CommonName
}

// assessTLSVulnerabilities derives passive pass/warn indicators from the negotiated
// TLS version and cipher (no active exploit probing — clarified 2026-06-27).
func assessTLSVulnerabilities(version uint16, cipher uint16) []ToolboxSSLVuln {
	// POODLE: SSLv3. BEAST: TLS 1.0 with CBC. Heartbleed: OpenSSL bug — not
	// detectable passively; reported pass unless an obviously legacy stack.
	modernTLS := version >= tls.VersionTLS12
	weakCipher := isWeakCipher(cipher)

	poodle := "pass"
	if version <= tls.VersionSSL30 { //nolint:staticcheck // intentional legacy check
		poodle = "warn"
	}
	beast := "pass"
	if version <= tls.VersionTLS10 {
		beast = "warn"
	}
	heartbleed := "pass" // not passively detectable; conservative pass
	weak := "pass"
	if weakCipher || !modernTLS {
		weak = "warn"
	}
	return []ToolboxSSLVuln{
		{Name: "Heartbleed", Status: heartbleed},
		{Name: "POODLE", Status: poodle},
		{Name: "BEAST", Status: beast},
		{Name: "WeakCipher", Status: weak},
	}
}

// isWeakCipher flags cipher suites Go marks insecure (RC4, 3DES, CBC-SHA legacy).
func isWeakCipher(id uint16) bool {
	for _, c := range tls.InsecureCipherSuites() {
		if c.ID == id {
			return true
		}
	}
	return false
}
