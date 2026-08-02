import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import UptimeStatWidget from './UptimeStatWidget.vue'
import type { ResolvedResource } from '@/composables/useDashboardData'
import type { Resource } from '@/types'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  USkeleton: { template: '<div data-testid="skeleton" />' },
}

function makeResource(over: Partial<Resource>): Resource {
  return {
    id: 'r1',
    name: 'api',
    type: 'http',
    target: '',
    interval: 60,
    timeout: 30,
    status: 'up',
    is_active: true,
    failure_count: 0,
    confirmation_checks: 2,
    confirmation_interval: 30,
    created_at: '',
    updated_at: '',
    uptime_30d: 0.997,
    hourly_uptime: [
      { hour: '2026-06-09T10:00:00Z', uptime_percent: 100, successful_count: 12, total_count: 12 },
      { hour: '2026-06-09T11:00:00Z', uptime_percent: 99, successful_count: 11, total_count: 12 },
    ],
    ...over,
  }
}

describe('UptimeStatWidget', () => {
  it('renders skeleton while loading and no data', () => {
    const wrapper = mount(UptimeStatWidget, {
      global: { stubs },
      props: { resources: [], loading: true },
    })
    expect(wrapper.find('[data-testid="uptime-widget-skeleton"]').exists()).toBe(true)
  })

  it('renders aggregate uptime as percent (uptime_30d × 100)', () => {
    const r: ResolvedResource = { id: 'r1', resource: makeResource({}) }
    const wrapper = mount(UptimeStatWidget, {
      global: { stubs },
      props: { resources: [r], loading: false },
    })
    expect(wrapper.text()).toContain('99.70')
    expect(wrapper.text()).toContain('%')
  })

  it('renders 24-bucket sparkline when hourly_uptime is present', () => {
    const r: ResolvedResource = { id: 'r1', resource: makeResource({}) }
    const wrapper = mount(UptimeStatWidget, {
      global: { stubs },
      props: { resources: [r], loading: false },
    })
    // 2 buckets → 2 bars rendered
    expect(wrapper.findAll('span[style*="height"]').length).toBeGreaterThanOrEqual(2)
  })

  it('renders tombstone notice for deleted resources (FR-024)', () => {
    const r: ResolvedResource = { id: 'r1', resource: makeResource({}) }
    const ghost: ResolvedResource = { id: 'ghost', resource: null }
    const wrapper = mount(UptimeStatWidget, {
      global: { stubs },
      props: { resources: [r, ghost], loading: false },
    })
    expect(wrapper.find('[data-testid="uptime-tombstone"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('1 resource removed')
  })

  it('renders em-dash when no uptime data available', () => {
    const r: ResolvedResource = { id: 'r1', resource: makeResource({ uptime_30d: undefined, uptime_7d: undefined, uptime: undefined, hourly_uptime: [] }) }
    const wrapper = mount(UptimeStatWidget, {
      global: { stubs },
      props: { resources: [r], loading: false },
    })
    expect(wrapper.text()).toContain('—')
  })
})
