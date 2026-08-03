import { getAuthenticatedClient, request } from '@/core/http/client'
import type {
  CreateResource,
  Resource,
  UpdateResource,
  HourlyUptimeStat,
  SystemCapabilities,
} from '@/types'

const successMsg = (m: string) => ({ headers: { 'x-success-message': m } })

// v1 list envelope: `{ data, meta }`. `per_page` is capped at 100 server-side.
const PER_PAGE = 100

interface ResourceListEnvelope {
  data: Resource[]
  meta: { total: number; page: number; per_page: number }
}

// fetchResources preserves the legacy "return ALL resources" contract on top of
// the paginated v1 `GET /v1/monitors` endpoint: it walks pages of `per_page=100`
// accumulating `.data` until it has collected `meta.total` items (or a page comes
// back short), then returns the flattened list.
export const fetchResources = async (): Promise<Resource[]> => {
  const client = getAuthenticatedClient()
  const all: Resource[] = []
  let page = 1

  for (;;) {
    const res = await request<ResourceListEnvelope>(client, 'v1/monitors', {
      searchParams: { page, per_page: PER_PAGE },
    })
    all.push(...res.data)

    const total = res.meta?.total ?? all.length
    if (all.length >= total || res.data.length < PER_PAGE) break
    page += 1
  }

  return all
}

export const fetchResource = async (id: string, limit?: number): Promise<Resource> => {
  const searchParams = limit ? { limit } : undefined
  const res = await request<{ data: Resource }>(getAuthenticatedClient(), `v1/monitors/${id}`, {
    searchParams,
  })
  return res.data
}

export const fetchUptimeStats = async (
  id: string,
): Promise<{ resource_id: string; stats: HourlyUptimeStat[] }> => {
  const res = await request<{ data: { resource_id: string; stats: HourlyUptimeStat[] } }>(
    getAuthenticatedClient(),
    `v1/monitors/${id}/uptime-stats`,
  )
  return res.data
}

export const createResource = async (resource: CreateResource): Promise<Resource> => {
  const res = await request<{ data: Resource }>(getAuthenticatedClient(), 'v1/monitors', {
    method: 'POST',
    json: resource,
    ...successMsg('Monitor created successfully'),
  })
  return res.data
}

export const updateResource = async (id: string, resource: UpdateResource): Promise<Resource> => {
  const res = await request<{ data: Resource }>(getAuthenticatedClient(), `v1/monitors/${id}`, {
    method: 'PATCH',
    json: resource,
    ...successMsg('Monitor updated successfully'),
  })
  return res.data
}

export const deleteResource = async (id: string): Promise<void> => {
  await request<void>(getAuthenticatedClient(), `v1/monitors/${id}`, {
    method: 'DELETE',
    ...successMsg('Monitor deleted successfully'),
  })
}

export const pauseResource = async (id: string): Promise<Resource> => {
  const res = await request<{ data: Resource }>(getAuthenticatedClient(), `v1/monitors/${id}/pause`, {
    method: 'POST',
    json: {},
    ...successMsg('Monitoring paused'),
  })
  return res.data
}

export const resumeResource = async (id: string): Promise<Resource> => {
  const res = await request<{ data: Resource }>(
    getAuthenticatedClient(),
    `v1/monitors/${id}/resume`,
    {
      method: 'POST',
      json: {},
      ...successMsg('Monitoring resumed'),
    },
  )
  return res.data
}

export const addTagsToResource = async (resourceId: string, tagIds: string[]): Promise<void> => {
  await request<void>(getAuthenticatedClient(), `v1/monitors/${resourceId}/tags`, {
    method: 'POST',
    json: { tag_ids: tagIds },
  })
}

export const removeTagFromResource = async (resourceId: string, tagId: string): Promise<void> => {
  await request<void>(getAuthenticatedClient(), `v1/monitors/${resourceId}/tags/${tagId}`, {
    method: 'DELETE',
  })
}

export const fetchCapabilities = async (): Promise<SystemCapabilities> => {
  return await request<SystemCapabilities>(getAuthenticatedClient(), 'system/capabilities')
}
