import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Host } from '@/types'

vi.mock('@/services/hostsService', () => ({
  listHosts: vi.fn().mockResolvedValue([]),
  listMonitors: vi.fn().mockResolvedValue([]),
}))

function makeHost(over: Partial<Host>): Host {
  return {
    id: 'h1',
    name: 'web-1',
    os: 'linux',
    agentVersion: '1.0.0',
    online: true,
    lastSeenAt: '2026-07-30T10:00:00Z',
    lastCpuPct: 10,
    lastMemPct: 20,
    lastDiskPct: 30,
    lastNetIn: 0,
    lastNetOut: 0,
    lastDisks: [],
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-30T10:00:00Z',
    ...over,
  }
}

describe('useHosts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('sorts offline hosts first, then by worst load desc, then by name', async () => {
    const { useHostStore } = await import('@/stores/hostStore')
    const { useHosts } = await import('./useHosts')
    const hostStore = useHostStore()
    hostStore.hosts = [
      makeHost({ id: 'a', name: 'alpha', online: true, lastCpuPct: 10, lastMemPct: 10, lastDiskPct: 10 }),
      makeHost({ id: 'b', name: 'bravo', online: true, lastCpuPct: 90, lastMemPct: 10, lastDiskPct: 10 }),
      makeHost({ id: 'c', name: 'charlie', online: false, lastCpuPct: 5, lastMemPct: 5, lastDiskPct: 5 }),
    ]
    const { hosts } = useHosts()
    expect(hosts.value.map((h) => h.id)).toEqual(['c', 'b', 'a'])
  })

  it('computes worstLoad as max(cpu, mem, disk), treating null as 0', async () => {
    const { useHostStore } = await import('@/stores/hostStore')
    const { useHosts } = await import('./useHosts')
    const hostStore = useHostStore()
    hostStore.hosts = [makeHost({ id: 'a', lastCpuPct: 12, lastMemPct: null, lastDiskPct: 47 })]
    const { hosts } = useHosts()
    expect(hosts.value[0]!.worstLoad).toBe(47)
  })

  it('derives serviceCount from monitors linked via hostId', async () => {
    const { listHosts, listMonitors } = await import('@/services/hostsService')
    const { useHosts } = await import('./useHosts')
    const mon = (id: string, hostId: string | null) => ({
      id,
      name: id,
      type: 'http',
      status: 'up',
      lastCheckedAt: null,
      hostId,
    })
    vi.mocked(listHosts).mockResolvedValue([
      makeHost({ id: 'h1', name: 'web-1' }),
      makeHost({ id: 'h2', name: 'db-1' }),
    ])
    vi.mocked(listMonitors).mockResolvedValue([
      mon('m1', 'h1'),
      mon('m2', 'h1'),
      mon('m3', 'h2'),
      mon('m4', null),
    ])
    const { hosts, fetch } = useHosts()
    await fetch()
    const byId = Object.fromEntries(hosts.value.map((h) => [h.id, h.serviceCount]))
    expect(byId.h1).toBe(2)
    expect(byId.h2).toBe(1)
  })
})
