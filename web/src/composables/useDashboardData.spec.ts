import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const resourcesRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<unknown[]>([])
})
const incidentsRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<unknown[]>([])
})

const loadResourcesMock = vi.fn(async () => {})
const fetchIncidentsMock = vi.fn(async () => {})

vi.mock('@/stores/resourceStore', () => ({
  useResourceStore: () => ({
    get resources() {
      return resourcesRef.value
    },
    loadResources: loadResourcesMock,
  }),
}))

vi.mock('@/stores/incidentStore', () => ({
  useIncidentStore: () => ({
    get incidents() {
      return incidentsRef.value
    },
    fetchIncidents: fetchIncidentsMock,
  }),
}))

import { useDashboardData } from './useDashboardData'
import { ref } from 'vue'
import type {
  DashboardRefreshInterval,
  DashboardScope,
  DashboardTimeRange,
} from '@/types'

function setup(overrides: {
  scope?: DashboardScope
  range?: DashboardTimeRange
  refresh?: DashboardRefreshInterval
} = {}) {
  const scope = ref<DashboardScope>(
    overrides.scope ?? { mode: 'tag', payload: { tagIds: [] } },
  )
  const timeRange = ref<DashboardTimeRange>(overrides.range ?? '24h')
  const refreshInterval = ref<DashboardRefreshInterval>(overrides.refresh ?? 'off')
  return useDashboardData({ scope, timeRange, refreshInterval })
}

describe('useDashboardData', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resourcesRef.value = [
      {
        id: 'r1',
        name: 'api',
        type: 'http',
        status: 'up',
        tags: [{ id: 't1', name: 'prod' }],
        uptime_30d: 0.99,
      },
      {
        id: 'r2',
        name: 'db',
        type: 'tcp',
        status: 'down',
        tags: [{ id: 't1', name: 'prod' }],
        uptime_30d: 0.85,
      },
      {
        id: 'r3',
        name: 'cache',
        type: 'http',
        status: 'up',
        tags: [{ id: 't2', name: 'dev' }],
        uptime_30d: 0.999,
      },
    ]
    incidentsRef.value = [
      { id: 'i1', resource_id: 'r2', cause: 'down', started_at: new Date().toISOString(), resolved_at: null },
      {
        id: 'i2',
        resource_id: 'r1',
        cause: 'old',
        started_at: new Date(Date.now() - 60 * 24 * 60 * 60 * 1000).toISOString(),
        resolved_at: new Date(Date.now() - 60 * 24 * 60 * 60 * 1000 + 1000).toISOString(),
      },
    ]
    loadResourcesMock.mockClear()
    fetchIncidentsMock.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('resolves tag-scoped resources from the store', () => {
    const d = setup({ scope: { mode: 'tag', payload: { tagIds: ['t1'] } } })
    expect(d.resources.value.map((r) => r.id)).toEqual(['r1', 'r2'])
  })

  it('resolves type-scoped resources', () => {
    const d = setup({ scope: { mode: 'type', payload: { types: ['http'] } } })
    expect(d.resources.value.map((r) => r.id).sort()).toEqual(['r1', 'r3'])
  })

  it('exposes tombstone entries for manual scope referencing deleted resources', () => {
    const d = setup({
      scope: { mode: 'manual', payload: { resourceIds: ['r1', 'ghost-id'] } },
    })
    const resolved = d.resolved.value
    expect(resolved.find((r) => r.id === 'r1')!.resource).not.toBeNull()
    expect(resolved.find((r) => r.id === 'ghost-id')!.resource).toBeNull()
    expect(d.resources.value.length).toBe(1) // live only
  })

  it('aggregateStatus is degraded when at least one resource is down', () => {
    const d = setup({ scope: { mode: 'type', payload: { types: ['http', 'tcp'] } } })
    expect(d.aggregateStatus.value).toBe('degraded')
  })

  it('aggregateStatus is operational when all up', () => {
    const d = setup({ scope: { mode: 'type', payload: { types: ['http'] } } })
    expect(d.aggregateStatus.value).toBe('operational')
  })

  it('incidents are filtered to scope resources', () => {
    const d = setup({ scope: { mode: 'type', payload: { types: ['http'] } } })
    // r1, r3 are http; i1 is for r2 (tcp); i2 is for r1 → match.
    expect(d.incidents.value.map((i) => i.id)).toEqual(['i2'])
  })

  it('incidentsInRange filters by time window', () => {
    const d = setup({
      scope: { mode: 'type', payload: { types: ['tcp'] } },
      range: '24h',
    })
    // i1 is recent (now), i2 is 60 days old → only i1
    expect(d.incidentsInRange.value.map((i) => i.id)).toEqual(['i1'])
  })

  it('refresh triggers both store loaders', async () => {
    const d = setup()
    await d.refresh()
    expect(loadResourcesMock).toHaveBeenCalled()
    expect(fetchIncidentsMock).toHaveBeenCalled()
  })

  it('start polls every refresh interval when visible', async () => {
    vi.useFakeTimers()
    const d = setup({ refresh: '30s' })
    d.start()
    await Promise.resolve()
    expect(loadResourcesMock).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(30_000)
    expect(loadResourcesMock).toHaveBeenCalledTimes(2)
    d.stop()
    vi.advanceTimersByTime(30_000)
    expect(loadResourcesMock).toHaveBeenCalledTimes(2)
  })

  it('refresh=off does not start a timer', async () => {
    vi.useFakeTimers()
    const d = setup({ refresh: 'off' })
    d.start()
    await Promise.resolve()
    expect(loadResourcesMock).toHaveBeenCalledTimes(1) // initial refresh only
    vi.advanceTimersByTime(120_000)
    expect(loadResourcesMock).toHaveBeenCalledTimes(1)
    d.stop()
  })

  it('polling pauses when document.visibilityState becomes hidden', async () => {
    vi.useFakeTimers()
    const d = setup({ refresh: '30s' })
    d.start()
    await Promise.resolve()
    expect(loadResourcesMock).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'hidden',
    })
    document.dispatchEvent(new Event('visibilitychange'))
    vi.advanceTimersByTime(60_000)
    expect(loadResourcesMock).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'visible',
    })
    document.dispatchEvent(new Event('visibilitychange'))
    await Promise.resolve()
    expect(loadResourcesMock).toHaveBeenCalledTimes(2) // refresh on resume
    d.stop()
  })
})
