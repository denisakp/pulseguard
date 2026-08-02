import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HostStatusBadge from './HostStatusBadge.vue'

describe('HostStatusBadge', () => {
  it('renders Online when the agent is connected', () => {
    const w = mount(HostStatusBadge, { props: { online: true, lastSeenAt: '2026-07-30T10:00:00Z' } })
    expect(w.text()).toContain('Online')
    expect(w.find('.bg-success').exists()).toBe(true)
  })

  it('renders Offline with relative last-seen when disconnected', () => {
    const past = new Date(Date.now() - 5 * 60 * 1000).toISOString()
    const w = mount(HostStatusBadge, { props: { online: false, lastSeenAt: past } })
    expect(w.text()).toContain('Offline')
    expect(w.text()).toContain('minutes ago')
    expect(w.find('.bg-dimmed').exists()).toBe(true)
  })

  it('renders Never seen when there is no last-seen timestamp', () => {
    const w = mount(HostStatusBadge, { props: { online: false, lastSeenAt: null } })
    expect(w.text()).toContain('Never seen')
  })
})
