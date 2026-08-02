// Package service — daily uptime aggregator.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
)

// UptimeAggregator recomputes per-resource per-day uptime ratios from raw
// monitoring_activity samples and upserts them into uptime_daily_agg.
//
// Cadence: invoked every 5 minutes by the platform scheduler. Each tick
// recomputes the current UTC day plus the previous day (in case late samples
// land after midnight). Idempotent — re-running over the same window
// produces identical rows.
type UptimeAggregator struct {
	resources  port.ResourceRepository
	activities port.MonitoringActivityRepository
	aggs       port.UptimeDailyAggRepository
	clock      func() time.Time
}

func NewUptimeAggregator(
	resources port.ResourceRepository,
	activities port.MonitoringActivityRepository,
	aggs port.UptimeDailyAggRepository,
) *UptimeAggregator {
	return &UptimeAggregator{
		resources:  resources,
		activities: activities,
		aggs:       aggs,
		clock:      time.Now,
	}
}

func (a *UptimeAggregator) SetClock(c func() time.Time) { a.clock = c }

// RunOnce recomputes today + yesterday for every active resource.
func (a *UptimeAggregator) RunOnce(ctx context.Context) error {
	now := a.clock().UTC()
	today := truncDayUTC(now)
	yesterday := today.AddDate(0, 0, -1)

	resources, err := a.resources.FindActive(ctx, 5000, 0)
	if err != nil {
		return fmt.Errorf("uptime_aggregator: list active: %w", err)
	}
	for _, r := range resources {
		for _, day := range []time.Time{yesterday, today} {
			if err := a.recompute(ctx, r.ID, day, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *UptimeAggregator) recompute(ctx context.Context, resourceID string, day, computedAt time.Time) error {
	dayStart := truncDayUTC(day)
	dayEnd := dayStart.Add(24 * time.Hour)

	// Pull a generous page; in practice ≤ 1440 samples/day at the slowest
	// 1-minute cadence. The activity table is append-only so two passes
	// over a stable window are deterministic.
	rows, err := a.activities.FindByResourceID(ctx, resourceID, 5000, 0)
	if err != nil {
		return fmt.Errorf("uptime_aggregator: load activities: %w", err)
	}

	var up, degraded, down, samples int
	for _, act := range rows {
		ts := act.CreatedAt
		if ts.Before(dayStart) || !ts.Before(dayEnd) {
			continue
		}
		samples++
		switch {
		case act.IsMaintenance:
			// Maintenance windows do not count as down or up — they are
			// excluded from the ratio entirely.
			samples--
		case act.Success:
			up++
		default:
			down++
		}
	}

	if samples == 0 {
		// No data for that day — skip; a NULL row would be misleading.
		// The public ribbon renders "no data" for missing days.
		return nil
	}

	ratio := float64(up) / float64(samples)

	return a.aggs.Upsert(ctx, &domain.UptimeDailyAgg{
		ResourceID:  resourceID,
		Day:         dayStart,
		Samples:     samples,
		Up:          up,
		Degraded:    degraded,
		Down:        down,
		UptimeRatio: ratio,
		ComputedAt:  computedAt,
	})
}
