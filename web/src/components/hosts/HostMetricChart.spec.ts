import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import HostMetricChart from './HostMetricChart.vue'

function points(values: number[]) {
  return values.map((v, i) => ({ t: `2026-07-30T10:0${i}:00.000Z`, value: v }))
}

describe('HostMetricChart', () => {
  it('renders an SVG polyline + area path for non-empty points', () => {
    const w = mount(HostMetricChart, {
      props: { points: points([10, 40, 25]), label: 'CPU', unit: '%' },
    })
    expect(w.find('svg').exists()).toBe(true)
    expect(w.find('polyline').exists()).toBe(true)
    expect(w.find('path').exists()).toBe(true)
    const pts = w.find('polyline').attributes('points') ?? ''
    // 3 points → 3 "x,y" pairs
    expect(pts.trim().split(/\s+/)).toHaveLength(3)
    expect(w.find('[data-testid="metric-empty"]').exists()).toBe(false)
  })

  it('shows the empty state when there are no points', () => {
    const w = mount(HostMetricChart, { props: { points: [], label: 'CPU', unit: '%' } })
    expect(w.find('[data-testid="metric-empty"]').exists()).toBe(true)
    expect(w.text()).toContain('No data in range')
    expect(w.find('svg').exists()).toBe(false)
  })

  it('pins the y-axis to 0–100 for percent units', () => {
    // value 50 of 100 → middle of the 40-unit viewBox (y = 20)
    const w = mount(HostMetricChart, { props: { points: points([50]), label: 'RAM', unit: '%' } })
    const pts = w.find('polyline').attributes('points') ?? ''
    const [, y] = pts.trim().split(',')
    expect(Number(y)).toBeCloseTo(20, 5)
  })

  it('humanises the latest value for byte units', () => {
    const w = mount(HostMetricChart, {
      props: { points: points([2048, 4096]), label: 'Net In', unit: 'bytes' },
    })
    expect(w.find('[data-testid="metric-latest"]').text()).toContain('KB')
  })
})
