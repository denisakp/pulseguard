import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { Dashboard } from '@/types'

const pushMock = vi.fn()
const replaceMock = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: replaceMock }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards/x', name: 'DashboardDetail' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

const authUserId = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<string | null>('user-default')
})

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({
    get userId() {
      return authUserId.value
    },
    email: 'me@example.com',
  }),
}))

const dashboardToReturn = await vi.hoisted(async () => {
  const vue = await import('vue')
  return vue.ref<Dashboard | null>(null)
})

const isStarredMock = vi.fn(() => false)
const toggleStarMock = vi.fn()
const saveLayoutMock = vi.fn(async (_id: string, widgets: unknown[]) => ({
  ...(dashboardToReturn.value as Record<string, unknown>),
  widgets,
  updatedAt: 'new',
}))

vi.mock('@/composables/useDashboards', () => ({
  useDashboards: () => ({
    get: vi.fn(async () => dashboardToReturn.value),
    isStarred: isStarredMock,
    toggleStar: toggleStarMock,
    saveLayout: saveLayoutMock,
  }),
}))

const useConfirmMock = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: (...args: unknown[]) => useConfirmMock(...args),
}))

const dashboardDataStubs = await vi.hoisted(async () => {
  const vue = await import('vue')
  return {
    loading: vue.ref(false),
    resolved: vue.ref<unknown[]>([]),
    resources: vue.ref<unknown[]>([]),
    activeIncidents: vue.ref<unknown[]>([]),
    incidentsInRange: vue.ref<unknown[]>([]),
    incidents: vue.ref<unknown[]>([]),
    aggregateStatus: vue.computed(() => 'operational' as const),
    error: vue.ref<string | null>(null),
  }
})

vi.mock('@/composables/useDashboardData', () => ({
  useDashboardData: () => ({
    ...dashboardDataStubs,
    start: vi.fn(),
    stop: vi.fn(),
    refresh: vi.fn(),
  }),
}))

const toastAddMock = vi.fn()
;(globalThis as Record<string, unknown>).useToast = () => ({ add: toastAddMock })

import DashboardDetailView from './DashboardDetailView.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  USkeleton: { template: '<div />' },
  UButton: {
    template: '<button :disabled="disabled" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'icon', 'to', 'disabled', 'loading'],
    emits: ['click'],
    inheritAttrs: false,
  },
  DashboardEditBanner: {
    template:
      '<div data-testid="edit-banner-stub"><button data-testid="banner-save" @click="$emit(\'save\')">save</button><button data-testid="banner-cancel" @click="$emit(\'cancel\')">cancel</button></div>',
    props: ['saving', 'dirty'],
    emits: ['save', 'cancel'],
  },
  draggable: {
    template:
      '<div data-testid="draggable-stub"><slot v-for="el in modelValue" :element="el" :key="el.id" name="item" /></div>',
    props: ['modelValue', 'itemKey', 'handle', 'animation'],
    emits: ['update:modelValue'],
  },
  UptimeStatWidget: { template: '<div data-testid="w-uptime" />', props: ['resources', 'loading'] },
  IncidentsListWidget: { template: '<div data-testid="w-incidents" />', props: ['resources', 'loading'] },
  ResponseTimeWidget: { template: '<div data-testid="w-rt" />', props: ['resources', 'loading'] },
  ResourceStatusGridWidget: { template: '<div data-testid="w-grid" />', props: ['resources', 'loading'] },
}

function makeDashboard(over: Partial<Dashboard> = {}): Dashboard {
  return {
    id: 'd1',
    name: 'Test',
    scope: { mode: 'tag', payload: { tagIds: ['x'] } },
    widgets: [
      { id: 'w1', widgetTypeId: 'uptime-stat', position: 0 },
      { id: 'w2', widgetTypeId: 'incidents-list', position: 1 },
    ],
    defaultTimeRange: '24h',
    refreshInterval: '30s',
    visibility: 'private',
    ownerId: 'user-default',
    ownerName: 'You',
    createdAt: '',
    updatedAt: '',
    ...over,
  }
}

