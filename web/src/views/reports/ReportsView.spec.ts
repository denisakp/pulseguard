import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { ReportHistoryEntry } from '@/types'

const monthlyRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<unknown>(null)
})
const historyRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<ReportHistoryEntry[]>([])
})
const latestDeliveredRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.computed(() => historyRef.value.find((h) => h.status === 'delivered') ?? null)
})

const loadAllMock = vi.fn(async () => {})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/reports', name: 'Reports' }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useReports', () => ({
  useReports: () => ({
    monthly: monthlyRef,
    history: historyRef,
    latestDelivered: latestDeliveredRef,
    loadAll: loadAllMock,
    toggleMonthly: vi.fn(),
  }),
}))

import ReportsView from './ReportsView.vue'

const stubs = {
  UCard: { template: '<div><slot /></div>', props: ['ui'] },
  UBadge: { template: '<span><slot /></span>', props: ['color', 'variant', 'size'] },
  UIcon: { template: '<span />', props: ['name'] },
  USwitch: {
    template: '<button :disabled="disabled"><slot /></button>',
    props: ['modelValue', 'disabled'],
    emits: ['update:modelValue'],
  },
  UAlert: {
    template:
      '<div :data-testid="dataTestid"><slot /><slot name="actions" /></div>',
    props: ['color', 'variant', 'icon', 'title', 'description'],
    computed: {
      dataTestid(this: { $attrs: Record<string, unknown> }) {
        return (this.$attrs['data-testid'] as string) ?? ''
      },
    },
  },
  UButton: {
    template: '<a :href="to" v-bind="$attrs"><slot /></a>',
    props: ['color', 'variant', 'size', 'icon', 'to'],
    inheritAttrs: false,
  },
}

describe('ReportsView', () => {
  beforeEach(() => {
    loadAllMock.mockClear()
    monthlyRef.value = {
      enabled: false,
      recipientEmail: 'admin@example.com',
      schedule: 'monthly-1st',
      scope: 'all-resources',
      lastSentAt: null,
    }
    historyRef.value = []
  })

  afterEach(() => {
    monthlyRef.value = null
    historyRef.value = []
  })

  it('calls loadAll on mount', () => {
    mount(ReportsView, { global: { stubs } })
    expect(loadAllMock).toHaveBeenCalled()
  })

  it('renders the EE upsell banner + upgrade CTA', () => {
    const wrapper = mount(ReportsView, { global: { stubs } })
    expect(wrapper.find('[data-testid="reports-ee-banner"]').exists()).toBe(true)
    const html = wrapper.html()
    expect(html).toContain('reports-upgrade-cta')
    expect(html).toContain('/settings/account?tab=plan')
  })

  it('shows the preview empty state when no history exists', async () => {
    const wrapper = mount(ReportsView, { global: { stubs } })
    await nextTick()
    expect(wrapper.find('[data-testid="reports-preview-empty"]').exists()).toBe(true)
  })

  it('renders the inline preview when at least one delivered entry exists', async () => {
    historyRef.value = [
      {
        id: 'h1',
        period: 'May 2026',
        sentAt: '2026-06-01T08:00:00Z',
        status: 'delivered',
        uptimePct: 99.5,
        incidentCount: 2,
        downtimeSeconds: 1200,
        recipientEmail: 'admin@example.com',
        resourceBreakdown: [{ name: 'api', uptimePct: 99.9, incidents: 1 }],
      },
    ]
    const wrapper = mount(ReportsView, { global: { stubs } })
    await nextTick()
    expect(wrapper.find('[data-testid="report-preview-inline"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="reports-preview-empty"]').exists()).toBe(false)
  })
})
