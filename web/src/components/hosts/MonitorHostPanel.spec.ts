import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const getHostMock = vi.fn()
const listMonitorsMock = vi.fn()

vi.mock('@/services/hostsService', () => ({
  getHost: (...a: unknown[]) => getHostMock(...a),
  listMonitors: (...a: unknown[]) => listMonitorsMock(...a),
}))

import MonitorHostPanel from './MonitorHostPanel.vue'

const HOST = {
  id: 'h1',
  name: 'web-01',
  os: 'linux',
  agentVersion: '1.0',
  online: true,
  lastSeenAt: '2026-01-01T00:00:00Z',
  lastCpuPct: 12.4,
  lastMemPct: 47,
  lastDiskPct: 80,
  lastNetIn: 0,
  lastNetOut: 0,
  lastDisks: [],
  createdAt: '',
  updatedAt: '',
}

const RESOURCES = [
  { id: 'm1', name: 'this monitor', type: 'http', status: 'up', lastCheckedAt: null, hostId: 'h1' },
  { id: 'm2', name: 'sibling A', type: 'http', status: 'up', lastCheckedAt: null, hostId: 'h1' },
  { id: 'm3', name: 'sibling B', type: 'tcp', status: 'down', lastCheckedAt: null, hostId: 'h1' },
  { id: 'm4', name: 'elsewhere', type: 'http', status: 'up', lastCheckedAt: null, hostId: 'h2' },
]

const stubs = {
  HostStatusBadge: { template: '<span data-testid="badge" />', props: ['online', 'lastSeenAt'] },
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template: '<a v-bind="$attrs"><slot /></a>',
    props: ['size', 'variant', 'color', 'icon'],
    inheritAttrs: false,
  },
  RouterLink: { template: '<a :href="to"><slot /></a>', props: ['to'] },
}

function mountPanel(monitorId?: string) {
  return mount(MonitorHostPanel, {
    global: { stubs },
    props: { hostId: 'h1', monitorId },
  })
}

describe('MonitorHostPanel', () => {
  beforeEach(() => {
    getHostMock.mockReset().mockResolvedValue(HOST)
    listMonitorsMock.mockReset().mockResolvedValue(RESOURCES)
  })

  afterEach(() => vi.clearAllMocks())

  it('renders host context when the host resolves', async () => {
    const w = mountPanel('m1')
    await flushPromises()
    expect(getHostMock).toHaveBeenCalledWith('h1')
    expect(w.find('[data-testid="host-name"]').text()).toContain('web-01')
    expect(w.find('[data-testid="badge"]').exists()).toBe(true)
    // CPU/RAM/Disk rounded percentages present
    expect(w.text()).toContain('12%')
    expect(w.text()).toContain('47%')
    expect(w.text()).toContain('80%')
    // "View host" affordance is present.
    const viewHost = w.find('[data-testid="view-host-link"]')
    expect(viewHost.exists()).toBe(true)
    expect(viewHost.text()).toContain('View host')
  })

  it('lists other services on the host, excluding the current monitor', async () => {
    const w = mountPanel('m1')
    await flushPromises()
    const others = (w.vm as unknown as { otherServices: { id: string }[] }).otherServices
    expect(others.map((r) => r.id)).toEqual(['m2', 'm3'])
    const list = w.find('[data-testid="other-services"]')
    expect(list.text()).toContain('sibling A')
    expect(list.text()).toContain('sibling B')
    expect(list.text()).not.toContain('elsewhere')
  })

  it('shows a graceful notice when the host fetch fails', async () => {
    getHostMock.mockRejectedValueOnce(new Error('boom'))
    const w = mountPanel('m1')
    await flushPromises()
    expect(w.find('[data-testid="host-not-found"]').exists()).toBe(true)
    expect(w.find('[data-testid="host-name"]').exists()).toBe(false)
  })

  it('degrades other-services list to empty when resources fetch fails', async () => {
    listMonitorsMock.mockRejectedValueOnce(new Error('boom'))
    const w = mountPanel('m1')
    await flushPromises()
    // Host still renders even though sibling fetch failed.
    expect(w.find('[data-testid="host-name"]').exists()).toBe(true)
    expect((w.vm as unknown as { otherServices: unknown[] }).otherServices).toHaveLength(0)
  })
})
