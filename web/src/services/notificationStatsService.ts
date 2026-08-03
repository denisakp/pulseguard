import { getAuthenticatedClient, request } from '@/core/http/client'

export interface NotificationStats {
  sent_30d: number
  pending: number
  failed_24h: number
}

export const fetchNotificationStats = async (): Promise<NotificationStats> => {
  const res = await request<{ data: NotificationStats }>(
    getAuthenticatedClient(),
    'v1/notifications/stats',
  )
  return res.data
}
