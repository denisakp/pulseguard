import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import IncidentHeader from './IncidentHeader.vue'

const stubs = {
  UButton: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
  UIcon: { template: '<span />' },
}

function mkIncident(opts: { resolved?: boolean } = {}) {
  return {
    id: 'incident-01HTMVABCDEF',
    resource_id: 'r1',
    resource: { id: 'r1', name: 'api.acme.com', type: 'http', status: 'down' },
    reason: 'HTTP 500',
    cause: 'Database connection refused',
    started_at: new Date(Date.now() - 600_000).toISOString(),
    resolved_at: opts.resolved ? new Date().toISOString() : null,
    created_at: new Date(Date.now() - 600_000).toISOString(),
    updated_at: new Date().toISOString(),
  }
}

describe('IncidentHeader', () => {
  it('renders Active state without a Resolve button when not resolved', () => {
    const w = mount(IncidentHeader, {
      global: { stubs },
      props: { incident: mkIncident({ resolved: false }) as unknown as import('@/types').Incident },
    })
    expect(w.text()).toContain('Active')
    const buttonTexts = w.findAll('button').map((b) => b.text())
    expect(buttonTexts).not.toContain('Resolve')
    expect(buttonTexts).toContain('Back')
  })

  it('renders Resolved state without Resolve button when resolved', () => {
    const w = mount(IncidentHeader, {
      global: { stubs },
      props: { incident: mkIncident({ resolved: true }) as unknown as import('@/types').Incident },
    })
    expect(w.text()).toContain('Resolved')
    // Only "Back" button when resolved (Resolve action gone)
    const buttonTexts = w.findAll('button').map((b) => b.text())
    expect(buttonTexts).not.toContain('Resolve')
    expect(buttonTexts).toContain('Back')
  })

  it('renders INC ID + resource name + cause', () => {
    const w = mount(IncidentHeader, {
      global: { stubs },
      props: { incident: mkIncident() as unknown as import('@/types').Incident },
    })
    expect(w.text()).toContain('INC-')
    expect(w.text()).toContain('api.acme.com')
    expect(w.text()).toContain('Database connection refused')
  })

  it('emits action back when Back clicked', async () => {
    const w = mount(IncidentHeader, {
      global: { stubs },
      props: { incident: mkIncident({ resolved: false }) as unknown as import('@/types').Incident },
    })
    const buttons = w.findAll('button')
    await buttons[0]?.trigger('click') // Back is the only action button
    expect(w.emitted('action')?.[0]?.[0]).toEqual({ kind: 'back' })
  })
})
