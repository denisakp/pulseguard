import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Resource } from '@/types'
import type { ResolvedResource } from '@/composables/useDashboardData'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards/x', name: 'DashboardDetail' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import ResourceStatusGridWidget from './ResourceStatusGridWidget.vue'

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
    ...over,
  }
}

describe('ResourceStatusGridWidget', () => {
  it('renders one cell per resource with status-driven data attribute', () => {
    const cells: ResolvedResource[] = [
      { id: 'r1', resource: makeResource({ status: 'up' }) },
      { id: 'r2', resource: makeResource({ id: 'r2', status: 'down' }) },
    ]
    const wrapper = mount(ResourceStatusGridWidget, {
      global: { stubs },
      props: { resources: cells, loading: false },
    })
    expect(wrapper.find('[data-testid="grid-cell-r1"]').attributes('data-status')).toBe('up')
    expect(wrapper.find('[data-testid="grid-cell-r2"]').attributes('data-status')).toBe('down')
  })

  it('tooltip / aria-label includes the resource name and status', () => {
    const cells: ResolvedResource[] = [{ id: 'r1', resource: makeResource({ status: 'up' }) }]
    const wrapper = mount(ResourceStatusGridWidget, {
      global: { stubs },
      props: { resources: cells, loading: false },
    })
    const cell = wrapper.find('[data-testid="grid-cell-r1"]')
    expect(cell.attributes('aria-label')).toContain('api')
    expect(cell.attributes('aria-label')).toContain('up')
  })

  it('renders a tombstone cell for a deleted resource (FR-024)', () => {
    const cells: ResolvedResource[] = [{ id: 'ghost', resource: null }]
    const wrapper = mount(ResourceStatusGridWidget, {
      global: { stubs },
      props: { resources: cells, loading: false },
    })
    const cell = wrapper.find('[data-testid="grid-tombstone-ghost"]')
    expect(cell.exists()).toBe(true)
    expect(cell.attributes('data-status')).toBe('tombstone')
    expect(cell.attributes('aria-label')).toBe('Resource removed')
  })

  it('cell click navigates to resource detail (skipped for tombstones)', async () => {
    const cells: ResolvedResource[] = [
      { id: 'r1', resource: makeResource({ status: 'up' }) },
      { id: 'ghost', resource: null },
    ]
    const wrapper = mount(ResourceStatusGridWidget, {
      global: { stubs },
      props: { resources: cells, loading: false },
    })
    await wrapper.find('[data-testid="grid-cell-r1"]').trigger('click')
    expect(pushMock).toHaveBeenCalledWith({ name: 'ResourceDetail', params: { id: 'r1' } })
    pushMock.mockClear()
    await wrapper.find('[data-testid="grid-tombstone-ghost"]').trigger('click')
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('renders skeleton on initial load', () => {
    const wrapper = mount(ResourceStatusGridWidget, {
      global: { stubs },
      props: { resources: [], loading: true },
    })
    expect(wrapper.find('[data-testid="grid-skeleton"]').exists()).toBe(true)
  })
})
