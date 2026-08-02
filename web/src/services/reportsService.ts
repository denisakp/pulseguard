import type { MonthlyReport, ReportHistoryEntry } from '@/types'
import { getAuthenticatedClient, request } from '@/core/http/client'

/**
 * Reports feed — always backed by the real v1 API.
 *
 * The backend persists the monthly-report configuration, generates the monthly
 * report (reusing daily uptime aggregates + incidents), and emails it via an
 * SMTP notification channel. v1 endpoints wrap payloads in a `{ data }`
 * envelope; the DTOs already match the `MonthlyReport` / `ReportHistoryEntry`
 * shapes (camelCase). The zero-resources guard lives in `useReports`.
 */
export interface ReportsFeed {
  fetchMonthly(): Promise<MonthlyReport>
  saveMonthly(next: MonthlyReport): Promise<MonthlyReport>
  fetchHistory(limit?: number): Promise<ReportHistoryEntry[]>
  fetchPendingPreview(): Promise<ReportHistoryEntry | null>
}

const successMsg = (m: string) => ({ headers: { 'x-success-message': m } })

export function createRemoteReportsFeed(): ReportsFeed {
  const client = () => getAuthenticatedClient()
  return {
    async fetchMonthly(): Promise<MonthlyReport> {
      const res = await request<{ data: MonthlyReport }>(client(), 'v1/reports/settings')
      return res.data
    },
    async saveMonthly(next: MonthlyReport): Promise<MonthlyReport> {
      const res = await request<{ data: MonthlyReport }>(client(), 'v1/reports/settings', {
        method: 'PUT',
        json: next,
        ...successMsg('Report settings saved'),
      })
      return res.data
    },
    async fetchHistory(limit = 6): Promise<ReportHistoryEntry[]> {
      const res = await request<{ data: ReportHistoryEntry[] }>(
        client(),
        `v1/reports/history?limit=${limit}`,
      )
      return res?.data ?? []
    },
    async fetchPendingPreview(): Promise<ReportHistoryEntry | null> {
      const res = await request<{ data: ReportHistoryEntry | null }>(client(), 'v1/reports/preview')
      return res?.data ?? null
    },
  }
}

let activeFeed: ReportsFeed = createRemoteReportsFeed()

const reportsService: ReportsFeed = {
  fetchMonthly: () => activeFeed.fetchMonthly(),
  saveMonthly: (next) => activeFeed.saveMonthly(next),
  fetchHistory: (limit) => activeFeed.fetchHistory(limit),
  fetchPendingPreview: () => activeFeed.fetchPendingPreview(),
}

export default reportsService

// Test-only: inject a fake feed (view specs) or reset to the real one.
export function __setReportsFeedForTests(feed: ReportsFeed): void {
  activeFeed = feed
}

export function __resetReportsForTests(): void {
  activeFeed = createRemoteReportsFeed()
}
