import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { NotFoundError } from '@/core/errors'
import type { Host } from '@/types'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'h1' } }),
}))

const getHostMock = vi.fn()
const getHostMetricsMock = vi.fn()
const listMonitorsMock = vi.fn()
vi.mock('@/services/hostsService', () => ({
  getHost: (...a: unknown[]) => getHostMock(...a),
  getHostMetrics: (...a: unknown[]) => getHostMetricsMock(...a),
  listMonitors: (...a: unknown[]) => listMonitorsMock(...a),
}))

import HostDetailView from './HostDetailView.vue'

function makeHost(over: Partial<Host> = {}): Host {
  return {
    id: 'h1',
    name: 'web-1',
    os: 'Ubuntu 24.04',
    agentVersion: '1.2.0',
    online: true,
    lastSeenAt: '2026-07-30T11:59:00Z',
    lastCpuPct: 10,
    lastMemPct: 40,
    lastDiskPct: 55,
    lastNetIn: 1000,
    lastNetOut: 500,
    lastDisks: [],
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-30T12:00:00Z',
    ...over,
  }
}

const stubs = {
  UButton: {
    props: ['dataRange'],
    template: '<button v-bind="$attrs"><slot /></button>',
    inheritAttrs: false,
  },
  UAlert: { template: '<div />' },
  UIcon: { template: '<span />', props: ['name'] },
  UEmpty: { props: ['title'], template: '<div data-testid="uempty">{{ title }}</div>' },
  HostMetricChart: {
    props: ['points', 'label', 'unit'],
    template: '<div class="host-metric-chart" :data-label="label" :data-count="points.length" />',
  },
  HostServicesList: {
    props: ['monitors'],
    template: '<div data-testid="services" :data-count="monitors.length" />',
  },
}

function build() {
  return mount(HostDetailView, { global: { stubs } })
}

beforeEach(() => {
  getHostMock.mockReset().mockResolvedValue(makeHost())
  getHostMetricsMock.mockReset().mockResolvedValue([])
  listMonitorsMock.mockReset().mockResolvedValue([])
  vi.useFakeTimers()
})

afterEach(() => {
  vi.clearAllTimers()
  vi.useRealTimers()
})

describe('HostDetailView', () => {
  it('renders the header fields (name / OS / agent / last-seen)', async () => {
    const w = build()
    await flushPromises()
    expect(w.text()).toContain('web-1')
    expect(w.find('[data-testid="host-os"]').text()).toContain('Ubuntu 24.04')
    expect(w.find('[data-testid="host-agent"]').text()).toContain('1.2.0')
    expect(w.find('[data-testid="host-last-seen"]').exists()).toBe(true)
  })

  it('refetches metrics when the range changes', async () => {
    const w = build()
    await flushPromises()
    expect(getHostMetricsMock).toHaveBeenCalledTimes(1)

    ;(w.vm as unknown as { setRange: (r: string) => void }).setRange('7d')
    await flushPromises()
    expect(getHostMetricsMock).toHaveBeenCalledTimes(2)
    const [, from, to] = getHostMetricsMock.mock.calls[1] as [string, Date, Date]
    expect(to.getTime() - from.getTime()).toBe(7 * 24 * 60 * 60 * 1000)
  })

  it('lists only the monitors linked to this host', async () => {
    listMonitorsMock.mockResolvedValue([
      { id: 'r1', name: 'api', type: 'http', status: 'up', hostId: 'h1', lastCheckedAt: null },
      { id: 'r2', name: 'other', type: 'tcp', status: 'up', hostId: 'other-host', lastCheckedAt: null },
      { id: 'r3', name: 'db', type: 'tcp', status: 'down', hostId: 'h1', lastCheckedAt: null },
    ])
    const w = build()
    await flushPromises()
    const monitors = (w.vm as unknown as { monitors: unknown[] }).monitors
    expect(monitors.map((m) => (m as { id: string }).id)).toEqual(['r1', 'r3'])
    expect(w.find('[data-testid="services"]').attributes('data-count')).toBe('2')
  })

  it('shows the disk empty state when the range has no metrics', async () => {
    const w = build()
    await flushPromises()
    expect(w.find('[data-testid="disk-charts"]').text()).toContain('No disk metrics in range')
  })

  it('renders the not-found state when the host is missing', async () => {
    getHostMock.mockRejectedValue(new NotFoundError('nope'))
    const w = build()
    await flushPromises()
    expect(w.find('[data-testid="host-not-found"]').exists()).toBe(true)
  })
})
