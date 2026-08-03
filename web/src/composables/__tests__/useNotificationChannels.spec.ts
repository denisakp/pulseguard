import { describe, expect, it, vi, beforeEach } from 'vitest'

const fetchChannelsMock = vi.fn()
const createChannelMock = vi.fn()
const updateChannelMock = vi.fn()
const deleteChannelMock = vi.fn()
const testChannelMock = vi.fn()
const testChannelConfigMock = vi.fn()

vi.mock('@/services/notificationChannelService', () => ({
  fetchChannels: (...a: unknown[]) => fetchChannelsMock(...a),
  createChannel: (...a: unknown[]) => createChannelMock(...a),
  updateChannel: (...a: unknown[]) => updateChannelMock(...a),
  deleteChannel: (...a: unknown[]) => deleteChannelMock(...a),
  testChannel: (...a: unknown[]) => testChannelMock(...a),
  testChannelConfig: (...a: unknown[]) => testChannelConfigMock(...a),
}))

import { useNotificationChannels } from '../useNotificationChannels'

beforeEach(() => {
  fetchChannelsMock.mockReset()
  createChannelMock.mockReset()
  updateChannelMock.mockReset()
  deleteChannelMock.mockReset()
  testChannelMock.mockReset()
  testChannelConfigMock.mockReset()
})

describe('useNotificationChannels', () => {
  it('loadChannels hydrates channels from the (v1) service', async () => {
    const list = [{ id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: false }]
    fetchChannelsMock.mockResolvedValue(list)
    const c = useNotificationChannels()
    await c.loadChannels()
    expect(c.channels.value).toEqual(list)
    expect(c.loading.value).toBe(false)
    expect(c.error.value).toBeNull()
  })

  it('loadChannels surfaces an error message and rethrows on failure', async () => {
    fetchChannelsMock.mockRejectedValue(new Error('boom'))
    const c = useNotificationChannels()
    await expect(c.loadChannels()).rejects.toThrow('boom')
    expect(c.error.value).toBe('boom')
    expect(c.loading.value).toBe(false)
  })

  it('addChannel appends the created channel', async () => {
    fetchChannelsMock.mockResolvedValue([])
    createChannelMock.mockResolvedValue({ id: 'new', name: 'N', type: 'slack', config: {} })
    const c = useNotificationChannels()
    await c.loadChannels()
    const created = await c.addChannel({ name: 'N', type: 'slack', config: {}, enabled_by_default: false })
    expect(created.id).toBe('new')
    expect(c.channels.value.map((x) => x.id)).toEqual(['new'])
  })

  it('updateChannel replaces the matching row', async () => {
    const a = { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: false }
    fetchChannelsMock.mockResolvedValue([a])
    updateChannelMock.mockResolvedValue({ ...a, name: 'A2' })
    const c = useNotificationChannels()
    await c.loadChannels()
    await c.updateChannel('a', { name: 'A2' })
    expect(c.channels.value.find((x) => x.id === 'a')?.name).toBe('A2')
  })

  it('deleteChannel removes the row', async () => {
    const a = { id: 'a', name: 'A', type: 'slack', config: {}, enabled_by_default: false }
    fetchChannelsMock.mockResolvedValue([a])
    deleteChannelMock.mockResolvedValue(undefined)
    const c = useNotificationChannels()
    await c.loadChannels()
    await c.deleteChannel('a')
    expect(c.channels.value.find((x) => x.id === 'a')).toBeUndefined()
  })

  it('testChannel and testChannelConfig delegate to the service', async () => {
    testChannelMock.mockResolvedValue(undefined)
    testChannelConfigMock.mockResolvedValue(undefined)
    const c = useNotificationChannels()
    await c.testChannel('a')
    await c.testChannelConfig({ type: 'slack', config: {} })
    expect(testChannelMock).toHaveBeenCalledWith('a')
    expect(testChannelConfigMock).toHaveBeenCalledWith({ type: 'slack', config: {} })
  })
})
