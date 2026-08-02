// Package service — public status aggregator.
package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/port"
)

// PublicStatusService computes the snapshot exposed at GET /status.
type PublicStatusService struct {
	resources       port.ResourceRepository
	components      port.ComponentRepository
	incidents       port.IncidentRepository
	maintenances    port.MaintenanceRepository
	uptimeAggs      port.UptimeDailyAggRepository
	settings        port.StatusPageSettingsRepository
	incidentUpdates port.IncidentUpdateRepository
	clock           func() time.Time
}

func NewPublicStatusService(
	resources port.ResourceRepository,
	components port.ComponentRepository,
	incidents port.IncidentRepository,
	maintenances port.MaintenanceRepository,
	uptimeAggs port.UptimeDailyAggRepository,
	settings port.StatusPageSettingsRepository,
	incidentUpdates port.IncidentUpdateRepository,
) *PublicStatusService {
	return &PublicStatusService{
		resources:       resources,
		components:      components,
		incidents:       incidents,
		maintenances:    maintenances,
		uptimeAggs:      uptimeAggs,
		settings:        settings,
		incidentUpdates: incidentUpdates,
		clock:           time.Now,
	}
}

// GetResourceWindows returns the per-resource detail panel payload —
// 24h / 7d / 30d / 90d windowed uptime + 30-day daily series + last 5
// incidents on that resource. Backs the OverallUptimePanel (US4).
func (s *PublicStatusService) GetResourceWindows(ctx context.Context, resourceID string) (*dto.PublicResourceWindowsResponse, error) {
	r, err := s.resources.FindByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	now := s.clock().UTC()
	today := truncDayUTC(now)

	out := &dto.PublicResourceWindowsResponse{
		ID:              r.ID,
		Name:            r.Name,
		Windows:         map[string]dto.PublicWindowStats{},
		Daily30d:        []dto.PublicRibbonEntry{},
		RecentIncidents: []dto.PublicIncidentSummary{},
	}

	// Load 90 days of aggregates once; reuse the slice for every window.
	aggs90, err := s.uptimeAggs.FindForResource(ctx, r.ID, today.AddDate(0, 0, -89), today)
	if err != nil {
		return nil, fmt.Errorf("public_status: load resource aggs: %w", err)
	}
	byDay := map[string]float64{}
	for _, a := range aggs90 {
		byDay[a.Day.UTC().Format("2006-01-02")] = a.UptimeRatio
	}

	// Recent incidents on this resource (max 5, newest-first).
	recent, err := s.incidents.FindByResource(ctx, r.ID, 5, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: load recent incidents: %w", err)
	}
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].StartedAt.After(recent[j].StartedAt) })
	for _, inc := range recent {
		sev := dto.PublicSeverityMinor
		if inc.ResolvedAt == nil {
			sev = dto.PublicSeverityMajor
		}
		out.RecentIncidents = append(out.RecentIncidents, dto.PublicIncidentSummary{
			ID:         inc.ID,
			Title:      incidentTitle(inc),
			StartedAt:  inc.StartedAt,
			ResolvedAt: inc.ResolvedAt,
			Severity:   sev,
			ResourceID: inc.ResourceID,
		})
	}

	// 30-day series for the chart.
	from30 := today.AddDate(0, 0, -29)
	out.Daily30d = buildRibbon(from30, today, byDay)

	// 4 windowed cards: incidents-per-window are counted by start time.
	type win struct {
		key  string
		days int
	}
	for _, w := range []win{
		{"24h", 1}, {"7d", 7}, {"30d", 30}, {"90d", 90},
	} {
		fromW := today.AddDate(0, 0, -(w.days - 1))
		windowRibbon := buildRibbon(fromW, today, byDay)
		ratio := averageRibbon(windowRibbon)
		// 24h is special-cased: use the latest single day's value if known.
		if w.key == "24h" {
			if v, ok := byDay[today.Format("2006-01-02")]; ok {
				ratio = v
			}
		}
		incCount := 0
		for _, inc := range recent {
			if !inc.StartedAt.Before(fromW) && !inc.StartedAt.After(today.Add(24*time.Hour)) {
				incCount++
			}
		}
		out.Windows[w.key] = dto.PublicWindowStats{
			UptimeRatio: ratio,
			Incidents:   incCount,
		}
	}
	return out, nil
}

