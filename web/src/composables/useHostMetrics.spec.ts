import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import type { HostMetricRange, HostMetricSample } from '@/types'

const getHostMetricsMock = vi.fn()
vi.mock('@/services/hostsService', () => ({
  getHostMetrics: (...args: unknown[]) => getHostMetricsMock(...args),
}))

import { useHostMetrics, rangeWindow } from './useHostMetrics'

function sample(over: Partial<HostMetricSample> = {}): HostMetricSample {
  return {
    sampledAt: '2026-07-30T10:00:00.000Z',
    cpuPct: 12,
    memPct: 40,
    netIn: 1000,
    netOut: 500,
    disks: [{ mount: '/', usedPct: 55 }],
    ...over,
  }
}

beforeEach(() => {
  getHostMetricsMock.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('rangeWindow', () => {
  it('resolves the correct span for each preset', () => {
    const now = new Date('2026-07-30T12:00:00.000Z')
    const cases: Array<[HostMetricRange, number]> = [
      ['1h', 60 * 60 * 1000],
      ['6h', 6 * 60 * 60 * 1000],
      ['24h', 24 * 60 * 60 * 1000],
      ['7d', 7 * 24 * 60 * 60 * 1000],
    ]
    for (const [range, ms] of cases) {
      const { from, to } = rangeWindow(range, now)
      expect(to.getTime()).toBe(now.getTime())
      expect(to.getTime() - from.getTime()).toBe(ms)
    }
  })
})

describe('useHostMetrics', () => {
  it('fetches with a from/to window matching the range and shapes points + disks', async () => {
    getHostMetricsMock.mockResolvedValue([
      sample({ sampledAt: '2026-07-30T10:00:00.000Z', disks: [{ mount: '/', usedPct: 55 }] }),
      sample({ sampledAt: '2026-07-30T10:01:00.000Z', disks: [{ mount: '/data', usedPct: 80 }] }),
    ])
    const range = ref<HostMetricRange>('6h')
    const m = useHostMetrics('h1', range)
    await flushPromises()

    expect(getHostMetricsMock).toHaveBeenCalledTimes(1)
    const [id, from, to] = getHostMetricsMock.mock.calls[0] as [string, Date, Date]
    expect(id).toBe('h1')
    expect(to.getTime() - from.getTime()).toBe(6 * 60 * 60 * 1000)

    expect(m.points.value).toHaveLength(2)
    expect(m.points.value[0]).toMatchObject({ cpuPct: 12, memPct: 40, netIn: 1000, netOut: 500 })
    expect(Object.keys(m.disksByMount.value).sort()).toEqual(['/', '/data'])
    expect(m.isEmpty.value).toBe(false)
    expect(m.error.value).toBeNull()
  })

  it('flags isEmpty (not error) on an empty result', async () => {
    getHostMetricsMock.mockResolvedValue([])
    const m = useHostMetrics('h1', ref<HostMetricRange>('1h'))
    await flushPromises()

    expect(m.isEmpty.value).toBe(true)
    expect(m.error.value).toBeNull()
    expect(m.points.value).toEqual([])
  })

  it('refetches when the range changes', async () => {
    getHostMetricsMock.mockResolvedValue([])
    const range = ref<HostMetricRange>('1h')
    useHostMetrics('h1', range)
    await flushPromises()
    expect(getHostMetricsMock).toHaveBeenCalledTimes(1)

    range.value = '24h'
    await flushPromises()
    expect(getHostMetricsMock).toHaveBeenCalledTimes(2)
    const [, from, to] = getHostMetricsMock.mock.calls[1] as [string, Date, Date]
    expect(to.getTime() - from.getTime()).toBe(24 * 60 * 60 * 1000)
  })

  it('captures an error message without throwing when the fetch rejects', async () => {
    getHostMetricsMock.mockRejectedValue(new Error('boom'))
    const m = useHostMetrics('h1', ref<HostMetricRange>('1h'))
    await flushPromises()

    expect(m.error.value).toBe('boom')
    expect(m.isEmpty.value).toBe(false)
  })
})
