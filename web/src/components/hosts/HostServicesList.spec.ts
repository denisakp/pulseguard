import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import HostServicesList, { type HostLinkedMonitor } from './HostServicesList.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a class="router-link" :data-to="JSON.stringify(to)"><slot /></a>',
}

function build(monitors: HostLinkedMonitor[]) {
  return mount(HostServicesList, {
    props: { monitors },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

describe('HostServicesList', () => {
  it('renders a row per linked monitor with a link to its detail route', () => {
    const w = build([
      { id: 'r1', name: 'api', type: 'http', status: 'up', lastCheckedAt: '2026-07-30T10:00:00Z' },
      { id: 'r2', name: 'db', type: 'tcp', status: 'down', lastCheckedAt: null },
    ])
    const rows = w.findAll('[data-testid="service-row"]')
    expect(rows).toHaveLength(2)
    expect(w.text()).toContain('api')
    expect(w.text()).toContain('db')

    const to = JSON.parse(rows[0]!.attributes('data-to') ?? '{}')
    expect(to).toEqual({ name: 'ResourceDetail', params: { id: 'r1' } })
  })

  it('renders "never" when a monitor has no last-check timestamp', () => {
    const w = build([{ id: 'r2', name: 'db', type: 'tcp', status: 'down', lastCheckedAt: null }])
    expect(w.text()).toContain('never')
  })

  it('shows the empty state when there are no monitors', () => {
    const w = build([])
    expect(w.find('[data-testid="services-empty"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="service-row"]')).toHaveLength(0)
  })
})
