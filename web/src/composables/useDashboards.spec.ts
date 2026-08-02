import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Dashboard, WidgetInstance } from '@/types'
import type { DashboardsFeed } from '@/services/dashboardsService'

const authUserId = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<string | null>('user-default')
})

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({
    get userId() {
      return authUserId.value
    },
    email: 'admin@example.com',
  }),
}))

import {
  useDashboards,
  __setDashboardsFeedActiveForTests,
  __resetUseDashboardsForTests,
} from './useDashboards'

function makeDashboard(over: Partial<Dashboard>): Dashboard {
  const base: Dashboard = {
    id: 'd-base',
    name: 'Base',
    scope: { mode: 'tag', payload: { tagIds: [] } },
    widgets: [],
    defaultTimeRange: '24h',
    refreshInterval: '30s',
    visibility: 'private',
    ownerId: 'user-default',
    ownerName: 'You',
    createdAt: '2026-06-01T10:00:00Z',
    updatedAt: '2026-06-09T10:00:00Z',
  }
  return { ...base, ...over }
}

function makeFakeFeed(initial: Dashboard[] = []): DashboardsFeed {
  let store = initial.slice()
  return {
    async list() {
      return store
        .slice()
        .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
    },
    async get(id) {
      return store.find((d) => d.id === id) ?? null
    },
    async create(input) {
      const created: Dashboard = {
        ...input,
        id: `d-${store.length + 1}`,
        createdAt: '2026-06-10T00:00:00Z',
        updatedAt: '2026-06-10T00:00:00Z',
      }
      store = [...store, created]
      return created
    },
    async update(id, patch) {
      const d = store.find((x) => x.id === id)!
      const merged = { ...d, ...patch, updatedAt: '2026-06-10T01:00:00Z' }
      store = store.map((x) => (x.id === id ? merged : x))
      return merged
    },
    async remove(id) {
      store = store.filter((d) => d.id !== id)
    },
    async saveLayout(id, widgets: WidgetInstance[]) {
      const d = store.find((x) => x.id === id)!
      const merged = { ...d, widgets, updatedAt: '2026-06-10T02:00:00Z' }
      store = store.map((x) => (x.id === id ? merged : x))
      return merged
    },
  }
}

describe('useDashboards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    __resetUseDashboardsForTests()
    authUserId.value = 'user-default'
    localStorage.clear()
    sessionStorage.clear()
  })

  afterEach(() => {
    __resetUseDashboardsForTests()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('load hydrates dashboards from the feed', async () => {
    const feed = makeFakeFeed([
      makeDashboard({ id: 'd1', name: 'one' }),
      makeDashboard({ id: 'd2', name: 'two', ownerId: 'other' }),
    ])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    expect(d.dashboards.value.length).toBe(2)
  })

  it('filter `mine` keeps only dashboards owned by current user', async () => {
    const feed = makeFakeFeed([
      makeDashboard({ id: 'd1', name: 'one' }),
      makeDashboard({ id: 'd2', name: 'two', ownerId: 'other' }),
    ])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    d.setFilter('mine')
    expect(d.filteredSorted.value.map((x) => x.id)).toEqual(['d1'])
  })

  it('filter `shared` is empty on CE (FR-013)', async () => {
    const feed = makeFakeFeed([makeDashboard({ id: 'd1' })])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    d.setFilter('shared')
    expect(d.filteredSorted.value).toEqual([])
  })

  it('filter `starred` reflects toggleStar state', async () => {
    const feed = makeFakeFeed([
      makeDashboard({ id: 'd1' }),
      makeDashboard({ id: 'd2', name: 'two' }),
    ])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    d.toggleStar('d2')
    d.setFilter('starred')
    expect(d.filteredSorted.value.map((x) => x.id)).toEqual(['d2'])
  })

  it('sort `name` switches to A→Z, `updated` to recent-first', async () => {
    const feed = makeFakeFeed([
      makeDashboard({ id: 'd1', name: 'Zebra', updatedAt: '2026-06-09T10:00:00Z' }),
      makeDashboard({ id: 'd2', name: 'Apple', updatedAt: '2026-06-08T10:00:00Z' }),
    ])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    d.setSort('name')
    expect(d.filteredSorted.value.map((x) => x.name)).toEqual(['Apple', 'Zebra'])
    d.setSort('updated')
    expect(d.filteredSorted.value.map((x) => x.name)).toEqual(['Zebra', 'Apple'])
  })

  it('create prepends to the list', async () => {
    const feed = makeFakeFeed([makeDashboard({ id: 'd1' })])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    await d.create({
      name: 'new',
      scope: { mode: 'tag', payload: { tagIds: [] } },
      widgets: [],
      defaultTimeRange: '24h',
      refreshInterval: '30s',
      visibility: 'private',
      ownerId: 'user-default',
      ownerName: 'You',
    })
    expect(d.dashboards.value.length).toBe(2)
    expect(d.dashboards.value[0]!.name).toBe('new')
  })

  it('toggleStar persists into localStorage under the per-user key', async () => {
    const feed = makeFakeFeed([makeDashboard({ id: 'd1' })])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    d.toggleStar('d1')
    const { nextTick } = await import('vue')
    await nextTick()
    expect(d.isStarred('d1')).toBe(true)
    expect(JSON.parse(localStorage.getItem('dashboard.starred.user-default') ?? '[]')).toEqual([
      'd1',
    ])
    d.toggleStar('d1')
    await nextTick()
    expect(d.isStarred('d1')).toBe(false)
  })

  it('isOwner correctly identifies current-user ownership', async () => {
    const feed = makeFakeFeed([
      makeDashboard({ id: 'd1' }),
      makeDashboard({ id: 'd2', ownerId: 'other' }),
    ])
    __setDashboardsFeedActiveForTests(feed)
    const d = useDashboards()
    await d.load()
    expect(d.isOwner(d.dashboards.value.find((x) => x.id === 'd1')!)).toBe(true)
    expect(d.isOwner(d.dashboards.value.find((x) => x.id === 'd2')!)).toBe(false)
  })
})
