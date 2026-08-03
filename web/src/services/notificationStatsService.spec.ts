import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'

import { fetchNotificationStats } from '@/services/notificationStatsService'
import { ServerError } from '@/core/errors'
import { server } from '@/test/msw/server'

describe('notificationStatsService', () => {
  it('sends GET to /v1/notifications/stats and unwraps the data envelope', async () => {
    const stats = { sent_30d: 42, pending: 3, failed_24h: 1 }
    server.use(http.get('*/v1/notifications/stats', () => HttpResponse.json({ data: stats })))
    const result = await fetchNotificationStats()
    expect(result).toEqual(stats)
  })

  it('propagates server errors as ServerError', async () => {
    server.use(http.get('*/v1/notifications/stats', () => HttpResponse.json({}, { status: 500 })))
    await expect(fetchNotificationStats()).rejects.toBeInstanceOf(ServerError)
  })
})
