import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { __createFeedForTests } from './notificationFeedService'
import { server } from '@/test/msw/server'

describe('notificationFeedService', () => {
  it('fetch unwraps the v1 {data} envelope into items', async () => {
    server.use(
      http.get('*/v1/notifications', () =>
        HttpResponse.json({
          data: [
            {
              id: 'n1',
              category: 'incident',
              severity: 'error',
              title: 'down',
              occurredAt: '2026-06-27T10:00:00Z',
              deepLink: '/incidents/n1',
              unread: true,
            },
          ],
          meta: { page: 1, per_page: 50, total: 1 },
        }),
      ),
    )
    const feed = __createFeedForTests()
    const items = await feed.fetch()
    expect(items).toHaveLength(1)
    expect(items[0]!.id).toBe('n1')
    expect(items[0]!.unread).toBe(true)
  })

  it('markRead POSTs to /{id}/read', async () => {
    let hit = ''
    server.use(
      http.post('*/v1/notifications/:id/read', ({ params }) => {
        hit = String(params.id)
        return new HttpResponse(null, { status: 204 })
      }),
    )
    await __createFeedForTests().markRead('abc')
    expect(hit).toBe('abc')
  })

  it('markAllRead POSTs to /read-all', async () => {
    let called = false
    server.use(
      http.post('*/v1/notifications/read-all', () => {
        called = true
        return HttpResponse.json({ data: { marked: 3 } })
      }),
    )
    await __createFeedForTests().markAllRead()
    expect(called).toBe(true)
  })
})
