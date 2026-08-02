import { ref, watch, toValue, type MaybeRefOrGetter, type Ref } from 'vue'
import { getHostMetrics } from '@/services/hostsService'
import type { HostMetricRange, HostMetricSample } from '@/types'

/**
 * useHostMetrics — resolves a `from`/`to` window from a
 * reactive `range` preset (1h/6h/24h/7d back from now), fetches the host's
 * metric samples via `hostsService.getHostMetrics`, and shapes them into the
 * series the detail-page charts consume.
 *
 * An empty result is a valid empty state (`isEmpty = true`), NOT an error.
 * Refetches whenever the range changes.
 */

/** One combined point of the scalar series (CPU / RAM / network). */
export interface HostMetricPoint {
  t: string
  cpuPct: number
  memPct: number
  netIn: number
  netOut: number
}

/** One point of a per-mount disk-usage series. */
export interface DiskPoint {
  t: string
  usedPct: number
}

/** Milliseconds spanned by each range preset. */
const RANGE_MS: Record<HostMetricRange, number> = {
  '1h': 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
}

/** Resolve the [from, to] window for a range preset ending at `now`. */
export function rangeWindow(range: HostMetricRange, now: Date = new Date()): { from: Date; to: Date } {
  const to = new Date(now.getTime())
  const from = new Date(now.getTime() - RANGE_MS[range])
  return { from, to }
}

export interface UseHostMetrics {
  points: Ref<HostMetricPoint[]>
  disksByMount: Ref<Record<string, DiskPoint[]>>
  loading: Ref<boolean>
  error: Ref<string | null>
  isEmpty: Ref<boolean>
  reload: () => Promise<void>
}

export function useHostMetrics(
  hostId: MaybeRefOrGetter<string>,
  range: Ref<HostMetricRange>,
): UseHostMetrics {
  const points = ref<HostMetricPoint[]>([])
  const disksByMount = ref<Record<string, DiskPoint[]>>({})
  const loading = ref(false)
  const error = ref<string | null>(null)
  const isEmpty = ref(false)

  function shape(samples: HostMetricSample[]) {
    points.value = samples.map((s) => ({
      t: s.sampledAt,
      cpuPct: s.cpuPct,
      memPct: s.memPct,
      netIn: s.netIn,
      netOut: s.netOut,
    }))
    const byMount: Record<string, DiskPoint[]> = {}
    for (const s of samples) {
      for (const d of s.disks) {
        ;(byMount[d.mount] ??= []).push({ t: s.sampledAt, usedPct: d.usedPct })
      }
    }
    disksByMount.value = byMount
    isEmpty.value = samples.length === 0
  }

  async function reload(): Promise<void> {
    const id = toValue(hostId)
    if (!id) return
    loading.value = true
    error.value = null
    try {
      const { from, to } = rangeWindow(range.value)
      const samples = await getHostMetrics(id, from, to)
      shape(samples)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load host metrics'
      points.value = []
      disksByMount.value = {}
      isEmpty.value = false
    } finally {
      loading.value = false
    }
  }

  // Initial fetch + refetch on every range change.
  watch(range, reload, { immediate: true })

  return { points, disksByMount, loading, error, isEmpty, reload }
}
