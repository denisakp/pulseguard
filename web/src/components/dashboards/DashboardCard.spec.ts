import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Dashboard, DashboardHealth } from '@/types'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards', name: 'Dashboards' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

const toggleStarMock = vi.fn()
const isStarredRef = vi.fn(() => false)

vi.mock('@/composables/useDashboards', () => ({
  useDashboards: () => ({
    toggleStar: toggleStarMock,
    isStarred: isStarredRef,
  }),
}))

vi.mock('@/widgets/widgetCatalog', () => ({
  getWidgetDefinition: () => ({ archetype: 'stat' }),
}))

import DashboardCard from './DashboardCard.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
}

function makeDashboard(): Dashboard {
  return {
    id: 'd1',
    name: 'Test dashboard',
    scope: { mode: 'tag', payload: { tagIds: ['production'] } },
    widgets: [
      { id: 'w1', widgetTypeId: 'uptime-stat', position: 0 },
      { id: 'w2', widgetTypeId: 'incidents-list', position: 1 },
    ],
    defaultTimeRange: '24h',
    refreshInterval: '30s',
    visibility: 'private',
    ownerId: 'user-default',
    ownerName: 'Alice',
    createdAt: '2026-05-15T10:00:00Z',
    updatedAt: new Date(Date.now() - 3 * 60_000).toISOString(),
  }
}

function makeHealth(): DashboardHealth {
  return { status: 'operational', summary: 'All healthy', resourceCount: 4 }
}

describe('DashboardCard', () => {
  beforeEach(() => {
    pushMock.mockClear()
    toggleStarMock.mockClear()
    isStarredRef.mockReturnValue(false)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders title, scope, health, widget count, owner', () => {
    const wrapper = mount(DashboardCard, {
      global: { stubs },
      props: { dashboard: makeDashboard(), health: makeHealth() },
    })
    expect(wrapper.text()).toContain('Test dashboard')
    expect(wrapper.text()).toContain('tag:production')
    expect(wrapper.text()).toContain('4 resources')
    expect(wrapper.text()).toContain('All healthy')
    expect(wrapper.text()).toContain('2 widgets')
    expect(wrapper.text()).toContain('Alice')
  })

  it('renders the Operational status pill', () => {
    const wrapper = mount(DashboardCard, {
      global: { stubs },
      props: { dashboard: makeDashboard(), health: makeHealth() },
    })
    expect(wrapper.find('[data-testid="dashboard-card-status"]').text()).toContain('Operational')
  })

  it('clicking the card navigates to detail', async () => {
    const wrapper = mount(DashboardCard, {
      global: { stubs },
      props: { dashboard: makeDashboard(), health: makeHealth() },
    })
    await wrapper.find('[data-testid="dashboard-card-d1"]').trigger('click')
    expect(pushMock).toHaveBeenCalledWith({ name: 'DashboardDetail', params: { id: 'd1' } })
  })

  it('clicking the star toggles state without navigating', async () => {
    const wrapper = mount(DashboardCard, {
      global: { stubs },
      props: { dashboard: makeDashboard(), health: makeHealth() },
    })
    await wrapper.find('[data-testid="dashboard-card-star-d1"]').trigger('click')
    expect(toggleStarMock).toHaveBeenCalledWith('d1')
    expect(pushMock).not.toHaveBeenCalled()
  })
})
