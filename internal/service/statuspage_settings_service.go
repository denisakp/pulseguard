package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
)

// Spec 059 fold: custom-domain DNS state lives on StatusPageSettings (migration 0018).

const (
	SSLProviderLetsEncrypt = "letsencrypt"
	SSLProviderExternal    = "external"
	SSLProviderDisabled    = "disabled"
)

var (
	hostnameRE = regexp.MustCompile(`^(?:(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)\.)+[a-z]{2,63}$`)
	hexColorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	radiusRE   = regexp.MustCompile(`^(0|[1-9][0-9]?)(px|rem|em)?$`)

	ErrCustomDomainInvalidHostname = errors.New("invalid hostname")
	ErrInvalidHexColor             = errors.New("invalid hex color, expected #RRGGBB")
	ErrInvalidThemeKey             = errors.New("invalid theme override key")
	ErrInvalidThemeValue           = errors.New("invalid theme override value")
)

// themeOverrideKeys is the closed whitelist of CSS-variable names operators
// may set under `theme_overrides`. Each entry maps to its validator.
var themeOverrideKeys = map[string]func(string) bool{
	"--status-bg":       validateHex,
	"--status-text":     validateHex,
	"--status-up":       validateHex,
	"--status-degraded": validateHex,
	"--status-down":     validateHex,
	"--status-radius":   validateRadius,
}

func validateHex(v string) bool    { return hexColorRE.MatchString(v) }
func validateRadius(v string) bool { return radiusRE.MatchString(v) }

func validatePrimaryColor(c string) error {
	if c == "" {
		return nil
	}
	if !hexColorRE.MatchString(c) {
		return ErrInvalidHexColor
	}
	return nil
}

func validateThemeOverrides(m map[string]string) error {
	for k, v := range m {
		check, ok := themeOverrideKeys[k]
		if !ok {
			return fmt.Errorf("%w: %s", ErrInvalidThemeKey, k)
		}
		if !check(v) {
			return fmt.Errorf("%w: %s=%q", ErrInvalidThemeValue, k, v)
		}
	}
	return nil
}

// Logo slot persistence. Blob storage is handled
// by the handler (writes to STATIC_DIR); the service only persists the URL.

var ErrInvalidLogoSlot = errors.New("invalid logo slot")

var validLogoSlots = map[string]struct{}{
	"light":   {},
	"dark":    {},
	"favicon": {},
}

func ValidateLogoSlot(slot string) error {
	if _, ok := validLogoSlots[slot]; !ok {
		return ErrInvalidLogoSlot
	}
	return nil
}

func (s *StatusPageSettingsService) SetLogoURL(ctx context.Context, slot, url string) (*domain.StatusPageSettings, error) {
	if err := ValidateLogoSlot(slot); err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	switch slot {
	case "light":
		existing.LogoURLLight = url
	case "dark":
		existing.LogoURLDark = url
	case "favicon":
		existing.FaviconURL = url
	}
	if err := s.repo.Upsert(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *StatusPageSettingsService) ClearLogo(ctx context.Context, slot string) (*domain.StatusPageSettings, error) {
	return s.SetLogoURL(ctx, slot, "")
}

// DNSResolver indirection so tests can swap the resolver.
type DNSResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupTXT(ctx context.Context, host string) ([]string, error)
}

type netResolver struct{ *net.Resolver }

func (n netResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	if n.Resolver == nil {
		return net.LookupCNAME(host)
	}
	return n.Resolver.LookupCNAME(ctx, host)
}

func (n netResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if n.Resolver == nil {
		return net.LookupTXT(host)
	}
	return n.Resolver.LookupTXT(ctx, host)
}

// StatusPageSettingsService handles status page settings logic including
// custom-domain DNS lifecycle.
type StatusPageSettingsService struct {
	repo        port.StatusPageSettingsRepository
	resolver    DNSResolver
	baseDomain  string
	sslProvider string
}

func NewStatusPageSettingsService(repo port.StatusPageSettingsRepository) *StatusPageSettingsService {
	return &StatusPageSettingsService{
		repo:        repo,
		resolver:    netResolver{},
		baseDomain:  "status.ogoune.app",
		sslProvider: SSLProviderExternal,
	}
}

// Configure wires runtime settings the constructor doesn't take. Bootstrap
// calls this after NewStatusPageSettingsService.
func (s *StatusPageSettingsService) Configure(baseDomain, sslProvider string) {
	if baseDomain != "" {
		s.baseDomain = baseDomain
	}
	if sslProvider != "" {
		s.sslProvider = sslProvider
	}
}

// SetResolver lets tests inject a fake DNS resolver.
func (s *StatusPageSettingsService) SetResolver(r DNSResolver) { s.resolver = r }

// SSLProvider returns the configured provider.
func (s *StatusPageSettingsService) SSLProvider() string { return s.sslProvider }

// GetSettings retrieves the current status page settings.
func (s *StatusPageSettingsService) GetSettings(ctx context.Context) (*domain.StatusPageSettings, error) {
	return s.repo.Get(ctx)
}

