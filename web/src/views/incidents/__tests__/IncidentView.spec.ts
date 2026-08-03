import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn(), resolve: () => ({ href: '#' }) }),
  useRoute: () => ({
    query: {},
    params: { id: 'i1' },
    path: '/incidents/i1',
    name: 'IncidentDetail',
  }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

const incident = {
  id: 'i1',
  resource_id: 'r1',
  resource: { id: 'r1', name: 'api', type: 'http', status: 'down' },
  reason: 'r',
  cause: 'HTTP 500',
  started_at: new Date(Date.now() - 600_000).toISOString(),
  resolved_at: null,
  created_at: '',
  updated_at: '',
  event_steps: [
    {
      id: 'e1',
      incident_id: 'i1',
      step: 'detected',
      message: 'Down',
      created_at: new Date(Date.now() - 600_000).toISOString(),
      updated_at: '',
    },
  ],
}
const getIncidentMock = vi.fn().mockResolvedValue(incident)

vi.mock('@/stores/incidentStore', () => ({
  useIncidentStore: () => ({
    getIncidentById: getIncidentMock,
  }),
}))

vi.mock('@/components/incidents/IncidentHeader.vue', () => ({
  default: {
    name: 'IncidentHeader',
    template: '<div data-testid="header" />',
    props: ['incident'],
  },
}))
vi.mock('@/components/incidents/IncidentTimeline.vue', () => ({
  default: {
    name: 'IncidentTimeline',
    template: '<div data-testid="timeline" />',
    props: ['events'],
  },
}))
vi.mock('@/components/incidents/DiagnosticsPanel.vue', () => ({
  default: {
    name: 'DiagnosticsPanel',
    template: '<div data-testid="diagnostics" />',
    props: ['diagnostics'],
  },
}))
vi.mock('@/components/incidents/NotificationsPanel.vue', () => ({
  default: {
    name: 'NotificationsPanel',
    template: '<div data-testid="notifications" />',
    props: ['events'],
  },
}))

import IncidentView from '../IncidentView.vue'

const stubs = { UEmpty: { template: '<div />' } }

function build() {
  setActivePinia(createPinia())
  return mount(IncidentView, { global: { stubs } })
}

beforeEach(() => {
  pushMock.mockReset()
  getIncidentMock.mockClear()
})

describe('IncidentView', () => {
  it('mount calls getIncidentById(id) + composes 4 sub-components', async () => {
    const w = build()
    await flushPromises()
    expect(getIncidentMock).toHaveBeenCalledWith('i1')
    expect(w.findComponent({ name: 'IncidentHeader' }).exists()).toBe(true)
    expect(w.findComponent({ name: 'IncidentTimeline' }).exists()).toBe(true)
    expect(w.findComponent({ name: 'DiagnosticsPanel' }).exists()).toBe(true)
    expect(w.findComponent({ name: 'NotificationsPanel' }).exists()).toBe(true)
  })

  it('Back action navigates to /incidents', async () => {
    const w = build()
    await flushPromises()
    ;(w.vm as unknown as { onAction: (p: { kind: string }) => void }).onAction({
      kind: 'back',
    })
    expect(pushMock).toHaveBeenCalledWith('/incidents')
  })

  it('dark-mode artifact check: root carries bg-default (FR-020)', async () => {
    document.documentElement.classList.add('dark')
    const w = build()
    await flushPromises()
    expect(w.find('.bg-default').exists()).toBe(true)
    document.documentElement.classList.remove('dark')
  })
})
