import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ResponseTimeWidget from './ResponseTimeWidget.vue'
import type { Resource } from '@/types'
import type { ResolvedResource } from '@/composables/useDashboardData'

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
    response_times: [
      { timestamp: '', response_time: 100 },
      { timestamp: '', response_time: 200 },
      { timestamp: '', response_time: 300 },
      { timestamp: '', response_time: 400 },
      { timestamp: '', response_time: 500 },
    ],
    ...over,
  }
}

describe('ResponseTimeWidget', () => {
  it('renders one row per resource with bar', () => {
    const r1: ResolvedResource = { id: 'r1', resource: makeResource({ name: 'api' }) }
    const r2: ResolvedResource = { id: 'r2', resource: makeResource({ name: 'web' }) }
    const wrapper = mount(ResponseTimeWidget, {
      global: { stubs },
      props: { resources: [r1, r2], loading: false },
    })
    expect(wrapper.text()).toContain('api')
    expect(wrapper.text()).toContain('web')
  })

  it('metric toggle switches percentile', async () => {
    const r: ResolvedResource = { id: 'r1', resource: makeResource({}) }
    const wrapper = mount(ResponseTimeWidget, {
      global: { stubs },
      props: { resources: [r], loading: false },
    })
    // p95 default — for 5 samples [100, 200, 300, 400, 500], floor(0.95*5)=4 → 500
    expect(wrapper.text()).toContain('500 ms')
    await wrapper.find('[data-testid="metric-p50"]').trigger('click')
    // p50 → floor(0.5*5)=2 → 300
    expect(wrapper.text()).toContain('300 ms')
  })

  it('renders tombstone notice when resource missing (FR-024)', () => {
    const ghost: ResolvedResource = { id: 'g', resource: null }
    const wrapper = mount(ResponseTimeWidget, {
      global: { stubs },
      props: { resources: [ghost], loading: false },
    })
    expect(wrapper.find('[data-testid="response-time-tombstone"]').exists()).toBe(true)
  })

  it('renders skeleton on initial load', () => {
    const wrapper = mount(ResponseTimeWidget, {
      global: { stubs },
      props: { resources: [], loading: true },
    })
    expect(wrapper.find('[data-testid="response-time-skeleton"]').exists()).toBe(true)
  })
})
