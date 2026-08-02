import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { NotificationFeedItem } from '@/types'
import type { NotificationFeed } from '@/services/notificationFeedService'

function fixture(): NotificationFeedItem[] {
  return [
    {
      id: 'a',
      category: 'incident',
      severity: 'error',
      title: 'Resource down',
      description: 'API failed',
      occurredAt: new Date(Date.now() - 60_000).toISOString(),
      unread: true,
    },
    {
      id: 'b',
      category: 'system',
      severity: 'info',
      title: 'Maintenance scheduled',
      occurredAt: new Date(Date.now() - 120_000).toISOString(),
      unread: true,
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

import UNotificationDropdown from './UNotificationDropdown.vue'
import {
  __setNotificationFeedForTests,
  __resetNotificationsForTests,
} from '@/composables/useNotifications'

const stubs = {
  UPopover: {
    template: '<div data-testid="popover"><slot /><slot name="content" /></div>',
    props: ['ui'],
  },
  UButton: {
    template: '<button :aria-label="ariaLabel"><slot name="leading" /><slot /></button>',
    props: ['color', 'variant', 'size', 'icon', 'to', 'external'],
    computed: {
      ariaLabel(this: { $attrs: Record<string, unknown> }) {
        return (this.$attrs['aria-label'] as string) ?? ''
      },
    },
  },
  UIcon: { template: '<span />', props: ['name'] },
}

describe('UNotificationDropdown', () => {
  let wrapper: ReturnType<typeof mount> | null = null

  beforeEach(() => {
    __resetNotificationsForTests()
    __setNotificationFeedForTests(makeFakeFeed(fixture()))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    __resetNotificationsForTests()
    vi.useRealTimers()
  })

  it('renders unread badge with the count from the fixture', async () => {
    wrapper = mount(UNotificationDropdown, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await Promise.resolve()
    await nextTick()
    const badge = wrapper.find('[data-testid="notification-badge"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('2')
  })

  it('exposes the bell aria-label including the unread count', async () => {
    wrapper = mount(UNotificationDropdown, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await Promise.resolve()
    await nextTick()
    const bell = wrapper.find('button[aria-label*="Notifications"]')
    expect(bell.exists()).toBe(true)
    expect(bell.attributes('aria-label')).toContain('2 unread')
  })

  it('hides the badge once everything is marked read', async () => {
    wrapper = mount(UNotificationDropdown, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await Promise.resolve()
    await nextTick()
    expect(wrapper.find('[data-testid="notification-badge"]').exists()).toBe(true)

    // Drive state via the composable directly — the dropdown content is mounted
    // by the real UPopover behind a teleport in Nuxt UI's component, which we
    // do not exercise here (covered by useNotifications.spec.ts).
    const { useNotifications } = await import('@/composables/useNotifications')
    await useNotifications().markAllRead()
    await nextTick()
    expect(wrapper.find('[data-testid="notification-badge"]').exists()).toBe(false)
  })
})
