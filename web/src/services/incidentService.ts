import { getAuthenticatedClient, request } from '@/core/http/client'
import type { Incident, IncidentsQueryParams, PaginatedResponse } from '@/types'

// v1 list envelope: `{ data, meta }`. `per_page` is capped at 100 server-side.
interface IncidentListEnvelope {
  data: Incident[]
  meta: { page: number; per_page: number; total: number }
}

/**
 * Fetch incidents from the v1 API. Maps caller filters onto the v1 query params
 * and unwraps the `{ data, meta }` envelope into the store-facing
 * `PaginatedResponse` shape (`limit = meta.per_page`, `offset = (page-1)*per_page`).
 */
export const fetchIncidents = async (
  params?: IncidentsQueryParams,
): Promise<PaginatedResponse<Incident>> => {
  const searchParams: Record<string, string | number> = {}
  if (params?.status !== undefined) searchParams.status = params.status
  // Legacy `unresolved=true` maps onto the v1 `status=open` filter.
  if (params?.unresolved === true) searchParams.status = 'open'
  if (params?.monitor_id !== undefined) searchParams.monitor_id = params.monitor_id
  if (params?.page !== undefined) searchParams.page = params.page
  if (params?.per_page !== undefined) searchParams.per_page = params.per_page

  const res = await request<IncidentListEnvelope>(getAuthenticatedClient(), 'v1/incidents', {
    searchParams,
  })

  const perPage = res.meta?.per_page ?? res.data.length
  return {
    data: res.data,
    total: res.meta?.total ?? res.data.length,
    limit: perPage,
    offset: res.meta ? (res.meta.page - 1) * res.meta.per_page : 0,
  }
}

export const fetchIncidentById = async (id: string): Promise<Incident> => {
  const res = await request<{ data: Incident }>(getAuthenticatedClient(), `v1/incidents/${id}`)
  return res.data
}

export const fetchUnresolvedIncidents = async (): Promise<Incident[]> => {
  const res = await fetchIncidents({ unresolved: true })
  return res.data
}
