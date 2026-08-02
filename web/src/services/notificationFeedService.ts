import type { NotificationFeedItem } from '@/types'
import { getAuthenticatedClient, request } from '@/core/http/client'

/**
 * In-app notification feed — always backed by the real v1 API.
 *
 * The bell UI (`useNotifications` / `UNotificationDropdown`) imports only the
 * `NotificationFeed` interface + the default export. The v1 endpoints wrap the
 * payload in a `{ data }` envelope; the DTO already matches `NotificationFeedItem`.
 */
export interface NotificationFeed {
  fetch(): Promise<NotificationFeedItem[]>
  markRead(id: string): Promise<void>
  markAllRead(): Promise<void>
}

function createNotificationFeed(): NotificationFeed {
  return {
    async fetch(): Promise<NotificationFeedItem[]> {
      const res = await request<{ data: NotificationFeedItem[] }>(
        getAuthenticatedClient(),
        'v1/notifications',
        { searchParams: { per_page: 50 } },
      )
      return res?.data ?? []
    },
    async markRead(id: string): Promise<void> {
      await request<void>(getAuthenticatedClient(), `v1/notifications/${id}/read`, { method: 'POST' })
    },
    async markAllRead(): Promise<void> {
      await request<void>(getAuthenticatedClient(), 'v1/notifications/read-all', {
        method: 'POST',
        json: {},
      })
    },
  }
}

const notificationFeedService: NotificationFeed = createNotificationFeed()

export default notificationFeedService

// Test-only: a fresh instance so MSW-backed specs don't share state.
export function __createFeedForTests(): NotificationFeed {
  return createNotificationFeed()
}
