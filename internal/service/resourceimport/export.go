package resourceimport

import (
	"context"
	"fmt"

	"github.com/denisakp/ogoune/internal/domain"
	"gopkg.in/yaml.v3"
)

// Export lists all current resources and returns a round-trippable manifest.
// Only declaration-shaped fields are emitted (name, type, target, timing,
// type-specific fields, and by-name tag/component/channel references); IDs,
// timestamps, status, and derived metrics are omitted.
func (s *Service) Export(ctx context.Context) (*Manifest, error) {
	all, err := s.resources.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	m := &Manifest{Version: ManifestVersion, Resources: make([]ResourceDecl, 0, len(all))}
	for _, r := range all {
		if r == nil {
			continue
		}
		m.Resources = append(m.Resources, toDecl(r))
	}
	return m, nil
}

// ExportYAML returns the current resource set marshaled as YAML.
func (s *Service) ExportYAML(ctx context.Context) ([]byte, error) {
	m, err := s.Export(ctx)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(m)
}

func toDecl(r *domain.Resource) ResourceDecl {
	decl := ResourceDecl{
		Name:                 r.Name,
		Type:                 string(r.Type),
		Target:               r.Target,
		Interval:             intPtr(r.Interval),
		Timeout:              intPtr(r.Timeout),
		Keyword:              r.Keyword,
		KeywordMode:          r.KeywordMode,
		ProtocolType:         r.ProtocolType,
		ProtocolPort:         r.ProtocolPort,
		HeartbeatInterval:    r.HeartbeatInterval,
		HeartbeatGrace:       r.HeartbeatGrace,
		Tags:                 tagNames(r.Tags),
		NotificationChannels: channelNames(r.NotificationChannels),
	}
	if r.ConfirmationChecks > 0 {
		decl.ConfirmationChecks = intPtr(r.ConfirmationChecks)
	}
	if r.ConfirmationInterval > 0 {
		decl.ConfirmationInterval = intPtr(r.ConfirmationInterval)
	}
	if r.Component != nil {
		decl.Component = r.Component.Name
	}
	// Heartbeat monitors have no meaningful external target.
	if r.Type == domain.ResourceHeartbeat {
		decl.Target = ""
	}
	return decl
}

func tagNames(tags []*domain.Tags) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != nil {
			out = append(out, t.Name)
		}
	}
	return out
}

func channelNames(channels []*domain.NotificationChannel) []string {
	if len(channels) == 0 {
		return nil
	}
	out := make([]string, 0, len(channels))
	for _, c := range channels {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}

func intPtr(v int) *int { return &v }
