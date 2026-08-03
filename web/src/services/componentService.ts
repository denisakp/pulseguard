import { getAuthenticatedClient, request } from '@/core/http/client'
import type {
  Component,
  CreateComponent,
  UpdateComponent,
  BulkAssignPayload,
  BulkRemovePayload,
} from '@/types'

const successMsg = (m: string) => ({ headers: { 'x-success-message': m } })

// v1 list envelope: `{ data, meta }`. `per_page` is capped at 100 server-side.
const PER_PAGE = 100

interface ComponentListEnvelope {
  data: Component[]
  meta: { total: number; page: number; per_page: number }
}

// fetchComponents preserves the legacy "return ALL components" contract on top
// of the paginated v1 `GET /v1/components` endpoint: it walks pages of
// `per_page=100` accumulating `.data` until it has collected `meta.total` items
// (or a page comes back short), then returns the flattened list.
export const fetchComponents = async (): Promise<Component[]> => {
  const client = getAuthenticatedClient()
  const all: Component[] = []
  let page = 1

  for (;;) {
    const res = await request<ComponentListEnvelope>(client, 'v1/components', {
      searchParams: { page, per_page: PER_PAGE },
    })
    all.push(...res.data)

    const total = res.meta?.total ?? all.length
    if (all.length >= total || res.data.length < PER_PAGE) break
    page += 1
  }

  return all
}

export const fetchComponent = async (id: string): Promise<Component> => {
  const res = await request<{ data: Component }>(getAuthenticatedClient(), `v1/components/${id}`)
  return res.data
}

export const createComponent = async (component: CreateComponent): Promise<Component> => {
  const res = await request<{ data: Component }>(getAuthenticatedClient(), 'v1/components', {
    method: 'POST',
    json: component,
    ...successMsg('Component created successfully'),
  })
  return res.data
}

export const updateComponent = async (
  id: string,
  component: UpdateComponent,
): Promise<Component> => {
  const res = await request<{ data: Component }>(getAuthenticatedClient(), `v1/components/${id}`, {
    method: 'PATCH',
    json: component,
    ...successMsg('Component updated successfully'),
  })
  return res.data
}

// Delete a component. Server returns 204 No Content on success. If the component
// still has resources attached the server returns 409 Conflict — `request<T>`
// routes that through `normalizeError` (→ ConflictError) and the shared error
// interceptor surfaces a "Conflict: <message>" toast to the user.
export const deleteComponent = async (id: string): Promise<void> => {
  await request<void>(getAuthenticatedClient(), `v1/components/${id}`, {
    method: 'DELETE',
    ...successMsg('Component deleted successfully'),
  })
}

export const bulkAssignToComponent = async (
  componentId: string,
  payload: BulkAssignPayload,
): Promise<void> => {
  // Response is `{ data: { message } }`; callers ignore the body.
  await request<{ data: { message: string } }>(
    getAuthenticatedClient(),
    `v1/components/${componentId}/resources/bulk-assign`,
    {
      method: 'POST',
      json: payload,
      ...successMsg('Resources assigned successfully'),
    },
  )
}

export const bulkRemoveFromComponent = async (payload: BulkRemovePayload): Promise<void> => {
  // Response is `{ data: { message } }`; callers ignore the body.
  await request<{ data: { message: string } }>(
    getAuthenticatedClient(),
    'v1/components/resources/bulk-remove',
    {
      method: 'POST',
      json: payload,
      ...successMsg('Resources removed from components successfully'),
    },
  )
}
