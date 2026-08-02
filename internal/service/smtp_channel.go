package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/pkg/notifier"
)

// ErrNoSMTPChannel indicates no SMTP notification channel is configured.
var ErrNoSMTPChannel = errors.New("no smtp notification channel configured")

// resolveOldestSMTP returns the oldest (by CreatedAt) SMTP notification channel
// and its parsed config. Shared by monthly reports (076) and operator alerting
// (083) so external delivery always targets the same, deterministic channel.
func resolveOldestSMTP(ctx context.Context, channels port.NotificationChannelRepository) (*domain.NotificationChannel, notifier.SMTPConfig, error) {
	list, err := channels.FindByType(ctx, domain.NotificationChannelTypeSMTP)
	if err != nil {
		return nil, notifier.SMTPConfig{}, fmt.Errorf("list smtp channels: %w", err)
	}
	if len(list) == 0 {
		return nil, notifier.SMTPConfig{}, ErrNoSMTPChannel
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	ch := list[0]
	var cfg notifier.SMTPConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return nil, notifier.SMTPConfig{}, fmt.Errorf("parse smtp channel config: %w", err)
	}
	return ch, cfg, nil
}

// smtpUser returns the effective SMTP username (User or legacy Username).
func smtpUser(cfg notifier.SMTPConfig) string {
	if cfg.User != "" {
		return cfg.User
	}
	return cfg.Username
}

// deliverOperatorSMTP sends a generic operator message to the oldest SMTP channel
// (spec 083 external delivery). Returns ErrNoSMTPChannel when none is configured.
// Best-effort: callers log the error and continue — delivery never blocks a scan.
func deliverOperatorSMTP(ctx context.Context, channels port.NotificationChannelRepository, op notifier.OperatorNotification) error {
	_, cfg, err := resolveOldestSMTP(ctx, channels)
	if err != nil {
		return err
	}
	recipient := cfg.Recipient
	if recipient == "" && len(cfg.Recipients) > 0 {
		recipient = cfg.Recipients[0]
	}
	n := notifier.NewSMTPNotifier(recipient, cfg.Sender, cfg.Host, string(cfg.Port), smtpUser(cfg), cfg.Password)
	return n.Send(ctx, notifier.NotificationPayload{Operator: &op})
}
