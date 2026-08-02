import type { Dashboard, WidgetInstance } from '@/types'
import { getAuthenticatedClient, request } from '@/core/http/client'
import { NotFoundError } from '@/core/errors'

/**
 * Dashboards feed — always backed by the real v1 API.
 *
 * Ownership (FR-025): read instance-wide, edit owner-only (backend returns 403
 * FORBIDDEN on non-owner mutation). The UI gates affordances so FORBIDDEN never
 * surfaces in the normal flow. v1 endpoints wrap payloads in a `{ data }`
 * envelope; the DTO already matches the `Dashboard` shape (camelCase).
 */
export interface DashboardsFeed {
  list(): Promise<Dashboard[]>
  get(id: string): Promise<Dashboard | null>
  create(input: Omit<Dashboard, 'id' | 'createdAt' | 'updatedAt'>): Promise<Dashboard>
  update(id: string, patch: Partial<Dashboard>): Promise<Dashboard>
  remove(id: string): Promise<void>
  saveLayout(id: string, widgets: WidgetInstance[]): Promise<Dashboard>
}

const successMsg = (m: string) => ({ headers: { 'x-success-message': m } })

export function createRemoteDashboardsFeed(): DashboardsFeed {
  const client = () => getAuthenticatedClient()
  return {
    async list(): Promise<Dashboard[]> {
      const res = await request<{ data: Dashboard[] }>(client(), 'v1/dashboards')
      return res?.data ?? []
    },
    async get(id: string): Promise<Dashboard | null> {
      try {
        const res = await request<{ data: Dashboard }>(client(), `v1/dashboards/${id}`)
        return res?.data ?? null
      } catch (e) {
        if (e instanceof NotFoundError) return null
        throw e
      }
    },
    async create(input): Promise<Dashboard> {
      const res = await request<{ data: Dashboard }>(client(), 'v1/dashboards', {
        method: 'POST',
        json: input,
        ...successMsg('Dashboard created'),
      })
      return res.data
    },
    async update(id, patch): Promise<Dashboard> {
      const res = await request<{ data: Dashboard }>(client(), `v1/dashboards/${id}`, {
        method: 'PATCH',
        json: patch,
        ...successMsg('Dashboard updated'),
      })
      return res.data
    },
    async remove(id): Promise<void> {
      await request<void>(client(), `v1/dashboards/${id}`, {
        method: 'DELETE',
        ...successMsg('Dashboard deleted'),
      })
    },
    async saveLayout(id, widgets): Promise<Dashboard> {
      const res = await request<{ data: Dashboard }>(client(), `v1/dashboards/${id}/layout`, {
        method: 'PUT',
        json: { widgets },
      })
      return res.data
    },
  }
}

let activeFeed: DashboardsFeed = createRemoteDashboardsFeed()

const dashboardsService: DashboardsFeed = {
  list: () => activeFeed.list(),
  get: (id) => activeFeed.get(id),
  create: (input) => activeFeed.create(input),
  update: (id, patch) => activeFeed.update(id, patch),
  remove: (id) => activeFeed.remove(id),
  saveLayout: (id, widgets) => activeFeed.saveLayout(id, widgets),
}

export default dashboardsService

// Test-only: inject a fake feed (view specs) or reset to the real one.
export function __setDashboardsFeedForTests(feed: DashboardsFeed): void {
  activeFeed = feed
}

export function __resetDashboardsForTests(): void {
  activeFeed = createRemoteDashboardsFeed()
}
