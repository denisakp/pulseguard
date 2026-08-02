package service

import (
	"sort"

	domain "github.com/denisakp/ogoune/internal/domain"
)

type resourceAgg struct {
	up, degraded, down int
}

// aggregatePeriod computes overall uptime % and total downtime seconds for a
// period from daily uptime aggregates, plus a per-resource breakdown. Reuses
// pre-aggregated data only: a resource/day with no aggregate row
// simply is not summed. Resources with no aggregate rows over the period
// contribute zero and are omitted from the breakdown. Pure — no I/O.
func aggregatePeriod(
	resources []*domain.Resource,
	aggs []*domain.UptimeDailyAgg,
	incidentByResource map[string]int,
) (uptimePct float64, downtimeSec int64, breakdown []domain.ReportBreakdownLine) {
	nameByID := make(map[string]string, len(resources))
	intervalByID := make(map[string]int, len(resources))
	for _, r := range resources {
		nameByID[r.ID] = r.Name
		intervalByID[r.ID] = r.Interval
	}

	perRes := make(map[string]*resourceAgg)
	for _, a := range aggs {
		ra, ok := perRes[a.ResourceID]
		if !ok {
			ra = &resourceAgg{}
			perRes[a.ResourceID] = ra
		}
		ra.up += a.Up
		ra.degraded += a.Degraded
		ra.down += a.Down
	}

	var totUp, totCounted int
	for id, ra := range perRes {
		counted := ra.up + ra.degraded + ra.down
		totUp += ra.up
		totCounted += counted
		downtimeSec += int64(ra.down) * int64(intervalByID[id])
		pct := 0.0
		if counted > 0 {
			pct = 100 * float64(ra.up) / float64(counted)
		}
		breakdown = append(breakdown, domain.ReportBreakdownLine{
			Name:      nameByID[id],
			UptimePct: pct,
			Incidents: incidentByResource[id],
		})
	}
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].Name < breakdown[j].Name })

	if totCounted > 0 {
		uptimePct = 100 * float64(totUp) / float64(totCounted)
	}
	return uptimePct, downtimeSec, breakdown
}
