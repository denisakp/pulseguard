package service

import (
	"context"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/fake"
)

type stubHostLister struct{ hosts []*domain.Host }

func (s *stubHostLister) List(_ context.Context, limit, offset int) ([]*domain.Host, error) {
	if offset >= len(s.hosts) {
		return nil, nil
	}
	end := offset + limit
	if end > len(s.hosts) {
		end = len(s.hosts)
	}
	return s.hosts[offset:end], nil
}

type stubEmitter struct {
	emitted []domain.EmittedNotification
	err     error
}

func (s *stubEmitter) Emit(_ context.Context, n domain.EmittedNotification) error {
	if s.err != nil {
		return s.err
	}
	s.emitted = append(s.emitted, n)
	return nil
}

func newLiveness(t *testing.T, hosts []*domain.Host, now time.Time, external bool) (*HostLivenessService, *stubEmitter) {
	t.Helper()
	em := &stubEmitter{}
	svc := NewHostLivenessService(
		&stubHostLister{hosts: hosts},
		fake.NewHostAlertStateFake(),
		em,
		fake.NewNotificationChannelFake(),
		30*time.Second,
		external,
	)
	svc.SetNow(func() time.Time { return now })
	return svc, em
}

func hostSeen(name string, seen *time.Time) *domain.Host {
	h := &domain.Host{Name: name, LastSeenAt: seen}
	h.EnsureID()
	return h
}

func countBySeverity(ns []domain.EmittedNotification, sev string) int {
	n := 0
	for _, x := range ns {
		if x.Severity == sev && x.Category == domain.NotificationCategoryHost {
			n++
		}
	}
	return n
}

func TestLiveness_ConfirmThenAlertOnce(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute) // offline (freshness 30s)
	h := hostSeen("web-1", &off)
	svc, em := newLiveness(t, []*domain.Host{h}, now, false)
	ctx := context.Background()

	// cycle 1: first offline observation → record, no alert.
	if err := svc.Detect(ctx); err != nil {
		t.Fatalf("detect1: %v", err)
	}
	if got := countBySeverity(em.emitted, domain.NotificationSeverityError); got != 0 {
		t.Fatalf("cycle1 offline alerts = %d, want 0 (anti-flap)", got)
	}
	// cycle 2: confirmed offline → exactly one alert.
	_ = svc.Detect(ctx)
	if got := countBySeverity(em.emitted, domain.NotificationSeverityError); got != 1 {
		t.Fatalf("cycle2 offline alerts = %d, want 1", got)
	}
	// cycle 3: still offline → no new alert.
	_ = svc.Detect(ctx)
	if got := countBySeverity(em.emitted, domain.NotificationSeverityError); got != 1 {
		t.Fatalf("cycle3 offline alerts = %d, want 1 (no repeat)", got)
	}
}

func TestLiveness_RecoveryOnce(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute)
	h := hostSeen("web-1", &off)
	svc, em := newLiveness(t, []*domain.Host{h}, now, false)
	ctx := context.Background()

	_ = svc.Detect(ctx) // record offline
	_ = svc.Detect(ctx) // alert offline
	// host recovers.
	fresh := now
	h.LastSeenAt = &fresh
	_ = svc.Detect(ctx)
	if got := countBySeverity(em.emitted, domain.NotificationSeveritySuccess); got != 1 {
		t.Fatalf("recovery notifications = %d, want 1", got)
	}
	// another cycle online → no more recoveries.
	_ = svc.Detect(ctx)
	if got := countBySeverity(em.emitted, domain.NotificationSeveritySuccess); got != 1 {
		t.Fatalf("recovery repeated = %d, want 1", got)
	}
}

func TestLiveness_FlapNoAlert(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute)
	h := hostSeen("web-1", &off)
	svc, em := newLiveness(t, []*domain.Host{h}, now, false)
	ctx := context.Background()

	_ = svc.Detect(ctx) // first offline observation (no alert)
	// recovers before confirmation cycle.
	fresh := now
	h.LastSeenAt = &fresh
	_ = svc.Detect(ctx)
	if len(em.emitted) != 0 {
		t.Fatalf("flap produced %d notifications, want 0", len(em.emitted))
	}
}

func TestLiveness_NeverReportedSkipped(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	h := hostSeen("fresh-host", nil) // never reported
	svc, em := newLiveness(t, []*domain.Host{h}, now, false)
	_ = svc.Detect(context.Background())
	_ = svc.Detect(context.Background())
	if len(em.emitted) != 0 {
		t.Fatalf("never-reported host produced %d notifications, want 0", len(em.emitted))
	}
}

func TestLiveness_RestartNoReAlert(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute)
	h := hostSeen("web-1", &off)

	em := &stubEmitter{}
	state := fake.NewHostAlertStateFake()
	// Pre-seed as already-alerted offline (simulates state before a restart).
	since := now.Add(-5 * time.Minute)
	_ = state.Upsert(context.Background(), &domain.HostAlertState{
		HostID: h.ID, State: domain.HostAlertStateOffline, OfflineSince: &since, Alerted: true, UpdatedAt: now,
	})
	svc := NewHostLivenessService(&stubHostLister{hosts: []*domain.Host{h}}, state, em, fake.NewNotificationChannelFake(), 30*time.Second, false)
	svc.SetNow(func() time.Time { return now })

	_ = svc.Detect(context.Background())
	if len(em.emitted) != 0 {
		t.Fatalf("restart re-alerted %d times, want 0", len(em.emitted))
	}
}

func TestLiveness_ExternalNoChannelSwallowed(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute)
	h := hostSeen("web-1", &off)
	svc, em := newLiveness(t, []*domain.Host{h}, now, true) // external delivery on, but no SMTP channel
	ctx := context.Background()

	_ = svc.Detect(ctx)
	if err := svc.Detect(ctx); err != nil {
		t.Fatalf("detect returned error despite missing SMTP channel: %v", err)
	}
	if got := countBySeverity(em.emitted, domain.NotificationSeverityError); got != 1 {
		t.Fatalf("feed alert count = %d, want 1 (feed still recorded)", got)
	}
}

func TestLiveness_FeedEmitErrorSwallowed(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	off := now.Add(-2 * time.Minute)
	h := hostSeen("web-1", &off)
	em := &stubEmitter{err: context.DeadlineExceeded}
	svc := NewHostLivenessService(&stubHostLister{hosts: []*domain.Host{h}}, fake.NewHostAlertStateFake(), em, fake.NewNotificationChannelFake(), 30*time.Second, false)
	svc.SetNow(func() time.Time { return now })
	ctx := context.Background()

	_ = svc.Detect(ctx)
	if err := svc.Detect(ctx); err != nil {
		t.Fatalf("detect aborted on feed emit error: %v", err)
	}
}