async function settle() {
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

describe('DashboardDetailView — read mode', () => {
  beforeEach(() => {
    pushMock.mockClear()
    replaceMock.mockClear()
    toastAddMock.mockClear()
    isStarredMock.mockReturnValue(false)
    authUserId.value = 'user-default'
    dashboardToReturn.value = makeDashboard()
  })

  afterEach(() => {
    dashboardToReturn.value = null
  })

  it('renders the widgets from the catalog', async () => {
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'd1' } })
    await settle()
    expect(wrapper.find('[data-testid="dashboard-widgets"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="widget-instance-w1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="widget-instance-w2"]').exists()).toBe(true)
  })

  it('shows Edit button when current user owns the dashboard', async () => {
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'd1' } })
    await settle()
    expect(wrapper.find('[data-testid="dashboard-edit-button"]').exists()).toBe(true)
  })

  it('hides Edit button when current user is not the owner', async () => {
    authUserId.value = 'other-user'
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'd1' } })
    await settle()
    expect(wrapper.find('[data-testid="dashboard-edit-button"]').exists()).toBe(false)
  })

  it('non-owner navigating directly to edit is redirected to read with a toast (FR-025)', async () => {
    authUserId.value = 'other-user'
    mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    expect(replaceMock).toHaveBeenCalledWith({ name: 'DashboardDetail', params: { id: 'd1' } })
  })

  it('renders not-found state when dashboard does not exist', async () => {
    dashboardToReturn.value = null
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'missing' } })
    await settle()
    expect(wrapper.find('[data-testid="dashboard-not-found"]').exists()).toBe(true)
  })

  it('time-range selector is initialised from dashboard.defaultTimeRange', async () => {
    dashboardToReturn.value = makeDashboard({ defaultTimeRange: '7d' })
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'd1' } })
    await settle()
    const select = wrapper.find('[data-testid="dashboard-time-range"]')
    expect((select.element as HTMLSelectElement).value).toBe('7d')
  })

  it('toggle star button calls toggleStar with the dashboard id', async () => {
    const wrapper = mount(DashboardDetailView, { global: { stubs }, props: { id: 'd1' } })
    await settle()
    await wrapper.find('[data-testid="dashboard-star"]').trigger('click')
    expect(toggleStarMock).toHaveBeenCalledWith('d1')
  })
})

describe('DashboardDetailView — edit mode', () => {
  beforeEach(() => {
    pushMock.mockClear()
    replaceMock.mockClear()
    saveLayoutMock.mockClear()
    useConfirmMock.mockReset().mockResolvedValue(true)
    isStarredMock.mockReturnValue(false)
    authUserId.value = 'user-default'
    dashboardToReturn.value = makeDashboard()
  })

  it('renders the edit banner when editMode prop is true', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    expect(wrapper.find('[data-testid="edit-banner-stub"]').exists()).toBe(true)
  })

  it('Save with no changes is a no-op (FR-030)', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    await wrapper.find('[data-testid="banner-save"]').trigger('click')
    await settle()
    expect(saveLayoutMock).not.toHaveBeenCalled()
    expect(replaceMock).toHaveBeenCalledWith({ name: 'DashboardDetail', params: { id: 'd1' } })
  })

  it('Save with zero widgets is blocked (FR-028)', async () => {
    dashboardToReturn.value = makeDashboard({ widgets: [] })
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    await wrapper.find('[data-testid="banner-save"]').trigger('click')
    await settle()
    expect(saveLayoutMock).not.toHaveBeenCalled()
  })

  it('Remove widget prompts confirm then drops it from working state', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    await wrapper.find('[data-testid="edit-remove-w1"]').trigger('click')
    await settle()
    expect(useConfirmMock).toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'destructive' }),
    )
    expect(wrapper.find('[data-testid="edit-widget-w1"]').exists()).toBe(false)
  })

  it('Cancel after a modification prompts the discard confirm', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    await wrapper.find('[data-testid="edit-remove-w1"]').trigger('click')
    await settle()
    useConfirmMock.mockClear()
    await wrapper.find('[data-testid="banner-cancel"]').trigger('click')
    await settle()
    expect(useConfirmMock).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Discard changes?' }),
    )
  })

  it('Add row of widgets exposes the picker, picking a widget appends', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    await wrapper.find('[data-testid="edit-add-row"]').trigger('click')
    await settle()
    expect(wrapper.find('[data-testid="edit-widget-picker"]').exists()).toBe(true)
    await wrapper.find('[data-testid="edit-add-uptime-stat"]').trigger('click')
    await settle()
    expect(wrapper.findAll('[data-testid^="edit-widget-"]').length).toBeGreaterThan(2)
  })

  it('Save with changes calls saveLayout then navigates back to read mode', async () => {
    const wrapper = mount(DashboardDetailView, {
      global: { stubs },
      props: { id: 'd1', editMode: true },
    })
    await settle()
    // Make a change: remove w1 (confirm auto-approves)
    await wrapper.find('[data-testid="edit-remove-w1"]').trigger('click')
    await settle()
    await wrapper.find('[data-testid="banner-save"]').trigger('click')
    await settle()
    expect(saveLayoutMock).toHaveBeenCalled()
    expect(replaceMock).toHaveBeenCalledWith({ name: 'DashboardDetail', params: { id: 'd1' } })
  })
})
