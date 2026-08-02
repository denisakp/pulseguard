package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"github.com/denisakp/ogoune/pkg/agentwire"
)

// Collector produces one metrics frame per call. Behind an interface so the
// stream loop can be unit-tested with a fake, keeping gopsutil off the test path.
type Collector interface {
	Collect(ctx context.Context) (agentwire.Frame, error)
}

// gopsutilCollector reads host metrics via gopsutil. A single failing metric is
// logged and skipped (its field stays zero / the mount is omitted) rather than
// failing the whole frame (FR-014).
type gopsutilCollector struct {
	agentVersion string
}

func newGopsutilCollector(agentVersion string) *gopsutilCollector {
	return &gopsutilCollector{agentVersion: agentVersion}
}

func (c *gopsutilCollector) Collect(ctx context.Context) (agentwire.Frame, error) {
	f := agentwire.Frame{AgentVersion: c.agentVersion}

	if info, err := host.InfoWithContext(ctx); err != nil {
		slog.Debug("collect: host info failed", "error", err)
	} else {
		f.OS = osLabel(info)
	}

	// CPU: percent since the previous call (≈ over the last interval).
	if pcts, err := cpu.PercentWithContext(ctx, 0, false); err != nil || len(pcts) == 0 {
		slog.Debug("collect: cpu failed", "error", err)
	} else {
		f.CPUPct = pcts[0]
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		slog.Debug("collect: mem failed", "error", err)
	} else {
		f.MemPct = vm.UsedPercent
	}

	if counters, err := net.IOCountersWithContext(ctx, false); err != nil || len(counters) == 0 {
		slog.Debug("collect: net failed", "error", err)
	} else {
		f.NetIn = int64(counters[0].BytesRecv)
		f.NetOut = int64(counters[0].BytesSent)
	}

	f.Disks = collectDisks(ctx)

	return f, nil
}

// pseudoFSTypes are non-storage filesystems we never report usage for (kernel
// virtual FS, cgroups, read-only snap images, etc.). Everything else — ext4,
// xfs, btrfs, zfs, vfat, and container/VM filesystems like overlay, virtiofs,
// and 9p — is reported. Using Partitions(all=true) + this denylist (instead of
// Partitions(physical=true)) is what makes disk usage show up inside containers
// and VMs, whose root FS is not a /dev-backed "physical" device.
var pseudoFSTypes = map[string]struct{}{
	"proc": {}, "sysfs": {}, "tmpfs": {}, "devtmpfs": {}, "devpts": {},
	"cgroup": {}, "cgroup2": {}, "mqueue": {}, "debugfs": {}, "tracefs": {},
	"securityfs": {}, "pstore": {}, "bpf": {}, "configfs": {}, "fusectl": {},
	"hugetlbfs": {}, "rpc_pipefs": {}, "binfmt_misc": {}, "autofs": {},
	"nsfs": {}, "ramfs": {}, "squashfs": {}, "efivarfs": {}, "fuse.gvfsd-fuse": {},
	"selinuxfs": {}, "sysctlfs": {}, "cgroupfs": {},
}

// collectDisks returns per-mount usage for real (non-pseudo) filesystems,
// skipping kernel/virtual mounts and anything that errors or reports zero total.
func collectDisks(ctx context.Context) []agentwire.DiskUsage {
	parts, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		slog.Debug("collect: disk partitions failed", "error", err)
		return nil
	}
	var out []agentwire.DiskUsage
	seen := make(map[string]struct{})
	for _, p := range parts {
		if _, pseudo := pseudoFSTypes[p.Fstype]; pseudo {
			continue
		}
		if isSystemMount(p.Mountpoint) {
			continue
		}
		if _, dup := seen[p.Mountpoint]; dup {
			continue
		}
		u, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			slog.Debug("collect: disk usage failed", "mount", p.Mountpoint, "error", err)
			continue
		}
		if u.Total == 0 {
			continue // virtual / empty mount
		}
		seen[p.Mountpoint] = struct{}{}
		out = append(out, agentwire.DiskUsage{Mount: p.Mountpoint, UsedPct: u.UsedPercent})
	}
	return out
}

// isSystemMount reports whether a mountpoint is under a kernel/pseudo tree we
// never report, even if its fstype slips past the denylist.
func isSystemMount(mp string) bool {
	for _, prefix := range []string{"/proc", "/sys", "/dev", "/run"} {
		if mp == prefix || strings.HasPrefix(mp, prefix+"/") {
			return true
		}
	}
	return false
}

func osLabel(info *host.InfoStat) string {
	if info.Platform == "" {
		return info.OS
	}
	if info.PlatformVersion == "" {
		return info.Platform
	}
	return fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion)
}
