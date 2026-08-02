import { http, request } from '@/core/http/client'
import type {
  PublicStatusSummary,
  PublicStatusIncidentsArchive,
  PublicStatusUptimeRange,
  PublicStatusResourceWindows,
  PublicIncidentDetail,
} from '@/types'

/**
 * Public status API. All endpoints are unauthenticated and
 * short-cached by the public_status_cache middleware on the server.
 */

export const fetchPublicStatusSummary = async (): Promise<PublicStatusSummary> => {
  return await request<PublicStatusSummary>(http, 'status')
}

export interface IncidentArchiveQuery {
  from?: string
  to?: string
  component_id?: string
}

export const fetchPublicStatusIncidents = async (
  q: IncidentArchiveQuery = {},
): Promise<PublicStatusIncidentsArchive> => {
  const searchParams = new URLSearchParams()
  if (q.from) searchParams.set('from', q.from)
  if (q.to) searchParams.set('to', q.to)
  if (q.component_id) searchParams.set('component_id', q.component_id)
  const qs = searchParams.toString()
  return await request<PublicStatusIncidentsArchive>(
    http,
    qs ? `status/incidents?${qs}` : 'status/incidents',
  )
}

export interface UptimeRangeQuery {
  from: string
  to: string
  component_id?: string
}

export const fetchPublicStatusUptime = async (
  q: UptimeRangeQuery,
): Promise<PublicStatusUptimeRange> => {
  const searchParams = new URLSearchParams({ from: q.from, to: q.to })
  if (q.component_id) searchParams.set('component_id', q.component_id)
  return await request<PublicStatusUptimeRange>(http, `status/uptime?${searchParams.toString()}`)
}

export const fetchPublicIncidentDetail = async (
  incidentID: string,
): Promise<PublicIncidentDetail> => {
  return await request<PublicIncidentDetail>(
    http,
    `status/incidents/${encodeURIComponent(incidentID)}`,
  )
}

export const fetchPublicStatusResourceWindows = async (
  resourceID: string,
): Promise<PublicStatusResourceWindows> => {
  return await request<PublicStatusResourceWindows>(
    http,
    `status/resource/${encodeURIComponent(resourceID)}/windows`,
  )
}
