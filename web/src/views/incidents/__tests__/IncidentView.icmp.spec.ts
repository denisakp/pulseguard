import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import IncidentView from '@/views/incidents/IncidentView.vue'
import type { Incident } from '@/types'

// IncidentView dependencies
vi.mock('vue-router', () => ({
  useRouter: () => ({ back: vi.fn() }),
  useRoute: () => ({ params: { id: 'test-incident-id' } }),
}))

vi.mock('@nuxt/ui/composables/useToast', () => ({
  useToast: () => ({ add: vi.fn() }),
}))

const getIncidentByIdMock = vi.fn()

vi.mock('@/stores/incidentStore', () => ({
  useIncidentStore: () => ({
    getIncidentById: getIncidentByIdMock,
    $id: 'incident',
  }),
}))

const makeIncident = (overrides: Partial<Incident> = {}): Incident => ({
  id: 'inc-001',
  resource_id: 'res-001',
  reason: 'timeout',
  cause: 'timeout',
  started_at: '2026-01-01T10:00:00Z',
  created_at: '2026-01-01T10:00:00Z',
  updated_at: '2026-01-01T10:00:00Z',
  ...overrides,
})

describe('IncidentView ICMP diagnostic rendering', () => {
  it('renders ICMP network diagnostic card when icmp_available is present', async () => {
    getIncidentByIdMock.mockResolvedValue(
      makeIncident({
        diagnostics: {
          id: 'diag-001',
          incident_id: 'inc-001',
          request_method: 'ICMP',
          request_url: '192.168.1.1',
          request_timeout: 2000,
          http_status_code: 0,
          response_size: 0,
          failure_type: 'host_unreachable',
          error_message: 'ping failed',
          error_summary: 'Host unreachable',
          total_duration: 2000,
          dns_duration: 0,
          tls_duration: 0,
          first_byte_duration: 0,
          body_truncated: false,
          body_encoded: false,
          icmp_available: true,
          icmp_reachable: false,
          icmp_rtt_ms: null,
          root_cause_hint: 'host_unreachable',
        },
      }),
    )

    const wrapper = mount(IncidentView)
    // Wait for mount + async loadIncident
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))

    const html = wrapper.html()
    // Should show the network diagnostics section
    expect(html.toLowerCase()).toMatch(/network diagnostic|icmp|ping/i)
  })

  it('renders RTT value when icmp_rtt_ms is provided', async () => {
    getIncidentByIdMock.mockResolvedValue(
      makeIncident({
        diagnostics: {
          id: 'diag-002',
          incident_id: 'inc-001',
          request_method: 'ICMP',
          request_url: '8.8.8.8',
          request_timeout: 2000,
          http_status_code: 0,
          response_size: 0,
          failure_type: 'service_down',
          error_message: '',
          error_summary: 'Service down',
          total_duration: 42,
          dns_duration: 0,
          tls_duration: 0,
          first_byte_duration: 0,
          body_truncated: false,
          body_encoded: false,
          icmp_available: true,
          icmp_reachable: true,
          icmp_rtt_ms: 42,
          root_cause_hint: 'service_down',
        },
      }),
    )

    const wrapper = mount(IncidentView)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))

    const html = wrapper.html()
    expect(html).toContain('42')
    expect(html).toMatch(/ms|rtt/i)
  })

  it('hides ICMP network diagnostic section when icmp_available is null (backward compat)', async () => {
    getIncidentByIdMock.mockResolvedValue(
      makeIncident({
        diagnostics: {
          id: 'diag-003',
          incident_id: 'inc-001',
          request_method: 'GET',
          request_url: 'https://example.com',
          request_timeout: 10000,
          http_status_code: 503,
          response_size: 0,
          failure_type: 'bad_status',
          error_message: 'service unavailable',
          error_summary: 'HTTP 503',
          total_duration: 500,
          dns_duration: 10,
          tls_duration: 20,
          first_byte_duration: 30,
          body_truncated: false,
          body_encoded: false,
          // No ICMP fields — legacy incident
        },
      }),
    )

    const wrapper = mount(IncidentView)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))

    const html = wrapper.html()
    // Should NOT show the ICMP network diagnostics section
    expect(html.toLowerCase()).not.toMatch(/network diagnostic/i)
  })

  it('shows root_cause_hint badge for host_unreachable', async () => {
    getIncidentByIdMock.mockResolvedValue(
      makeIncident({
        diagnostics: {
          id: 'diag-004',
          incident_id: 'inc-001',
          request_method: 'ICMP',
          request_url: '10.0.0.1',
          request_timeout: 2000,
          http_status_code: 0,
          response_size: 0,
          failure_type: 'host_unreachable',
          error_message: '',
          error_summary: 'Host unreachable',
          total_duration: 2000,
          dns_duration: 0,
          tls_duration: 0,
          first_byte_duration: 0,
          body_truncated: false,
          body_encoded: false,
          icmp_available: true,
          icmp_reachable: false,
          icmp_rtt_ms: null,
          root_cause_hint: 'host_unreachable',
        },
      }),
    )

    const wrapper = mount(IncidentView)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))

    const html = wrapper.html()
    // Should show the hint somewhere
    expect(html.toLowerCase()).toMatch(/host.unreachable|host_unreachable/i)
  })

  it('hides ICMP section when incident has no diagnostics at all', async () => {
    getIncidentByIdMock.mockResolvedValue(makeIncident())

    const wrapper = mount(IncidentView)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))

    const html = wrapper.html()
    expect(html.toLowerCase()).not.toMatch(/network diagnostic/i)
  })
})
