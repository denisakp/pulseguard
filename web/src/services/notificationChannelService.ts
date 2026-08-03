import { getAuthenticatedClient, request } from '@/core/http/client'
import type {
  NotificationChannel,
  CreateNotificationChannel,
  UpdateNotificationChannel,
  TestNotificationChannelConfig,
} from '@/types'

// v1 list envelope: `{ data, meta }`. `per_page` is capped at 100 server-side.
const PER_PAGE = 100

interface ChannelListEnvelope {
  data: NotificationChannel[]
  meta: { total: number; page: number; per_page: number }
}

// fetchChannels preserves the legacy "return ALL channels" contract on top of
// the paginated v1 `GET /v1/notification-channels` endpoint: it walks pages of
// `per_page=100` accumulating `.data` until it has collected `meta.total` items
// (or a page comes back short), then returns the flattened list.
export const fetchChannels = async (): Promise<NotificationChannel[]> => {
  const client = getAuthenticatedClient()
  const all: NotificationChannel[] = []
  let page = 1

  for (;;) {
    const res = await request<ChannelListEnvelope>(client, 'v1/notification-channels', {
      searchParams: { page, per_page: PER_PAGE },
    })
    all.push(...res.data)

    const total = res.meta?.total ?? all.length
    if (all.length >= total || res.data.length < PER_PAGE) break
    page += 1
  }

  return all
}

export const fetchChannel = async (id: string): Promise<NotificationChannel> => {
  const res = await request<{ data: NotificationChannel }>(
    getAuthenticatedClient(),
    `v1/notification-channels/${id}`,
  )
  return res.data
}

export const createChannel = async (
  payload: CreateNotificationChannel,
): Promise<NotificationChannel> => {
  const res = await request<{ data: NotificationChannel }>(
    getAuthenticatedClient(),
    'v1/notification-channels',
    { method: 'POST', json: payload },
  )
  return res.data
}

export const updateChannel = async (
  id: string,
  payload: UpdateNotificationChannel,
): Promise<NotificationChannel> => {
  const res = await request<{ data: NotificationChannel }>(
    getAuthenticatedClient(),
    `v1/notification-channels/${id}`,
    { method: 'PATCH', json: payload },
  )
  return res.data
}

export const deleteChannel = async (id: string): Promise<void> => {
  await request<void>(getAuthenticatedClient(), `v1/notification-channels/${id}`, {
    method: 'DELETE',
  })
}

export interface ChannelTestResult {
  delivered: boolean
  error?: string
  latency_ms: number
}

export const testChannel = async (id: string): Promise<ChannelTestResult> => {
  const started = performance.now()
  try {
    await request<{ data: { message: string } }>(
      getAuthenticatedClient(),
      `v1/notification-channels/${id}/test`,
      { method: 'POST', json: {} },
    )
    return { delivered: true, latency_ms: Math.round(performance.now() - started) }
  } catch (e) {
    return {
      delivered: false,
      error: e instanceof Error ? e.message : 'Test failed',
      latency_ms: Math.round(performance.now() - started),
    }
  }
}

export const setDefault = async (id: string): Promise<void> => {
  await updateChannel(id, { enabled_by_default: true })
}

export const testChannelConfig = async (payload: TestNotificationChannelConfig): Promise<void> => {
  await request<{ data: { message: string } }>(
    getAuthenticatedClient(),
    'v1/notification-channels/test-config',
    { method: 'POST', json: payload },
  )
}
