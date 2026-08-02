import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

const toggleMonthlyMock = vi.fn()
const monthlyRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<{
    enabled: boolean
    recipientEmail: string
    schedule: string
    scope: string
    lastSentAt: string | null
  } | null>(null)
})

vi.mock('@/composables/useReports', () => ({
  useReports: () => ({
    monthly: monthlyRef,
    toggleMonthly: toggleMonthlyMock,
  }),
}))

import MonthlyReportCard from './MonthlyReportCard.vue'

const stubs = {
  UCard: {
    template: '<div data-testid="ucard"><slot /></div>',
    props: ['ui'],
  },
  UBadge: { template: '<span><slot /></span>', props: ['color', 'variant', 'size'] },
  UIcon: { template: '<span />', props: ['name'] },
  USwitch: {
    template:
      '<button :data-state="modelValue ? `on` : `off`" :disabled="disabled" @click="$emit(`update:modelValue`, !modelValue)"></button>',
    props: ['modelValue', 'disabled', 'ariaLabel'],
    emits: ['update:modelValue'],
  },
}

describe('MonthlyReportCard', () => {
  beforeEach(() => {
    toggleMonthlyMock.mockReset()
    monthlyRef.value = {
      enabled: false,
      recipientEmail: 'admin@example.com',
      schedule: 'monthly-1st',
      scope: 'all-resources',
      lastSentAt: null,
    }
  })

  afterEach(() => {
    monthlyRef.value = null
  })

  it('renders the 4 info columns (Recipient / Schedule / Scope / Last Sent)', () => {
    const wrapper = mount(MonthlyReportCard, { global: { stubs } })
    const info = wrapper.find('[data-testid="monthly-report-info"]')
    expect(info.exists()).toBe(true)
    expect(info.text()).toContain('Recipient')
    expect(info.text()).toContain('Schedule')
    expect(info.text()).toContain('Scope')
    expect(info.text()).toContain('Last Sent')
    expect(info.text()).toContain('admin@example.com')
    expect(info.text()).toContain('Never')
  })

  it('toggle click calls toggleMonthly(true) and reflects new state', async () => {
    toggleMonthlyMock.mockImplementation(async (next: boolean) => {
      monthlyRef.value = { ...(monthlyRef.value as NonNullable<typeof monthlyRef.value>), enabled: next }
    })
    const wrapper = mount(MonthlyReportCard, { global: { stubs }, attachTo: document.body })
    const toggle = wrapper.find('button[data-state]')
    expect(toggle.exists()).toBe(true)
    expect(toggle.attributes('data-state')).toMatch(/unchecked|off/)
    await toggle.trigger('click')
    expect(toggleMonthlyMock).toHaveBeenCalledWith(true)
    await nextTick()
    expect(wrapper.find('button[data-state]').attributes('data-state')).toMatch(/checked|on/)
    wrapper.unmount()
  })

  it('renders inline error "Add a monitor first" when service throws NO_RESOURCES', async () => {
    toggleMonthlyMock.mockRejectedValue(new Error('NO_RESOURCES'))
    const wrapper = mount(MonthlyReportCard, { global: { stubs } })
    await wrapper.find('[data-testid="monthly-report-toggle"]').trigger('click')
    await nextTick()
    await nextTick()
    expect(wrapper.text()).toContain('Add a monitor first')
  })

  it('shows next-send label when enabled', async () => {
    monthlyRef.value = {
      enabled: true,
      recipientEmail: 'admin@example.com',
      schedule: 'monthly-1st',
      scope: 'all-resources',
      lastSentAt: null,
    }
    const wrapper = mount(MonthlyReportCard, { global: { stubs } })
    expect(wrapper.find('[data-testid="monthly-report-info"]').text()).toContain('Next:')
  })
})
