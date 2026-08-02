import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Host } from '@/types'

const listHostsMock = vi.fn()

vi.mock('@/services/hostsService', () => ({
  listHosts: () => listHostsMock(),
}))

function makeHost(over: Partial<Host>): Host {
  return {
    id: 'h1',
    name: 'web-1',
    os: 'linux',
    agentVersion: '1.0.0',
    online: true,
    lastSeenAt: '2026-07-30T10:00:00Z',
    lastCpuPct: 12,
    lastMemPct: 40,
    lastDiskPct: 55,
    lastNetIn: 100,
    lastNetOut: 200,
    lastDisks: [],
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-30T10:00:00Z',
    ...over,
  }
}

describe('hostStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listHostsMock.mockReset()
    vi.useRealTimers()
  })

  it('fetch populates hosts and count', async () => {
    listHostsMock.mockResolvedValue([makeHost({ id: 'h1' }), makeHost({ id: 'h2', name: 'db-1' })])
    const { useHostStore } = await import('../hostStore')
    const store = useHostStore()
    await store.fetch()
    expect(store.hosts).toHaveLength(2)
    expect(store.count).toBe(2)
    expect(store.error).toBeNull()
  })

  it('captures the error message when fetch fails', async () => {
    listHostsMock.mockRejectedValue(new Error('boom'))
    const { useHostStore } = await import('../hostStore')
    const store = useHostStore()
    await store.fetch()
    expect(store.hosts).toHaveLength(0)
    expect(store.error).toBe('boom')
    expect(store.loading).toBe(false)
  })

  it('startPolling fetches immediately then on interval; stopPolling clears it', async () => {
    vi.useFakeTimers()
    listHostsMock.mockResolvedValue([makeHost({})])
    const { useHostStore } = await import('../hostStore')
    const store = useHostStore()

    store.startPolling(5000)
    expect(listHostsMock).toHaveBeenCalledTimes(1) // immediate

    vi.advanceTimersByTime(5000)
    expect(listHostsMock).toHaveBeenCalledTimes(2)

    vi.advanceTimersByTime(5000)
    expect(listHostsMock).toHaveBeenCalledTimes(3)

    store.stopPolling()
    vi.advanceTimersByTime(15000)
    expect(listHostsMock).toHaveBeenCalledTimes(3) // no more after stop
  })
})
