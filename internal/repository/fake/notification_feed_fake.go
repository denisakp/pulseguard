package fake

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// NotificationFeedRepository — in-memory port.NotificationFeedRepository for tests.
type NotificationFeedRepository struct {
	mu   sync.RWMutex
	byID map[string]*domain.FeedNotification
}

func NewNotificationFeedRepository() *NotificationFeedRepository {
	return &NotificationFeedRepository{byID: make(map[string]*domain.FeedNotification)}
}

func (r *NotificationFeedRepository) Create(ctx context.Context, n *domain.FeedNotification) (*domain.FeedNotification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n.EnsureID()
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = now
	}
	if n.OccurredAt.IsZero() {
		n.OccurredAt = now
	}
	if _, exists := r.byID[n.ID]; exists {
		return nil, ErrDuplicate
	}
	cp := *n
	r.byID[n.ID] = &cp
	out := cp
	return &out, nil
}

// visible reports whether a notification is visible to userID (instance-wide or targeted).
func visible(n *domain.FeedNotification, userID string) bool {
	return n.UserID == nil || *n.UserID == userID
}

func (r *NotificationFeedRepository) filtered(userID string, category *string) []*domain.FeedNotification {
	out := make([]*domain.FeedNotification, 0)
	for _, n := range r.byID {
		if !visible(n, userID) {
			continue
		}
		if category != nil && n.Category != *category {
			continue
		}
		cp := *n
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurredAt.After(out[j].OccurredAt) })
	return out
}

func (r *NotificationFeedRepository) ListForUser(ctx context.Context, userID string, category *string, limit, offset int) ([]*domain.FeedNotification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := r.filtered(userID, category)
	if offset >= len(all) {
		return []*domain.FeedNotification{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *NotificationFeedRepository) CountForUser(ctx context.Context, userID string, category *string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.filtered(userID, category))), nil
}

func (r *NotificationFeedRepository) MarkRead(ctx context.Context, id string, at time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.byID[id]
	if !ok {
		return 0, nil // 0 rows affected → service maps to not-found
	}
	if n.ReadAt == nil {
		t := at
		n.ReadAt = &t
	}
	n.UpdatedAt = at
	return 1, nil
}

func (r *NotificationFeedRepository) MarkAllRead(ctx context.Context, userID string, before, at time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, n := range r.byID {
		if !visible(n, userID) || n.ReadAt != nil {
			continue
		}
		if n.OccurredAt.After(before) {
			continue
		}
		t := at
		n.ReadAt = &t
		n.UpdatedAt = at
		count++
	}
	return count, nil
}

func (r *NotificationFeedRepository) ListUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time, limit int) ([]*domain.FeedNotification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inScope := func(cat string) bool {
		for _, c := range categories {
			if c == cat {
				return true
			}
		}
		return false
	}
	out := make([]*domain.FeedNotification, 0)
	for _, n := range r.byID {
		if n.UserID != nil || n.ReadAt != nil {
			continue // instance-wide + unread only
		}
		if n.OccurredAt.After(cutoff) || !inScope(n.Category) {
			continue
		}
		cp := *n
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurredAt.After(out[j].OccurredAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *NotificationFeedRepository) CountUnreadForEscalation(ctx context.Context, categories []string, cutoff time.Time) (int, error) {
	items, _ := r.ListUnreadForEscalation(ctx, categories, cutoff, 0)
	return len(items), nil
}

func (r *NotificationFeedRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for id, n := range r.byID {
		if n.OccurredAt.Before(cutoff) {
			delete(r.byID, id)
			count++
		}
	}
	return count, nil
}