// UpdateSettings updates the status page settings. When `custom_domain` is
// set for the first time (or changed), seeds the 2 DNS records and resets
// verification + SSL state to pending/none.
func (s *StatusPageSettingsService) UpdateSettings(ctx context.Context, settings *domain.StatusPageSettings) error {
	settings.Name = strings.TrimSpace(settings.Name)
	settings.HomepageURL = strings.TrimSpace(settings.HomepageURL)
	settings.CustomDomain = strings.ToLower(strings.TrimSpace(settings.CustomDomain))
	settings.UmamiWebsiteID = strings.TrimSpace(settings.UmamiWebsiteID)
	settings.UmamiScriptURL = strings.TrimSpace(settings.UmamiScriptURL)

	if settings.Name == "" {
		settings.Name = "Status Page"
	}

	// Validate hostname if provided.
	if settings.CustomDomain != "" {
		if err := validateHostname(settings.CustomDomain); err != nil {
			return err
		}
	}

	// Spec 060 / US5 — branding validation.
	settings.PrimaryColor = strings.TrimSpace(settings.PrimaryColor)
	if err := validatePrimaryColor(settings.PrimaryColor); err != nil {
		return err
	}
	if err := validateThemeOverrides(settings.ThemeOverrides); err != nil {
		return err
	}

	existing, err := s.repo.Get(ctx)
	if err == nil && existing != nil {
		// Re-seed DNS records when the hostname changes (or was empty before).
		if settings.CustomDomain != existing.CustomDomain {
			settings.CustomDomainDNS = s.seedDNSRecords(settings.CustomDomain)
			settings.CustomDomainStatus = domain.DomainStatusPending
			settings.CustomDomainSSL = domain.DomainSSLStatusNone
		} else {
			// Preserve previous DNS state on partial saves that don't touch the domain.
			settings.CustomDomainDNS = existing.CustomDomainDNS
			settings.CustomDomainStatus = existing.CustomDomainStatus
			settings.CustomDomainSSL = existing.CustomDomainSSL
		}
	} else {
		// First save.
		if settings.CustomDomain != "" {
			settings.CustomDomainDNS = s.seedDNSRecords(settings.CustomDomain)
		} else {
			settings.CustomDomainDNS = nil
		}
		settings.CustomDomainStatus = domain.DomainStatusPending
		settings.CustomDomainSSL = domain.DomainSSLStatusNone
	}

	// Empty hostname → clear DNS state entirely.
	if settings.CustomDomain == "" {
		settings.CustomDomainDNS = nil
		settings.CustomDomainStatus = domain.DomainStatusPending
		settings.CustomDomainSSL = domain.DomainSSLStatusNone
	}

	return s.repo.Upsert(ctx, settings)
}

// VerifyDomain re-resolves the seeded DNS records and rolls up the domain
// status. When SSL_PROVIDER=letsencrypt and verification succeeds, the
// ssl_status flips from `none` to `provisioning` (the `provisioning → active`
// leg via ACME callback is deferred per FR-040).
func (s *StatusPageSettingsService) VerifyDomain(ctx context.Context) (*domain.StatusPageSettings, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if settings.CustomDomain == "" {
		return settings, nil
	}

	allOK := true
	updated := make([]domain.DNSRecord, 0, len(settings.CustomDomainDNS))
	for _, rec := range settings.CustomDomainDNS {
		next := rec
		next.LastError = nil
		switch strings.ToUpper(rec.Type) {
		case "CNAME":
			got, lookupErr := s.resolver.LookupCNAME(ctx, rec.Host)
			if lookupErr != nil {
				next.Status = "failed"
				msg := lookupErr.Error()
				next.LastError = &msg
				allOK = false
				break
			}
			if strings.TrimSuffix(strings.ToLower(got), ".") != strings.ToLower(rec.Value) {
				next.Status = "failed"
				msg := fmt.Sprintf("CNAME mismatch: got %q want %q", got, rec.Value)
				next.LastError = &msg
				allOK = false
				break
			}
			next.Status = "verified"
		case "TXT":
			vals, lookupErr := s.resolver.LookupTXT(ctx, rec.Host)
			if lookupErr != nil {
				next.Status = "failed"
				msg := lookupErr.Error()
				next.LastError = &msg
				allOK = false
				break
			}
			match := false
			for _, v := range vals {
				if v == rec.Value {
					match = true
					break
				}
			}
			if !match {
				next.Status = "failed"
				msg := "TXT record missing or mismatched"
				next.LastError = &msg
				allOK = false
				break
			}
			next.Status = "verified"
		}
		updated = append(updated, next)
	}
	settings.CustomDomainDNS = updated

	if allOK {
		settings.CustomDomainStatus = domain.DomainStatusVerified
		if s.sslProvider == SSLProviderLetsEncrypt && settings.CustomDomainSSL == domain.DomainSSLStatusNone {
			settings.CustomDomainSSL = domain.DomainSSLStatusProvisioning
			slog.Warn("ssl_provisioning_deferred",
				"domain", settings.CustomDomain,
				"reason", "ACME issuance callback not implemented (FR-040)",
			)
		}
	} else {
		settings.CustomDomainStatus = domain.DomainStatusFailed
	}

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *StatusPageSettingsService) seedDNSRecords(host string) []domain.DNSRecord {
	txtRaw := make([]byte, 24)
	_, _ = rand.Read(txtRaw)
	txtValue := base64.RawURLEncoding.EncodeToString(txtRaw)

	cnameTarget := s.baseDomain
	if cnameTarget == "" {
		cnameTarget = "status.ogoune.app"
	}

	return []domain.DNSRecord{
		{Type: "CNAME", Host: host, Value: cnameTarget, Status: "pending"},
		{Type: "TXT", Host: "_ogoune-challenge." + host, Value: txtValue, Status: "pending"},
	}
}

func validateHostname(host string) error {
	if len(host) == 0 || len(host) > 253 {
		return ErrCustomDomainInvalidHostname
	}
	if !hostnameRE.MatchString(host) {
		return ErrCustomDomainInvalidHostname
	}
	return nil
}

var _ = time.Second // keep import for future use
