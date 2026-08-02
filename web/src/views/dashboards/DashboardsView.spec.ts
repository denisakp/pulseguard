import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

const filterRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<'all' | 'mine' | 'shared' | 'starred'>('all')
})
const sortRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<'updated' | 'name'>('updated')
})
const dashboardsRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<unknown[]>([])
})
const filteredSortedRef = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.computed(() => dashboardsRef.value)
})

const loadMock = vi.fn(async () => {})
const setFilterMock = vi.fn((f: 'all' | 'mine' | 'shared' | 'starred') => {
  filterRef.value = f
})
const setSortMock = vi.fn((s: 'updated' | 'name') => {
  sortRef.value = s
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards', name: 'Dashboards' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useDashboards', () => ({
  useDashboards: () => ({
    dashboards: dashboardsRef,
    filteredSorted: filteredSortedRef,
    filter: filterRef,
    sort: sortRef,
    load: loadMock,
    setFilter: setFilterMock,
    setSort: setSortMock,
    isStarred: () => false,
    toggleStar: vi.fn(),
  }),
}))

import DashboardsView from './DashboardsView.vue'

const stubs = {
  UButton: {
    template: '<button @click="$emit(\'click\')" v-bind="$attrs"><slot /></button>',
    props: ['color', 'variant', 'size', 'icon'],
    emits: ['click'],
    inheritAttrs: false,
  },
  UIcon: { template: '<span />', props: ['name'] },
  DashboardCard: {
    template:
      '<button :data-testid="`dashboard-card-${dashboard.id}`">{{ dashboard.name }}</button>',
    props: ['dashboard', 'health'],
  },
  DashboardWizardModal: {
    template: '<div v-if="open" data-testid="wizard-mount" />',
    props: ['open'],
    emits: ['update:open'],
  },
}

function fixture() {
  return [
    {
      id: 'd1',
      name: 'A dashboard',
      scope: { mode: 'tag', payload: { tagIds: ['x'] } },
      widgets: [{ id: 'w1', widgetTypeId: 'uptime-stat', position: 0 }],
      defaultTimeRange: '24h',
      refreshInterval: '30s',
      visibility: 'private',
      ownerId: 'user-default',
      ownerName: 'You',
      createdAt: '2026-05-01T10:00:00Z',
      updatedAt: '2026-06-09T10:00:00Z',
    },
    {
      id: 'd2',
      name: 'Z dashboard',
      scope: { mode: 'type', payload: { types: ['http'] } },
      widgets: [{ id: 'w2', widgetTypeId: 'response-time', position: 0 }],
      defaultTimeRange: '7d',
      refreshInterval: '1m',
      visibility: 'private',
      ownerId: 'user-default',
      ownerName: 'You',
      createdAt: '2026-04-01T10:00:00Z',
      updatedAt: '2026-06-08T10:00:00Z',
    },
  ]
}

describe('DashboardsView', () => {
  beforeEach(() => {
    loadMock.mockClear()
    setFilterMock.mockClear()
    setSortMock.mockClear()
    filterRef.value = 'all'
    sortRef.value = 'updated'
    dashboardsRef.value = fixture()
  })

  afterEach(() => {
    dashboardsRef.value = []
  })

  it('calls load on mount', () => {
    mount(DashboardsView, { global: { stubs } })
    expect(loadMock).toHaveBeenCalled()
  })

  it('renders one DashboardCard per item + placeholder card', async () => {
    const wrapper = mount(DashboardsView, { global: { stubs } })
    await nextTick()
    expect(wrapper.find('[data-testid="dashboard-card-d1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboard-card-d2"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="dashboards-placeholder-card"]').exists()).toBe(true)
  })

  it('placeholder card click opens the wizard', async () => {
    const wrapper = mount(DashboardsView, { global: { stubs } })
    await nextTick()
    await wrapper.find('[data-testid="dashboards-placeholder-card"]').trigger('click')
    await nextTick()
    expect(wrapper.find('[data-testid="wizard-mount"]').exists()).toBe(true)
  })

  it('"Shared" filter on CE shows the EE empty state', async () => {
    dashboardsRef.value = []
    filterRef.value = 'shared'
    const wrapper = mount(DashboardsView, { global: { stubs } })
    await nextTick()
    expect(wrapper.find('[data-testid="dashboards-shared-empty"]').exists()).toBe(true)
  })

  it('filter pill click calls setFilter', async () => {
    const wrapper = mount(DashboardsView, { global: { stubs } })
    await nextTick()
    await wrapper.find('[data-testid="filter-pill-mine"]').trigger('click')
    expect(setFilterMock).toHaveBeenCalledWith('mine')
  })

  it('sort select change calls setSort', async () => {
    const wrapper = mount(DashboardsView, { global: { stubs } })
    await nextTick()
    const select = wrapper.find('[data-testid="dashboards-sort"]')
    await select.setValue('name')
    expect(setSortMock).toHaveBeenCalledWith('name')
  })
})
