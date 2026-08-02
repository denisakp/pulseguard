import { computed, ref } from 'vue'
import { useHostStore } from '@/stores/hostStore'
import { listMonitors } from '@/services/hostsService'
import type { Host, MonitorSummary } from '@/types'

/**
 * Enriched Hosts feed. Wraps `useHostStore` and derives, per host:
 *   - `serviceCount` — number of monitors linked to the host, derived from the
 *     v1 monitors list (the only monitor read that carries `host_id`).
 *   - `worstLoad` — max(cpu, mem, disk), used for trouble-first sorting.
 *
 * The list is sorted trouble-first: offline hosts, then by worstLoad desc, then
 * by name (case-insensitive) so operators see the hosts needing attention first.
 */
export interface EnrichedHost extends Host {
  serviceCount: number
  worstLoad: number
}

function worstLoadOf(h: Host): number {
  return Math.max(h.lastCpuPct ?? 0, h.lastMemPct ?? 0, h.lastDiskPct ?? 0)
}

export function useHosts() {
  const hostStore = useHostStore()
  const monitors = ref<MonitorSummary[]>([])

  /** monitor host links → count per host id. */
  const serviceCounts = computed<Record<string, number>>(() => {
    const counts: Record<string, number> = {}
    for (const m of monitors.value) {
      if (m.hostId) counts[m.hostId] = (counts[m.hostId] ?? 0) + 1
    }
    return counts
  })

  const hosts = computed<EnrichedHost[]>(() => {
    const enriched = hostStore.hosts.map((h) => ({
      ...h,
      serviceCount: serviceCounts.value[h.id] ?? 0,
      worstLoad: worstLoadOf(h),
    }))
    enriched.sort((a, b) => {
      // offline first
      if (a.online !== b.online) return a.online ? 1 : -1
      // then worst load desc
      if (a.worstLoad !== b.worstLoad) return b.worstLoad - a.worstLoad
      // then name asc
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
    })
    return enriched
  })

  const loading = computed(() => hostStore.loading)
  const error = computed(() => hostStore.error)
  const count = computed(() => hostStore.count)

  async function fetch() {
    const [, mons] = await Promise.all([hostStore.fetch(), loadMonitors()])
    void mons
  }

  async function loadMonitors() {
    try {
      monitors.value = await listMonitors()
    } catch {
      // Service counts are best-effort; a monitors failure must not blank hosts.
      monitors.value = []
    }
  }

  return {
    hosts,
    loading,
    error,
    count,
    fetch,
    startPolling: hostStore.startPolling,
    stopPolling: hostStore.stopPolling,
  }
}
