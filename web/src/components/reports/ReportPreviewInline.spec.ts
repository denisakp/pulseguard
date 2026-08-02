import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ReportHistoryEntry } from '@/types'
import ReportPreviewInline from './ReportPreviewInline.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
}

function makeEntry(): ReportHistoryEntry {
  return {
    id: 'h1',
    period: 'May 2026',
    sentAt: '2026-06-01T08:00:00Z',
    status: 'delivered',
    uptimePct: 99.94,
    incidentCount: 3,
    downtimeSeconds: 1620,
    recipientEmail: 'admin@example.com',
    resourceBreakdown: [
      { name: 'api.example.com', uptimePct: 99.97, incidents: 1 },
      { name: 'web.example.com', uptimePct: 100, incidents: 0 },
    ],
  }
}

describe('ReportPreviewInline', () => {
  it('renders the brand + period header', () => {
    const wrapper = mount(ReportPreviewInline, { global: { stubs }, props: { entry: makeEntry() } })
    expect(wrapper.text()).toContain('Ogoune')
    expect(wrapper.text()).toContain('May 2026')
  })

  it('renders the three big stats (uptime / incidents / downtime)', () => {
    const wrapper = mount(ReportPreviewInline, { global: { stubs }, props: { entry: makeEntry() } })
    expect(wrapper.text()).toContain('99.94%')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('27 min')
  })

  it('renders the per-resource breakdown table', () => {
    const wrapper = mount(ReportPreviewInline, { global: { stubs }, props: { entry: makeEntry() } })
    const tbl = wrapper.find('[data-testid="report-preview-breakdown"]')
    expect(tbl.exists()).toBe(true)
    expect(tbl.text()).toContain('api.example.com')
    expect(tbl.text()).toContain('99.97%')
    expect(tbl.text()).toContain('web.example.com')
    expect(tbl.text()).toContain('100.00%')
  })

  it('renders the recipient in the footer', () => {
    const wrapper = mount(ReportPreviewInline, { global: { stubs }, props: { entry: makeEntry() } })
    expect(wrapper.text()).toContain('admin@example.com')
  })
})
