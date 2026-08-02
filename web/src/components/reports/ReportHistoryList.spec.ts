import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ReportHistoryEntry } from '@/types'
import ReportHistoryList from './ReportHistoryList.vue'

const stubs = {
  UCard: { template: '<div><slot /></div>', props: ['ui'] },
  UBadge: {
    template: '<span class="ubadge-stub" :data-color="color"><slot /></span>',
    props: ['color', 'variant', 'size'],
    inheritAttrs: true,
  },
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template:
      '<button :aria-label="ariaLabel" @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'icon'],
    emits: ['click'],
    computed: {
      ariaLabel(this: { $attrs: Record<string, unknown> }) {
        return (this.$attrs['aria-label'] as string) ?? ''
      },
    },
  },
}

function makeEntries(): ReportHistoryEntry[] {
  return [
    {
      id: 'h1',
      period: 'May 2026',
      sentAt: '2026-06-01T08:00:00Z',
      status: 'delivered',
      uptimePct: 99.94,
      incidentCount: 3,
      downtimeSeconds: 1620,
      recipientEmail: 'admin@example.com',
      resourceBreakdown: [],
    },
    {
      id: 'h2',
      period: 'April 2026',
      sentAt: '2026-05-01T08:00:00Z',
      status: 'pending',
      uptimePct: 99.5,
      incidentCount: 5,
      downtimeSeconds: 9000,
      recipientEmail: 'admin@example.com',
      resourceBreakdown: [],
    },
  ]
}

describe('ReportHistoryList', () => {
  it('renders one row per entry with uptime + incidents + downtime', () => {
    const wrapper = mount(ReportHistoryList, {
      global: { stubs },
      props: { entries: makeEntries() },
    })
    expect(wrapper.find('[data-testid="history-row-h1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="history-row-h2"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('May 2026')
    expect(wrapper.text()).toContain('99.94% uptime')
    expect(wrapper.text()).toContain('3 incidents')
  })

  it('renders status badge with the right label per entry', () => {
    const wrapper = mount(ReportHistoryList, {
      global: { stubs },
      props: { entries: makeEntries() },
    })
    expect(wrapper.find('[data-testid="history-status-h1"]').text()).toBe('Delivered')
    expect(wrapper.find('[data-testid="history-status-h2"]').text()).toBe('Pending')
  })

  it('emits `select` with the entry id when View is clicked', async () => {
    const wrapper = mount(ReportHistoryList, {
      global: { stubs },
      props: { entries: makeEntries() },
    })
    await wrapper.find('[data-testid="history-row-h1"] button[aria-label="View report"]').trigger(
      'click',
    )
    expect(wrapper.emitted('select')).toEqual([['h1']])
  })

  it('renders empty state when entries is empty', () => {
    const wrapper = mount(ReportHistoryList, { global: { stubs }, props: { entries: [] } })
    expect(wrapper.find('[data-testid="history-empty"]').exists()).toBe(true)
  })

  it('highlights the selected row', () => {
    const wrapper = mount(ReportHistoryList, {
      global: { stubs },
      props: { entries: makeEntries(), selectedId: 'h2' },
    })
    const row = wrapper.find('[data-testid="history-row-h2"]')
    expect(row.classes()).toContain('bg-elevated')
  })
})
