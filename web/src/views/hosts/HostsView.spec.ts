import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { Host } from '@/types'

function makeHost(over: Partial<Host> & { serviceCount?: number; worstLoad?: number }): Host & {
  serviceCount: number
  worstLoad: number
} {
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
    lastNetIn: 0,
    lastNetOut: 0,
    lastDisks: [],
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-30T10:00:00Z',
    serviceCount: 3,
    worstLoad: 55,
    ...over,
  }
}

const hostsRef = ref<ReturnType<typeof makeHost>[]>([])
const loadingRef = ref(false)
const countRef = ref(0)
const errorRef = ref<string | null>(null)
const fetchMock = vi.fn().mockResolvedValue(undefined)
const startPolling = vi.fn()
const stopPolling = vi.fn()

vi.mock('@/composables/useHosts', () => ({
  useHosts: () => ({
    hosts: hostsRef,
    loading: loadingRef,
    error: errorRef,
    count: countRef,
    fetch: fetchMock,
    startPolling,
    stopPolling,
  }),
}))

const useConfirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: (opts: unknown) => useConfirmMock(opts),
}))

const rotateCredentialMock = vi.fn()
const deleteHostMock = vi.fn()
vi.mock('@/services/hostsService', () => ({
  rotateCredential: (id: string) => rotateCredentialMock(id),
  deleteHost: (id: string) => deleteHostMock(id),
}))

import HostsView from './HostsView.vue'

// NuxtUI components register under their real names (no `U` prefix); stub by
// those so the DOM assertions hit our lightweight replacements.
const stubs = {
  Button: { template: '<button><slot /></button>' },
  Alert: { template: '<div />' },
  Empty: { template: '<div data-testid="empty"><slot name="actions" /></div>' },
  Table: { name: 'Table', template: '<div data-testid="table" />', props: ['columns', 'data', 'loading'] },
  Modal: { template: '<div><slot name="body" /></div>' },
  RegisterHostModal: { name: 'RegisterHostModal', template: '<div data-testid="register-modal" />' },
  HostStatusBadge: { template: '<span />' },
  HostCredentialReveal: { template: '<div />' },
}

function build() {
  return mount(HostsView, { global: { stubs } })
}

// render a column cell to its text content (cells are single-span vnodes).
function cellText(col: { cell?: (ctx: unknown) => { children?: unknown } }, host: unknown): string {
  const vnode = col.cell?.({ row: { original: host } }) as { children?: unknown }
  return String(vnode?.children ?? '')
}

beforeEach(() => {
  hostsRef.value = []
  loadingRef.value = false
  countRef.value = 0
  errorRef.value = null
  useConfirmMock.mockReset()
  rotateCredentialMock.mockReset()
  deleteHostMock.mockReset()
  fetchMock.mockClear()
})

describe('HostsView', () => {
  it('exposes columns with the expected ids', () => {
    const w = build()
    const cols = (w.vm as unknown as { columns: Array<{ id: string }> }).columns.map((c) => c.id)
    expect(cols).toEqual(['name', 'cpu', 'mem', 'disk', 'services', 'last_seen', 'actions'])
  })

  it('renders metric percentages for online hosts and dashes for offline ones', () => {
    const w = build()
    const cols = (
      w.vm as unknown as {
        columns: Array<{ id: string; cell?: (c: unknown) => { children?: unknown } }>
      }
    ).columns
    const cpu = cols.find((c) => c.id === 'cpu')!
    const disk = cols.find((c) => c.id === 'disk')!
    const services = cols.find((c) => c.id === 'services')!
    const lastSeen = cols.find((c) => c.id === 'last_seen')!

    const online = makeHost({ online: true, lastCpuPct: 12, lastDiskPct: 55, serviceCount: 3 })
    const offline = makeHost({
      online: false,
      lastCpuPct: 12,
      lastDiskPct: 55,
      lastSeenAt: null,
      serviceCount: 0,
    })

    expect(cellText(cpu, online)).toBe('12%')
    expect(cellText(cpu, offline)).toBe('—') // dashed when offline
    expect(cellText(disk, online)).toBe('55%')
    expect(cellText(services, online)).toBe('3')
    expect(cellText(lastSeen, offline)).toBe('—') // dashed when never seen
  })

  it('shows the onboarding empty state when there are zero hosts', () => {
    countRef.value = 0
    loadingRef.value = false
    const w = build()
    expect(w.find('[data-testid="empty"]').exists()).toBe(true)
    expect(w.find('[data-testid="table"]').exists()).toBe(false)
  })

  it('renders the table when hosts are present', () => {
    countRef.value = 1
    hostsRef.value = [makeHost({})]
    const w = build()
    expect(w.find('[data-testid="table"]').exists()).toBe(true)
    expect(w.find('[data-testid="empty"]').exists()).toBe(false)
  })

  it('register button opens the register modal', async () => {
    const w = build()
    expect((w.vm as unknown as { showRegister: boolean }).showRegister).toBe(false)
    await w.find('button').trigger('click') // header "Register host"
    expect((w.vm as unknown as { showRegister: boolean }).showRegister).toBe(true)
  })

  it('delete row action confirms then deletes and refetches', async () => {
    useConfirmMock.mockResolvedValueOnce(true)
    deleteHostMock.mockResolvedValue(undefined)
    const w = build()
    await (
      w.vm as unknown as {
        onAction: (p: { action: { label: string }; row: unknown }) => Promise<void>
      }
    ).onAction({ action: { label: 'Delete' }, row: makeHost({ id: 'h9', name: 'db-1' }) })
    expect(useConfirmMock).toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'destructive', ctaLabel: 'Delete' }),
    )
    expect(deleteHostMock).toHaveBeenCalledWith('h9')
    expect(fetchMock).toHaveBeenCalled()
  })

  it('rotate row action confirms then rotates and reveals the new credential', async () => {
    useConfirmMock.mockResolvedValueOnce(true)
    rotateCredentialMock.mockResolvedValue({ credential: 'ag_live_new', prefix: 'ag_live' })
    const w = build()
    await (
      w.vm as unknown as {
        onAction: (p: { action: { label: string }; row: unknown }) => Promise<void>
      }
    ).onAction({ action: { label: 'Rotate credential' }, row: makeHost({ id: 'h9' }) })
    expect(rotateCredentialMock).toHaveBeenCalledWith('h9')
    expect((w.vm as unknown as { rotated: unknown }).rotated).toMatchObject({
      credential: 'ag_live_new',
    })
  })
})
