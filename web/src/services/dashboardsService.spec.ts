import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { createRemoteDashboardsFeed } from './dashboardsService'
import { server } from '@/test/msw/server'
import type { Dashboard } from '@/types'

function sample(id = 'd1'): Dashboard {
  return {
    id,
    name: 'Prod',
    scope: { mode: 'tag', payload: { tagIds: ['t1'] } },
    widgets: [{ id: 'w1', widgetTypeId: 'uptime-stat', position: 0 }],
    defaultTimeRange: '24h',
    refreshInterval: '1m',
    visibility: 'team',
    ownerId: 'alice',
    ownerName: 'Alice',
    createdAt: '2026-07-02T10:00:00Z',
    updatedAt: '2026-07-02T10:00:00Z',
  }
}

describe('dashboardsService', () => {
  it('list unwraps the {data} envelope', async () => {
    server.use(http.get('*/v1/dashboards', () => HttpResponse.json({ data: [sample()] })))
    const items = await createRemoteDashboardsFeed().list()
    expect(items).toHaveLength(1)
    expect(items[0]!.ownerName).toBe('Alice')
  })

  it('get returns the dashboard', async () => {
    server.use(http.get('*/v1/dashboards/d1', () => HttpResponse.json({ data: sample() })))
    const d = await createRemoteDashboardsFeed().get('d1')
    expect(d?.id).toBe('d1')
  })

  it('get maps 404 to null', async () => {
    server.use(
      http.get('*/v1/dashboards/missing', () =>
        HttpResponse.json({ error: { code: 'DASHBOARD_NOT_FOUND', message: 'x' } }, { status: 404 }),
      ),
    )
    expect(await createRemoteDashboardsFeed().get('missing')).toBeNull()
  })

  it('create POSTs and returns the created dashboard', async () => {
    let body: unknown
    server.use(
      http.post('*/v1/dashboards', async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ data: sample('new') }, { status: 201 })
      }),
    )
    const created = await createRemoteDashboardsFeed().create({
      name: 'Prod',
      scope: { mode: 'tag', payload: { tagIds: ['t1'] } },
      widgets: [],
      defaultTimeRange: '24h',
      refreshInterval: '1m',
      visibility: 'team',
      ownerId: 'alice',
      ownerName: 'Alice',
    })
    expect(created.id).toBe('new')
    expect(body).toMatchObject({ name: 'Prod' })
  })

  it('update PATCHes', async () => {
    let method = ''
    server.use(
      http.patch('*/v1/dashboards/d1', ({ request }) => {
        method = request.method
        return HttpResponse.json({ data: sample() })
      }),
    )
    await createRemoteDashboardsFeed().update('d1', { name: 'Renamed' })
    expect(method).toBe('PATCH')
  })

  it('saveLayout PUTs the widgets', async () => {
    let payload: { widgets?: unknown } = {}
    server.use(
      http.put('*/v1/dashboards/d1/layout', async ({ request }) => {
        payload = (await request.json()) as { widgets?: unknown }
        return HttpResponse.json({ data: sample() })
      }),
    )
    await createRemoteDashboardsFeed().saveLayout('d1', [{ id: 'w1', widgetTypeId: 'uptime-stat', position: 0 }])
    expect(Array.isArray(payload.widgets)).toBe(true)
  })

  it('remove DELETEs', async () => {
    let hit = false
    server.use(
      http.delete('*/v1/dashboards/d1', () => {
        hit = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    await createRemoteDashboardsFeed().remove('d1')
    expect(hit).toBe(true)
  })
})