// GetIncidentDetail returns the incident plus its lifecycle update timeline
// for the public detail page (US7).
func (s *PublicStatusService) GetIncidentDetail(ctx context.Context, incidentID string) (*dto.PublicIncidentDetail, error) {
	inc, err := s.incidents.FindByID(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	sev := dto.PublicSeverityMinor
	if inc.ResolvedAt == nil {
		sev = dto.PublicSeverityMajor
	}
	out := &dto.PublicIncidentDetail{
		ID:         inc.ID,
		Title:      incidentTitle(inc),
		Severity:   sev,
		StartedAt:  inc.StartedAt,
		ResolvedAt: inc.ResolvedAt,
		ResourceID: inc.ResourceID,
		Updates:    []dto.PublicIncidentUpdate{},
	}
	if s.incidentUpdates != nil {
		rows, err := s.incidentUpdates.ListByIncident(ctx, incidentID)
		if err != nil {
			return nil, fmt.Errorf("public_status: list incident updates: %w", err)
		}
		out.Updates = make([]dto.PublicIncidentUpdate, 0, len(rows))
		for _, u := range rows {
			out.Updates = append(out.Updates, dto.PublicIncidentUpdate{
				ID:       u.ID,
				Status:   dto.PublicIncidentUpdateStatus(u.Status),
				Message:  u.Message,
				PostedAt: u.PostedAt,
			})
		}
	}
	return out, nil
}

// SetClock overrides the wall clock — used by tests.
func (s *PublicStatusService) SetClock(c func() time.Time) { s.clock = c }

// GetCurrent returns the verdict + components + standalone resources +
// current-month incidents per spec 060 contract.
func (s *PublicStatusService) GetCurrent(ctx context.Context) (*dto.PublicStatus, error) {
	now := s.clock().UTC()

	// Load all active resources + components.
	components, err := s.components.List(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: list components: %w", err)
	}
	resources, err := s.resources.FindActive(ctx, 5000, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: list resources: %w", err)
	}

	// Index resources by component_id.
	byComponent := map[string][]*domain.Resource{}
	var standalone []*domain.Resource
	for _, r := range resources {
		if r.ComponentID != nil && *r.ComponentID != "" {
			byComponent[*r.ComponentID] = append(byComponent[*r.ComponentID], r)
		} else {
			standalone = append(standalone, r)
		}
	}

	// Build 90-day ribbon window once: [today-89 ... today].
	to := truncDayUTC(now)
	from := to.AddDate(0, 0, -89)

	// Bulk-fetch ribbons by resource id, grouped per resource.
	allIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		allIDs = append(allIDs, r.ID)
	}
	aggs, err := s.uptimeAggs.FindRange(ctx, allIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("public_status: load uptime aggs: %w", err)
	}
	aggsByResource := map[string]map[string]float64{}
	for _, a := range aggs {
		key := a.Day.UTC().Format("2006-01-02")
		m := aggsByResource[a.ResourceID]
		if m == nil {
			m = map[string]float64{}
			aggsByResource[a.ResourceID] = m
		}
		m[key] = a.UptimeRatio
	}

	// Build per-resource summaries.
	resourceSummary := func(r *domain.Resource) dto.PublicResource {
		state := s.resourceState(ctx, r, now)
		ribbon := buildRibbon(from, to, aggsByResource[r.ID])
		return dto.PublicResource{
			ID:             r.ID,
			Name:           r.Name,
			Host:           r.Target,
			CurrentState:   state,
			Uptime90dRatio: averageRibbon(ribbon),
			UptimeRibbon:   ribbon,
		}
	}

	// Components in order (sorted by Name).
	sort.SliceStable(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	out := &dto.PublicStatus{
		GeneratedAt:           now,
		Components:            make([]dto.PublicComponent, 0, len(components)),
		StandaloneResources:   make([]dto.PublicResource, 0, len(standalone)),
		CurrentMonthIncidents: []dto.PublicIncidentSummary{},
	}

	for _, c := range components {
		members := byComponent[c.ID]
		sort.SliceStable(members, func(i, j int) bool { return members[i].Name < members[j].Name })
		resSummaries := make([]dto.PublicResource, 0, len(members))
		for _, r := range members {
			resSummaries = append(resSummaries, resourceSummary(r))
		}
		out.Components = append(out.Components, dto.PublicComponent{
			ID:              c.ID,
			Name:            c.Name,
			AggregatedState: aggregateComponentState(resSummaries),
			Resources:       resSummaries,
		})
	}

	sort.SliceStable(standalone, func(i, j int) bool { return standalone[i].Name < standalone[j].Name })
	for _, r := range standalone {
		out.StandaloneResources = append(out.StandaloneResources, resourceSummary(r))
	}

	out.Verdict = computeVerdict(out.Components, out.StandaloneResources)
	out.Branding = s.loadBranding(ctx)
	out.UptimeWindow = s.loadUptimeWindow(ctx, now)

	// Current month incidents.
	monthIncidents, err := s.loadMonthIncidents(ctx, now)
	if err != nil {
		return nil, err
	}
	out.CurrentMonthIncidents = monthIncidents

	return out, nil
}

// resourceState maps the domain status into the public aggregated state,
// with maintenance overriding "down" when an active window covers the
// resource (FR-005).
func (s *PublicStatusService) resourceState(ctx context.Context, r *domain.Resource, now time.Time) dto.PublicAggregatedState {
	// Maintenance override first.
	if s.maintenances != nil {
		windows, err := s.maintenances.FindActiveForResource(ctx, r.ID, now)
		if err == nil && len(windows) > 0 {
			return dto.PublicStateMaintenance
		}
	}
	switch r.Status {
	case domain.StatusUp:
		return dto.PublicStateUp
	case domain.StatusDown, domain.StatusError:
		return dto.PublicStateDown
	case domain.StatusFlapping, domain.StatusWarn:
		return dto.PublicStateDegraded
	case domain.StatusPaused:
		return dto.PublicStateMaintenance
	default:
		return dto.PublicStateUnknown
	}
}

// GetIncidents returns incidents grouped by year-month (newest-first) over
// the [from, to] window, optionally filtered by component. Used by GET
// /status/incidents (US2 — spec 060).
func (s *PublicStatusService) GetIncidents(ctx context.Context, from, to time.Time, componentID string) (*dto.PublicIncidentsResponse, error) {
	now := s.clock().UTC()
	if from.IsZero() {
		from = now.AddDate(0, 0, -90)
	}
	if to.IsZero() {
		to = now
	}
	if from.After(to) {
		return nil, fmt.Errorf("public_status: from > to")
	}

	// Resolve component membership if a filter is requested.
	var resourceFilter map[string]struct{}
	if componentID != "" {
		members, err := s.resources.FindByComponentID(ctx, componentID)
		if err != nil {
			return nil, fmt.Errorf("public_status: load component resources: %w", err)
		}
		resourceFilter = map[string]struct{}{}
		for _, r := range members {
			resourceFilter[r.ID] = struct{}{}
		}
	}

	// MVP page size: 500 incidents max — backed by paginated repo call.
	rows, err := s.incidents.List(ctx, 500, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: list incidents: %w", err)
	}

	monthMap := map[string][]dto.PublicIncidentSummary{}
	var total int
	for _, inc := range rows {
		if inc.StartedAt.Before(from) || inc.StartedAt.After(to) {
			continue
		}
		if resourceFilter != nil {
			if _, ok := resourceFilter[inc.ResourceID]; !ok {
				continue
			}
		}
		key := inc.StartedAt.UTC().Format("2006-01")
		sev := dto.PublicSeverityMinor
		if inc.ResolvedAt == nil {
			sev = dto.PublicSeverityMajor
		}
		monthMap[key] = append(monthMap[key], dto.PublicIncidentSummary{
			ID:          inc.ID,
			Title:       incidentTitle(inc),
			StartedAt:   inc.StartedAt,
			ResolvedAt:  inc.ResolvedAt,
			Severity:    sev,
			ComponentID: componentID,
			ResourceID:  inc.ResourceID,
		})
		total++
	}

	// Sort each month newest-first.
	for _, list := range monthMap {
		sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	}
	// Months newest-first.
	keys := make([]string, 0, len(monthMap))
	for k := range monthMap {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	months := make([]dto.PublicIncidentMonth, 0, len(keys))
	for _, k := range keys {
		months = append(months, dto.PublicIncidentMonth{
			YearMonth: k,
			Count:     len(monthMap[k]),
			Incidents: monthMap[k],
		})
	}

	return &dto.PublicIncidentsResponse{
		GeneratedAt: now,
		Total:       total,
		Months:      months,
	}, nil
}

// GetUptime returns per-day uptime ratios over [from, to], optionally
// scoped to one component. When component_id is empty, the daily ratio
// is the mean across all active resources; otherwise across the
// component's resources only.
//
// The handler enforces the 1-year max-span guard (US3, FR-026).
func (s *PublicStatusService) GetUptime(ctx context.Context, componentID string, from, to time.Time) (*dto.PublicUptimeResponse, error) {
	now := s.clock().UTC()
	if from.IsZero() {
		from = now.AddDate(0, 0, -89)
	}
	if to.IsZero() {
		to = now
	}
	if from.After(to) {
		return nil, fmt.Errorf("public_status: from > to")
	}
	fromDay := truncDayUTC(from)
	toDay := truncDayUTC(to)

	// Resolve target resources.
	var resourceIDs []string
	if componentID != "" {
		members, err := s.resources.FindByComponentID(ctx, componentID)
		if err != nil {
			return nil, fmt.Errorf("public_status: load component resources: %w", err)
		}
		for _, r := range members {
			resourceIDs = append(resourceIDs, r.ID)
		}
	} else {
		all, err := s.resources.FindActive(ctx, 5000, 0)
		if err != nil {
			return nil, fmt.Errorf("public_status: list resources: %w", err)
		}
		for _, r := range all {
			resourceIDs = append(resourceIDs, r.ID)
		}
	}

	aggs, err := s.uptimeAggs.FindRange(ctx, resourceIDs, fromDay, toDay)
	if err != nil {
		return nil, fmt.Errorf("public_status: load uptime aggs: %w", err)
	}

	// Group per day: accumulate ratio + samples across resources.
	type dayBucket struct {
		ratioSum   float64
		samplesSum int
		count      int
	}
	buckets := map[string]*dayBucket{}
	for _, a := range aggs {
		key := a.Day.UTC().Format("2006-01-02")
		b := buckets[key]
		if b == nil {
			b = &dayBucket{}
			buckets[key] = b
		}
		b.ratioSum += a.UptimeRatio
		b.samplesSum += a.Samples
		b.count++
	}

	// Pull current-month incidents to attach per-day incident counts.
	incidents, err := s.incidents.List(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: list incidents: %w", err)
	}
	inComponent := map[string]struct{}{}
	if componentID != "" {
		for _, id := range resourceIDs {
			inComponent[id] = struct{}{}
		}
	}
	incidentsByDay := map[string][]dto.PublicIncidentSummary{}
	for _, inc := range incidents {
		if inc.StartedAt.Before(fromDay) || inc.StartedAt.After(toDay.Add(24*time.Hour)) {
			continue
		}
		if componentID != "" {
			if _, ok := inComponent[inc.ResourceID]; !ok {
				continue
			}
		}
		key := inc.StartedAt.UTC().Format("2006-01-02")
		sev := dto.PublicSeverityMinor
		if inc.ResolvedAt == nil {
			sev = dto.PublicSeverityMajor
		}
		incidentsByDay[key] = append(incidentsByDay[key], dto.PublicIncidentSummary{
			ID:         inc.ID,
			Title:      incidentTitle(inc),
			StartedAt:  inc.StartedAt,
			ResolvedAt: inc.ResolvedAt,
			Severity:   sev,
			ResourceID: inc.ResourceID,
		})
	}

	// Materialize one entry per day in the window.
	days := make([]dto.PublicUptimeDay, 0)
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		related := incidentsByDay[key]
		entry := dto.PublicUptimeDay{
			Day:              key,
			Incidents:        len(related),
			RelatedIncidents: related,
		}
		if entry.RelatedIncidents == nil {
			entry.RelatedIncidents = []dto.PublicIncidentSummary{}
		}
		if b, ok := buckets[key]; ok && b.count > 0 {
			entry.UptimeRatio = b.ratioSum / float64(b.count)
			entry.Samples = b.samplesSum
			// Approximation: convert the day-level ratio into an equivalent
			// downtime span over 24h. Surfaces a human-friendly figure for
			// the tooltip without needing per-incident duration aggregation.
			entry.DowntimeSeconds = int((1.0 - entry.UptimeRatio) * 86400.0)
		}
		days = append(days, entry)
	}

	return &dto.PublicUptimeResponse{
		GeneratedAt: now,
		Days:        days,
	}, nil
}

func (s *PublicStatusService) loadUptimeWindow(ctx context.Context, now time.Time) dto.PublicUptimeWindow {
	today := truncDayUTC(now)
	w := dto.PublicUptimeWindow{LatestDay: today.Format("2006-01-02")}
	if s.uptimeAggs == nil {
		return w
	}
	earliest, err := s.uptimeAggs.FindEarliestDay(ctx)
	if err == nil && !earliest.IsZero() {
		w.EarliestDay = earliest.UTC().Format("2006-01-02")
	}
	return w
}

func (s *PublicStatusService) loadBranding(ctx context.Context) dto.PublicBranding {
	if s.settings == nil {
		return dto.PublicBranding{Name: "Status Page"}
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil || cfg == nil {
		return dto.PublicBranding{Name: "Status Page"}
	}
	name := cfg.Name
	if name == "" {
		name = "Status Page"
	}
	return dto.PublicBranding{
		Name:           name,
		HomepageURL:    cfg.HomepageURL,
		LogoURLLight:   cfg.LogoURLLight,
		LogoURLDark:    cfg.LogoURLDark,
		FaviconURL:     cfg.FaviconURL,
		PrimaryColor:   cfg.PrimaryColor,
		UmamiWebsiteID: cfg.UmamiWebsiteID,
		UmamiScriptURL: cfg.UmamiScriptURL,
	}
}

// loadMonthIncidents returns the public-shape incidents that started in the
// current month and resolved (or are still open).
func (s *PublicStatusService) loadMonthIncidents(ctx context.Context, now time.Time) ([]dto.PublicIncidentSummary, error) {
	// Cheap heuristic for the MVP: pull last 200 incidents, filter by start month.
	rows, err := s.incidents.List(ctx, 200, 0)
	if err != nil {
		return nil, fmt.Errorf("public_status: list incidents: %w", err)
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	out := make([]dto.PublicIncidentSummary, 0)
	for _, inc := range rows {
		if inc.StartedAt.Before(monthStart) || !inc.StartedAt.Before(monthEnd) {
			continue
		}
		sev := dto.PublicSeverityMinor
		if inc.ResolvedAt == nil {
			sev = dto.PublicSeverityMajor
		}
		out = append(out, dto.PublicIncidentSummary{
			ID:         inc.ID,
			Title:      incidentTitle(inc),
			StartedAt:  inc.StartedAt,
			ResolvedAt: inc.ResolvedAt,
			Severity:   sev,
			ResourceID: inc.ResourceID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

func incidentTitle(inc *domain.Incident) string {
	if inc.Cause != "" {
		return inc.Cause
	}
	return "Incident on " + inc.Resource.Name
}

// computeVerdict applies FR-002 with the clarified promotion rule:
// Major if ≥ 1 component fully down OR ≥ 50% components in degraded/down,
// else partial if any non-up resource exists, else operational.
func computeVerdict(components []dto.PublicComponent, standalone []dto.PublicResource) dto.PublicVerdict {
	totalComponents := len(components)
	componentDown := 0
	componentDegradedOrDown := 0
	hasNonUp := false

	for _, c := range components {
		switch c.AggregatedState {
		case dto.PublicStateDown:
			componentDown++
			componentDegradedOrDown++
			hasNonUp = true
		case dto.PublicStateDegraded:
			componentDegradedOrDown++
			hasNonUp = true
		case dto.PublicStateMaintenance, dto.PublicStateUnknown:
			hasNonUp = true
		}
	}
	for _, r := range standalone {
		switch r.CurrentState {
		case dto.PublicStateDown:
			hasNonUp = true
			componentDown++
		case dto.PublicStateDegraded:
			hasNonUp = true
		case dto.PublicStateMaintenance, dto.PublicStateUnknown:
			hasNonUp = true
		}
	}

	// 50% rule requires at least 2 affected components — a single degraded
	// component on a 1-component page is "partial", not "major".
	majorTrigger := componentDown >= 1 ||
		(totalComponents >= 2 && componentDegradedOrDown >= 2 && componentDegradedOrDown*2 >= totalComponents)

	switch {
	case majorTrigger:
		return dto.PublicVerdict{
			Status: dto.VerdictMajorOutage,
			Label:  "Major Outage",
			Color:  "red",
		}
	case hasNonUp:
		return dto.PublicVerdict{
			Status: dto.VerdictPartialDegradation,
			Label:  "Partial Degradation",
			Color:  "orange",
		}
	default:
		return dto.PublicVerdict{
			Status: dto.VerdictOperational,
			Label:  "All Systems Operational",
			Color:  "green",
		}
	}
}

// aggregateComponentState — max(severity) with maintenance override per FR-003.
// down > degraded > maintenance > unknown > up.
func aggregateComponentState(resources []dto.PublicResource) dto.PublicAggregatedState {
	if len(resources) == 0 {
		return dto.PublicStateUnknown
	}
	rank := map[dto.PublicAggregatedState]int{
		dto.PublicStateUp:          0,
		dto.PublicStateUnknown:     1,
		dto.PublicStateMaintenance: 2,
		dto.PublicStateDegraded:    3,
		dto.PublicStateDown:        4,
	}
	worst := dto.PublicStateUp
	for _, r := range resources {
		if rank[r.CurrentState] > rank[worst] {
			worst = r.CurrentState
		}
	}
	return worst
}

func buildRibbon(from, to time.Time, byDay map[string]float64) []dto.PublicRibbonEntry {
	out := make([]dto.PublicRibbonEntry, 0, 90)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		entry := dto.PublicRibbonEntry{Day: key}
		if v, ok := byDay[key]; ok {
			r := v
			entry.Ratio = &r
		}
		out = append(out, entry)
	}
	return out
}

// averageRibbon computes the mean over days with known data. Days with no
// data are excluded — a freshly added resource must read 100% on its 4 days
// of history, not 4.4% (its 4 days divided by 90).
func averageRibbon(ribbon []dto.PublicRibbonEntry) float64 {
	sum := 0.0
	count := 0
	for _, e := range ribbon {
		if e.Ratio == nil {
			continue
		}
		sum += *e.Ratio
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func truncDayUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
