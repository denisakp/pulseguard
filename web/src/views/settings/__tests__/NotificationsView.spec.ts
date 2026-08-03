/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — spec 059 polish debt: index-access narrowing
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const fetchChannelsMock = vi.fn()
const createChannelMock = vi.fn()
const updateChannelMock = vi.fn()
const setDefaultMock = vi.fn()
const deleteChannelMock = vi.fn()

vi.mock('@/services/notificationChannelService', () => ({
  fetchChannels: (...a: unknown[]) => fetchChannelsMock(...a),
  createChannel: (...a: unknown[]) => createChannelMock(...a),
  updateChannel: (...a: unknown[]) => updateChannelMock(...a),
  setDefault: (...a: unknown[]) => setDefaultMock(...a),
  deleteChannel: (...a: unknown[]) => deleteChannelMock(...a),
}))

const fetchNotificationStatsMock = vi.fn().mockResolvedValue(null)
vi.mock('@/services/notificationStatsService', () => ({
  fetchNotificationStats: (...a: unknown[]) => fetchNotificationStatsMock(...a),
}))

const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: (opts: unknown) => confirmMock(opts),
}))

vi.mock('@/components/settings/notifications/ChannelModal.vue', () => ({
  default: {
    name: 'ChannelModal',
    template: '<div data-testid="modal" />',
    props: ['open', 'initial'],
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), resolve: () => ({ href: '#' }) }),
  useRoute: () => ({
    path: '/settings/notifications',
    params: {},
    query: {},
    name: 'SettingsNotifications',
  }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import NotificationsView from '../NotificationsView.vue'

type Channel = {
  id: string
  name: string
  type: 'smtp' | 'slack' | 'webhook'
  config: Record<string, unknown>
  enabled_by_default: boolean
}

type Vm = {
  channels: Channel[]
  stats: { label: string; value: string }[]
  openCreate: () => void
  onSubmit: (p: {
    type: string
    name: string
    is_default: boolean
    is_active: boolean
    config: Record<string, unknown>
  }) => Promise<void>
  onToggleDefault: (c: Channel) => Promise<void>
  onDelete: (c: Channel) => Promise<void>
}

beforeEach(() => {
  fetchChannelsMock.mockReset()
  createChannelMock.mockReset()
  updateChannelMock.mockReset()
  setDefaultMock.mockReset()
  deleteChannelMock.mockReset()
  confirmMock.mockReset()
})

describe('NotificationsView', () => {
  it('empty list renders an empty-state CTA', async () => {
    fetchChannelsMock.mockResolvedValue([])
    const w = mount(NotificationsView)
    await flushPromises()
    expect(w.text()).toContain('No notification channels yet')
  })

  it('stats row renders 4 KPIs with placeholders for backend-pending metrics', async () => {
    fetchChannelsMock.mockResolvedValue([
      { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: false },
    ])
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm
    expect(vm.stats.length).toBe(4)
    expect(vm.stats[0].label).toBe('CHANNELS')
    expect(vm.stats[0].value).toBe('1')
    // When the stats endpoint hasn't returned (or fails), the KPI shows
    // the em dash placeholder. The previous "Backend metric pending"
    // hint is gone now that the counters are wired to a real endpoint.
    expect(vm.stats[1].value).toBe('—')
  })

  it('Default toggle on row B → previous default A un-toggled (optimistic)', async () => {
    const a: Channel = { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: true }
    const b: Channel = { id: 'b', name: 'B', type: 'smtp', config: {}, enabled_by_default: false }
    fetchChannelsMock.mockResolvedValue([a, b])
    setDefaultMock.mockResolvedValue(undefined)
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm
    await vm.onToggleDefault(b)
    expect(vm.channels.find((c) => c.id === 'a')?.enabled_by_default).toBe(false)
    expect(vm.channels.find((c) => c.id === 'b')?.enabled_by_default).toBe(true)
    expect(setDefaultMock).toHaveBeenCalledWith('b')
  })

  it('Default toggle rollback on service failure', async () => {
    const a: Channel = { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: true }
    const b: Channel = { id: 'b', name: 'B', type: 'smtp', config: {}, enabled_by_default: false }
    fetchChannelsMock.mockResolvedValue([a, b])
    setDefaultMock.mockRejectedValue(new Error('422'))
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm
    await vm.onToggleDefault(b)
    expect(vm.channels.find((c) => c.id === 'a')?.enabled_by_default).toBe(true)
    expect(vm.channels.find((c) => c.id === 'b')?.enabled_by_default).toBe(false)
  })

  it('onSubmit creates channel via service and reloads list', async () => {
    fetchChannelsMock.mockResolvedValue([])
    createChannelMock.mockResolvedValue({ id: 'new' })
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm
    await vm.onSubmit({
      type: 'slack',
      name: 'oncall',
      is_default: false,
      is_active: true,
      config: { webhook_url: 'https://hooks.slack.com/x', channel: 'oncall' },
    })
    expect(createChannelMock).toHaveBeenCalled()
    expect(fetchChannelsMock).toHaveBeenCalledTimes(2)
  })

  it('onSubmit in edit mode strips a blank (masked) secret from the update payload', async () => {
    const chan = {
      id: 'c1',
      name: 'Ops',
      type: 'smtp',
      config: { host: 'h', port: 587, username: 'u', sender: 'a@b.co', recipient: 'c@d.co' },
      enabled_by_default: false,
    }
    fetchChannelsMock.mockResolvedValue([chan])
    updateChannelMock.mockResolvedValue({ id: 'c1' })
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm & { openEdit: (c: unknown) => void }
    vm.openEdit(chan)
    await vm.onSubmit({
      type: 'smtp',
      name: 'Ops',
      is_default: false,
      is_active: true,
      config: {
        host: 'h',
        port: 587,
        username: 'u',
        password: '',
        sender: 'a@b.co',
        recipient: 'c@d.co',
      },
    })
    const [id, payload] = updateChannelMock.mock.calls[0]
    expect(id).toBe('c1')
    expect(payload.config).not.toHaveProperty('password')
    expect(payload.config.host).toBe('h')
  })

  it('onSubmit in edit mode keeps a newly typed secret', async () => {
    const chan = {
      id: 'c1',
      name: 'Ops',
      type: 'smtp',
      config: { host: 'h', port: 587, username: 'u', sender: 'a@b.co', recipient: 'c@d.co' },
      enabled_by_default: false,
    }
    fetchChannelsMock.mockResolvedValue([chan])
    updateChannelMock.mockResolvedValue({ id: 'c1' })
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm & { openEdit: (c: unknown) => void }
    vm.openEdit(chan)
    await vm.onSubmit({
      type: 'smtp',
      name: 'Ops',
      is_default: false,
      is_active: true,
      config: {
        host: 'h',
        port: 587,
        username: 'u',
        password: 'n3w',
        sender: 'a@b.co',
        recipient: 'c@d.co',
      },
    })
    const [, payload] = updateChannelMock.mock.calls[0]
    expect(payload.config.password).toBe('n3w')
  })

  it('onDelete with confirm true → service deleteChannel called and row removed', async () => {
    const a: Channel = { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: false }
    fetchChannelsMock.mockResolvedValue([a])
    confirmMock.mockResolvedValue(true)
    deleteChannelMock.mockResolvedValue(undefined)
    const w = mount(NotificationsView)
    await flushPromises()
    const vm = w.vm as unknown as Vm
    await vm.onDelete(a)
    expect(deleteChannelMock).toHaveBeenCalledWith('a')
    expect(vm.channels.find((c) => c.id === 'a')).toBeUndefined()
  })
})
