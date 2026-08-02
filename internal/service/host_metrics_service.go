package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/port"
	"github.com/denisakp/ogoune/internal/repository"
)

// IngestSample is a decoded, presence-validated agent metric frame. The WS
// handler owns JSON decoding + required-field presence; the service owns
// range validation, timestamp assignment, and persistence.
type IngestSample struct {
	OS           *string
	AgentVersion *string
	CPUPct       float64
	MemPct       float64
	NetIn        int64
	NetOut       int64
	Disks        []domain.DiskUsage
}

// HostMetricsService ingests agent samples, serves history, and runs retention.
type HostMetricsService struct {
	metrics   port.HostMetricsRepository
	hosts     port.HostRepository
	rawWindow time.Duration // keep raw samples within this window (default 48h)
	retention time.Duration // purge samples older than this (default 7d)
	now       func() time.Time
}

func NewHostMetricsService(metrics port.HostMetricsRepository, hosts port.HostRepository, rawWindow, retention time.Duration) *HostMetricsService {
	if rawWindow <= 0 {
		rawWindow = 48 * time.Hour
	}
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &HostMetricsService{
		metrics:   metrics,
		hosts:     hosts,
		rawWindow: rawWindow,
		retention: retention,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SetNow overrides the clock. Tests only.
func (s *HostMetricsService) SetNow(fn func() time.Time) { s.now = fn }

// Ingest validates + clamps a sample, assigns the server timestamp, stores it,
// and updates the host's denormalized snapshot + last_seen_at. A malformed
// sample (NaN/Inf) is rejected and last_seen_at is NOT advanced.
func (s *HostMetricsService) Ingest(ctx context.Context, hostID string, in IngestSample) error {
	if invalidNumber(in.CPUPct) || invalidNumber(in.MemPct) {
		return ErrValidationFailed
	}

	cpu := clampPct(in.CPUPct)
	mem := clampPct(in.MemPct)
	disks := make([]domain.DiskUsage, 0, len(in.Disks))
	worst := 0.0
	hasDisk := false
	for _, d := range in.Disks {
		if invalidNumber(d.UsedPct) {
			return ErrValidationFailed
		}
		p := clampPct(d.UsedPct)
		disks = append(disks, domain.DiskUsage{Mount: d.Mount, UsedPct: p})
		if p > worst {
			worst = p
		}
		hasDisk = true
	}

	now := s.now()
	sample := &domain.HostMetricSample{
		HostID:    hostID,
		SampledAt: now,
		CPUPct:    cpu,
		MemPct:    mem,
		NetIn:     in.NetIn,
		NetOut:    in.NetOut,
		Disks:     disks,
	}
	if err := s.metrics.Insert(ctx, sample); err != nil {
		return err
	}

	snapshot := &domain.Host{
		Base:         domain.Base{ID: hostID},
		OS:           in.OS,
		AgentVersion: in.AgentVersion,
		LastSeenAt:   &now,
		LastCPUPct:   &cpu,
		LastMemPct:   &mem,
		LastNetIn:    &in.NetIn,
		LastNetOut:   &in.NetOut,
		LastDisks:    disks,
	}
	if hasDisk {
		w := worst
		snapshot.LastDiskPct = &w
	}
	if err := s.hosts.UpdateSnapshot(ctx, snapshot); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrHostNotFound
		}
		return err
	}
	return nil
}

// History returns a host's samples in [from,to], chronological. When both bounds
// are zero, defaults to the last hour.
func (s *HostMetricsService) History(ctx context.Context, hostID string, from, to time.Time) ([]*domain.HostMetricSample, error) {
	if from.IsZero() && to.IsZero() {
		to = s.now()
		from = to.Add(-1 * time.Hour)
	}
	if to.IsZero() {
		to = s.now()
	}
	return s.metrics.ListInRange(ctx, hostID, from, to)
}

// RunRetention decimates samples older than the raw window to ≤1/min, then
// purges samples older than the retention window.
func (s *HostMetricsService) RunRetention(ctx context.Context) error {
	now := s.now()
	if _, err := s.metrics.Decimate(ctx, now.Add(-s.rawWindow)); err != nil {
		return err
	}
	if _, err := s.metrics.DeleteOlderThan(ctx, now.Add(-s.retention)); err != nil {
		return err
	}
	return nil
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func invalidNumber(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}
