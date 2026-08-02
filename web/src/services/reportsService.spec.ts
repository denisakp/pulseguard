import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { createRemoteReportsFeed } from './reportsService'
import { server } from '@/test/msw/server'
import type { MonthlyReport } from '@/types'

describe('reportsService', () => {
  it('fetchMonthly unwraps the {data} envelope', async () => {
    server.use(
      http.get('*/v1/reports/settings', () =>
        HttpResponse.json({
          data: {
            enabled: true,
            recipientEmail: 'ops@example.com',
            schedule: 'monthly-1st',
            scope: 'all-resources',
            lastSentAt: null,
          },
        }),
      ),
    )
    const got = await createRemoteReportsFeed().fetchMonthly()
    expect(got.enabled).toBe(true)
    expect(got.recipientEmail).toBe('ops@example.com')
  })

  it('saveMonthly PUTs the config and returns the updated settings', async () => {
    let sent: MonthlyReport | null = null
    server.use(
      http.put('*/v1/reports/settings', async ({ request }) => {
        sent = (await request.json()) as MonthlyReport
        return HttpResponse.json({ data: sent })
      }),
    )
    const next: MonthlyReport = {
      enabled: true,
      recipientEmail: 'a@b.com',
      schedule: 'monthly-1st',
      scope: 'all-resources',
      lastSentAt: null,
    }
    const got = await createRemoteReportsFeed().saveMonthly(next)
    expect(sent).not.toBeNull()
    expect(got.recipientEmail).toBe('a@b.com')
  })

  it('fetchHistory forwards the limit and unwraps the list', async () => {
    let url = ''
    server.use(
      http.get('*/v1/reports/history', ({ request }) => {
        url = request.url
        return HttpResponse.json({
          data: [
            {
              id: 'r1',
              period: '2026-06',
              sentAt: '2026-07-01T00:05:00Z',
              status: 'delivered',
              uptimePct: 99.9,
              incidentCount: 1,
              downtimeSeconds: 300,
              recipientEmail: 'ops@example.com',
              resourceBreakdown: [{ name: 'API', uptimePct: 99.9, incidents: 1 }],
            },
          ],
        })
      }),
    )
    const rows = await createRemoteReportsFeed().fetchHistory(3)
    expect(url).toContain('limit=3')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.period).toBe('2026-06')
  })

  it('fetchHistory returns [] when the envelope has no data', async () => {
    server.use(http.get('*/v1/reports/history', () => HttpResponse.json({ data: null })))
    expect(await createRemoteReportsFeed().fetchHistory()).toEqual([])
  })

  it('fetchPendingPreview maps a data object', async () => {
    server.use(
      http.get('*/v1/reports/preview', () =>
        HttpResponse.json({
          data: {
            id: '',
            period: '2026-07',
            sentAt: '2026-07-15T10:00:00Z',
            status: 'pending',
            uptimePct: 100,
            incidentCount: 0,
            downtimeSeconds: 0,
            recipientEmail: 'ops@example.com',
            resourceBreakdown: [],
          },
        }),
      ),
    )
    const pv = await createRemoteReportsFeed().fetchPendingPreview()
    expect(pv?.period).toBe('2026-07')
  })

  it('fetchPendingPreview maps null to null', async () => {
    server.use(http.get('*/v1/reports/preview', () => HttpResponse.json({ data: null })))
    expect(await createRemoteReportsFeed().fetchPendingPreview()).toBeNull()
  })
})
