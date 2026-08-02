import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { NotificationFeedItem } from '@/types'
import type { NotificationFeed } from '@/services/notificationFeedService'

function fixture(): NotificationFeedItem[] {
  return [
    {
      id: 'a',
      category: 'incident',
      severity: 'error',
      title: 'Down',
      occurredAt: new Date(Date.now() - 60_000).toISOString(),
      unread: true,
    },
    {
      id: 'b',
      category: 'system',
      severity: 'info',
      title: 'Maint',
      occurredAt: new Date(Date.now() - 120_000).toISOString(),
      unread: true,
    },
    {
      id: 'c',
      category: 'general',
      severity: 'info',
      title: 'News',
      occurredAt: new Date(Date.now() - 240_000).toISOString(),
      unread: false,
    },
  ]
}

function makeFakeFeed(items: NotificationFeedItem[]): NotificationFeed {
  const local = items.map((n) => ({ ...n }))
  return {
    async fetch() {
      return local.map((n) => ({ ...n }))
    },
    async markRead(id: string) {
      const t = local.find((n) => n.id === id)
      if (t) t.unread = false
    },
    async markAllRead() {
      for (const n of local) n.unread = false
    },
  }
}

import {
  useNotifications,
  __setNotificationFeedForTests,
  __resetNotificationsForTests,
} from './useNotifications'

describe('useNotifications', () => {
  beforeEach(() => {
    __resetNotificationsForTests()
  })

  afterEach(() => {
    __resetNotificationsForTests()
    vi.useRealTimers()
  })

  it('refresh hydrates items from the feed', async () => {
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
    const n = useNotifications()
    await n.refresh()
    expect(n.items.value.length).toBe(3)
  })

  it('unreadCount excludes locally-read items', async () => {
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
    const n = useNotifications()
    await n.refresh()
    expect(n.unreadCount.value).toBe(2)
    await n.markRead('a')
    expect(n.unreadCount.value).toBe(1)
  })

  it('markAllRead clears the badge', async () => {
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
    const n = useNotifications()
    await n.refresh()
    await n.markAllRead()
    expect(n.unreadCount.value).toBe(0)
  })

  it('tab filter routes by category', async () => {
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
    const n = useNotifications()
    await n.refresh()
    n.setTab('incident')
    expect(n.filteredItems.value.map((i) => i.id)).toEqual(['a'])
    n.setTab('system')
    expect(n.filteredItems.value.map((i) => i.id)).toEqual(['b'])
    n.setTab('all')
    expect(n.filteredItems.value.length).toBe(3)
  })

  it('start polls every 30s when visible and stop clears the timer', async () => {
    vi.useFakeTimers()
    const fakeFeed = makeFakeFeed(fixture())
    const spy = vi.spyOn(fakeFeed, 'fetch')
    __setNotificationFeedForTests(fakeFeed)
    const n = useNotifications()
    n.start()
    await Promise.resolve()
    expect(spy).toHaveBeenCalledTimes(1) // initial refresh
    vi.advanceTimersByTime(30_000)
    expect(spy).toHaveBeenCalledTimes(2)
    vi.advanceTimersByTime(30_000)
    expect(spy).toHaveBeenCalledTimes(3)
    n.stop()
    vi.advanceTimersByTime(30_000)
    expect(spy).toHaveBeenCalledTimes(3) // no more after stop
  })

  it('pauses polling when document becomes hidden', async () => {
    vi.useFakeTimers()
    const fakeFeed = makeFakeFeed(fixture())
    const spy = vi.spyOn(fakeFeed, 'fetch')
    __setNotificationFeedForTests(fakeFeed)
    const n = useNotifications()
    n.start()
    await Promise.resolve()
    expect(spy).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'hidden',
    })
    document.dispatchEvent(new Event('visibilitychange'))
    vi.advanceTimersByTime(60_000)
    expect(spy).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'visible',
    })
    document.dispatchEvent(new Event('visibilitychange'))
    await Promise.resolve()
    expect(spy).toHaveBeenCalledTimes(2) // refresh on resume
    n.stop()
  })

  it('exposes per-tab counts', async () => {
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
    const n = useNotifications()
    await n.refresh()
    expect(n.tabCounts.value).toEqual({ all: 3, incident: 1, system: 1 })
  })
})
