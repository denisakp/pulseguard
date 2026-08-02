package notifier

import (
	"context"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// ComponentNotification carries derived component state for aggregated notifications.
type ComponentNotification struct {
	Component domain.Component
	Status    domain.ComponentStatus
	Previous  *domain.ComponentStatus
	Impacted  []ComponentResource
}

// ComponentResource captures a simplified view of a resource for notifications.
type ComponentResource struct {
	ID     string
	Name   string
	Status domain.ResourceStatus
}

// NotificationPayload is a union of either a resource incident, a component update, or an expiry alert.
// Exactly one field should be non-nil per dispatch.
type NotificationPayload struct {
	Incident  *domain.Incident
	Component *ComponentNotification
	Expiry    *ExpiryNotification
	Flapping  *FlappingNotification
	Reminder  *ReminderNotification
	Report    *ReportNotification
	Operator  *OperatorNotification
}

// OperatorNotification is a plain operator-facing message (spec 083): an
// agent-down / recovery alert or an unread-notification digest. It carries no
// monitoring-specific structure — the SMTP and webhook notifiers render it
// generically. It is NOT a new channel type; existing channels deliver it.
type OperatorNotification struct {
	Title string   // subject / heading
	Body  string   // one-line summary
	Items []string // optional bullet list (e.g. offline hosts, unread entries)
}

// ExpiryNotification carries expiry-specific data for threshold alert dispatching.
type ExpiryNotification struct {
	Resource      domain.Resource
	ExpiryType    string // "ssl" | "domain"
	DaysRemaining int
	ExpiresAt     time.Time
	Issuer        string // certificate issuer (SSL) or registrar (domain)
	Threshold     int    // which threshold triggered this alert
	TriggeredAt   time.Time
}

type FlappingNotification struct {
	Resource           domain.Resource
	TransitionCount    int
	WindowSeconds      int
	MaxDurationMinutes int
	FlapStartedAt      *time.Time
	Stabilized         bool
	FinalStatus        domain.ResourceStatus
	TriggeredAt        time.Time
}

type ReminderNotification struct {
	Resource       domain.Resource
	Incident       domain.Incident
	ElapsedMinutes int
	TriggeredAt    time.Time
}

// Notifier defines the interface for sending notifications.
// Both SMTP and Webhook notifiers implement this interface.
type Notifier interface {
	Send(ctx context.Context, payload NotificationPayload) error
}
