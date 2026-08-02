package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/denisakp/ogoune/pkg/notifier"
)

// escalationFixture bundles the fresh fakes + service used by each escalation test.
type escalationFixture struct {
	feed  *fake.NotificationFeedRepository
	state *fake.NotificationEscalationStateFake
	svc   *NotificationEscalationService
}

func newEscalationFixture(t *testing.T, now time.Time) *escalationFixture {
	t.Helper()
	feed := fake.NewNotificationFeedRepository()
	state := fake.NewNotificationEscalationStateFake()
	channels := fake.NewNotificationChannelFake()
	svc := NewNotificationEscalationService(feed, state, channels, 30*time.Minute)
	svc.SetNow(func() time.Time { return now })
	return &escalationFixture{feed: feed, state: state, svc: svc}
}

// addFeed inserts an instance-wide (UserID nil) error-severity feed notification.
func (f *escalationFixture) addFeed(t *testing.T, category, title string, occurred time.Time, readAt *time.Time) {
	t.Helper()
	_, err := f.feed.Create(context.Background(), &domain.FeedNotification{
		UserID:     nil,
		Category:   category,
		Severity:   domain.NotificationSeverityError,
		Title:      title,
		OccurredAt: occurred,
		ReadAt:     readAt,
	})
	if err != nil {
		t.Fatalf("seed feed notification: %v", err)
	}
}

func TestEscalation_DigestWhenUnread(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour) // older than the 30m cutoff → qualifies

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, nil)
	f.addFeed(t, domain.NotificationCategoryHost, "agent offline", old, nil)

	var calls int
	var last notifier.OperatorNotification
	f.svc.SetDeliver(func(_ context.Context, op notifier.OperatorNotification) error {
		calls++
		last = op
		return nil
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if calls != 1 {
		t.Fatalf("deliver calls = %d, want 1", calls)
	}
	if !strings.Contains(last.Title, "2") {
		t.Errorf("digest title = %q, want it to mention count 2", last.Title)
	}
	if len(last.Items) == 0 {
		t.Errorf("digest items empty, want non-empty")
	}

	st, err := f.state.Get(ctx)
	if err != nil {
		t.Fatalf("get state after digest: %v", err)
	}
	if st.WatermarkOccurredAt == nil {
		t.Errorf("watermark not advanced, want non-nil WatermarkOccurredAt")
	}
}

func TestEscalation_DedupNoResend(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, nil)
	f.addFeed(t, domain.NotificationCategoryHost, "agent offline", old, nil)

	var calls int
	f.svc.SetDeliver(func(_ context.Context, _ notifier.OperatorNotification) error {
		calls++
		return nil
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect1: %v", err)
	}
	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect2: %v", err)
	}
	if calls != 1 {
		t.Fatalf("deliver calls = %d, want 1 (second cycle deduped)", calls)
	}
}

func TestEscalation_AllReadNoDigest(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)
	read := now.Add(-10 * time.Minute)

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, &read)
	f.addFeed(t, domain.NotificationCategoryHost, "agent offline", old, &read)

	var calls int
	f.svc.SetDeliver(func(_ context.Context, _ notifier.OperatorNotification) error {
		calls++
		return nil
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if calls != 0 {
		t.Fatalf("deliver calls = %d, want 0 (all read)", calls)
	}
}

func TestEscalation_NewerEntryNewDigest(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, nil)

	var calls int
	f.svc.SetDeliver(func(_ context.Context, _ notifier.OperatorNotification) error {
		calls++
		return nil
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect1: %v", err)
	}
	if calls != 1 {
		t.Fatalf("deliver calls after cycle1 = %d, want 1", calls)
	}

	// Newer than the 1h watermark, still older than the 30m cutoff → new digest.
	newer := now.Add(-45 * time.Minute)
	f.addFeed(t, domain.NotificationCategoryIncident, "second monitor down", newer, nil)

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect2: %v", err)
	}
	if calls != 2 {
		t.Fatalf("deliver calls after cycle2 = %d, want 2 (newer entry re-digested)", calls)
	}
}

func TestEscalation_InformationalIgnored(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)

	f.addFeed(t, domain.NotificationCategoryGeneral, "welcome aboard", old, nil)
	f.addFeed(t, domain.NotificationCategoryGeneral, "digest tip", old, nil)

	var calls int
	f.svc.SetDeliver(func(_ context.Context, _ notifier.OperatorNotification) error {
		calls++
		return nil
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if calls != 0 {
		t.Fatalf("deliver calls = %d, want 0 (general category not actionable)", calls)
	}
}

func TestEscalation_NoSMTPChannelNoop(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now) // default deliver over an empty channel fake
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, nil)
	f.addFeed(t, domain.NotificationCategoryHost, "agent offline", old, nil)

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect returned error despite missing SMTP channel: %v", err)
	}

	if _, err := f.state.Get(ctx); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("state get err = %v, want repository.ErrNotFound (watermark not advanced)", err)
	}
}

func TestEscalation_SendFailureWatermarkUnchanged(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	f := newEscalationFixture(t, now)
	ctx := context.Background()
	old := now.Add(-1 * time.Hour)

	f.addFeed(t, domain.NotificationCategoryIncident, "monitor down", old, nil)
	f.addFeed(t, domain.NotificationCategoryHost, "agent offline", old, nil)

	f.svc.SetDeliver(func(_ context.Context, _ notifier.OperatorNotification) error {
		return errors.New("smtp send failed")
	})

	if err := f.svc.Detect(ctx); err != nil {
		t.Fatalf("detect returned error on delivery failure: %v", err)
	}

	if _, err := f.state.Get(ctx); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("state get err = %v, want repository.ErrNotFound (watermark not advanced)", err)
	}
}
