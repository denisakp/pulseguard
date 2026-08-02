import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Incident, Resource } from '@/types'
import type { ResolvedResource } from '@/composables/useDashboardData'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards/x', name: 'DashboardDetail' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import IncidentsListWidget from './IncidentsListWidget.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  USkeleton: { template: '<div data-testid="skeleton" />' },
}

function makeResource(): Resource {
  return {
    id: 'r1',
    name: 'api',
    type: 'http',
    target: '',
    interval: 60,
    timeout: 30,
    status: 'up',
    is_active: true,
    failure_count: 0,
    confirmation_checks: 2,
    confirmation_interval: 30,
    created_at: '',
    updated_at: '',
  }
}

function makeIncident(over: Partial<Incident>): Incident {
  return {
    id: 'i1',
    resource_id: 'r1',
    reason: 'http_error',
    cause: 'Status code 500',
    started_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    resolved_at: null,
    created_at: '',
    updated_at: '',
    ...over,
  }
}

describe('IncidentsListWidget', () => {
  it('renders empty-state copy when no incidents', () => {
    const wrapper = mount(IncidentsListWidget, {
      global: { stubs },
      props: { incidents: [], resources: [], loading: false },
    })
    expect(wrapper.text()).toContain('No incidents in this window')
  })

  it('renders one row per incident (sorted desc by started_at), capped at limit', () => {
    const olderIso = new Date(Date.now() - 60 * 60_000).toISOString()
    const incidents: Incident[] = [
      makeIncident({ id: 'old', started_at: olderIso }),
      makeIncident({ id: 'new' }),
    ]
    const resources: ResolvedResource[] = [{ id: 'r1', resource: makeResource() }]
    const wrapper = mount(IncidentsListWidget, {
      global: { stubs },
      props: { incidents, resources, loading: false, limit: 1 },
    })
    const rows = wrapper.findAll('[data-testid^="incident-row-"]')
    expect(rows.length).toBe(1)
    expect(rows[0]!.attributes('data-testid')).toBe('incident-row-new')
  })

  it('row click navigates to the incident detail route', async () => {
    const wrapper = mount(IncidentsListWidget, {
      global: { stubs },
      props: {
        incidents: [makeIncident({})],
        resources: [{ id: 'r1', resource: makeResource() }],
        loading: false,
      },
    })
    await wrapper.find('[data-testid="incident-row-i1"]').trigger('click')
    expect(pushMock).toHaveBeenCalledWith({ name: 'IncidentDetail', params: { id: 'i1' } })
  })

  it('renders tombstone resource name + warning footer when resource is deleted (FR-024)', () => {
    const incident = makeIncident({ resource_id: 'ghost-id' })
    const resources: ResolvedResource[] = [{ id: 'ghost-id', resource: null }]
    const wrapper = mount(IncidentsListWidget, {
      global: { stubs },
      props: { incidents: [incident], resources, loading: false },
    })
    expect(wrapper.find('[data-testid="incident-resource-i1"]').text()).toContain('Resource removed')
    expect(wrapper.find('[data-testid="incidents-tombstone-count"]').exists()).toBe(true)
  })
})
