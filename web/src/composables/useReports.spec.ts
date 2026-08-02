import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import type { ReportsFeed } from '@/services/reportsService'

const resourcesRef = ref<unknown[]>([])

vi.mock('@/stores/resourceStore', () => ({
  useResourceStore: () => ({
    resources: resourcesRef.value,
  }),
}))

import {
  useReports,
  __setReportsFeedActiveForTests,
  __resetUseReportsForTests,
} from './useReports'

function makeFakeFeed(initial?: Partial<ReportsFeed>): ReportsFeed {
  let monthly = {
    enabled: false,
    recipientEmail: 'admin@example.com',
    schedule: 'monthly-1st' as const,
    scope: 'all-resources' as const,
    lastSentAt: null as string | null,
  }
  return {
    fetchMonthly: vi.fn(async () => ({ ...monthly })),
    saveMonthly: vi.fn(async (next) => {
      monthly = { ...next }
      return { ...monthly }
    }),
    fetchHistory: vi.fn(async () => [
      {
        id: 'h1',
        period: 'May 2026',
        sentAt: '2026-06-01T08:00:00Z',
        status: 'delivered' as const,
        uptimePct: 99.5,
        incidentCount: 2,
        downtimeSeconds: 1200,
        recipientEmail: 'admin@example.com',
        resourceBreakdown: [],
      },
    ]),
    fetchPendingPreview: vi.fn(async () => null),
    ...initial,
  }
}

describe('useReports', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    __resetUseReportsForTests()
    resourcesRef.value = [{ id: 'r1' }]
  })

  afterEach(() => {
    __resetUseReportsForTests()
  })

  it('loadAll hydrates monthly + history from the feed', async () => {
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    expect(r.monthly.value?.enabled).toBe(false)
    expect(r.history.value.length).toBe(1)
    expect(r.latestDelivered.value?.id).toBe('h1')
  })

  it('loadAll is idempotent (no duplicate fetches)', async () => {
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    await r.loadAll()
    expect(feed.fetchMonthly).toHaveBeenCalledTimes(1)
  })

  it('toggleMonthly(true) succeeds when resources exist', async () => {
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    const next = await r.toggleMonthly(true)
    expect(next?.enabled).toBe(true)
    expect(r.monthly.value?.enabled).toBe(true)
  })

  it('toggleMonthly(true) throws NO_RESOURCES when org is empty (FR-007)', async () => {
    resourcesRef.value = []
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    await expect(r.toggleMonthly(true)).rejects.toThrow('NO_RESOURCES')
    expect(r.monthly.value?.enabled).toBe(false)
  })

  it('toggleMonthly(false) is allowed even with zero resources', async () => {
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    await r.toggleMonthly(true)
    resourcesRef.value = []
    await expect(r.toggleMonthly(false)).resolves.toBeDefined()
    expect(r.monthly.value?.enabled).toBe(false)
  })

  it('setRecipient updates the persisted monthly state', async () => {
    const feed = makeFakeFeed()
    __setReportsFeedActiveForTests(feed)
    const r = useReports()
    await r.loadAll()
    await r.setRecipient('ops@example.com')
    expect(r.monthly.value?.recipientEmail).toBe('ops@example.com')
  })
})
