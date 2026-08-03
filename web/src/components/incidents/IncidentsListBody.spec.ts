import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const fetchMock = vi.fn().mockResolvedValue(undefined)
const storeIncidents = [
  {
    id: 'i1',
    resource_id: 'r1',
    resource: { id: 'r1', name: 'api' },
    reason: 'r',
    cause: 'HTTP 500',
    started_at: new Date().toISOString(),
    resolved_at: null,
    created_at: '',
    updated_at: '',
  },
  {
    id: 'i2',
    resource_id: 'r2',
    resource: { id: 'r2', name: 'db' },
    reason: 'r',
    cause: 'TCP reset',
    started_at: new Date().toISOString(),
    resolved_at: new Date().toISOString(),
    created_at: '',
    updated_at: '',
  },
]

vi.mock('@/stores/incidentStore', () => ({
  useIncidentStore: () => ({
    get incidents() {
      return storeIncidents
    },
    fetchIncidents: fetchMock,
  }),
}))

import IncidentsListBody from './IncidentsListBody.vue'

const stubs = {
  UEmpty: { template: '<div data-testid="empty" />' },
  UIcon: { template: '<span />' },
}

beforeEach(() => {
  fetchMock.mockClear()
})

describe('IncidentsListBody', () => {
  it('with filter.resource_id only renders matching incidents', async () => {
    setActivePinia(createPinia())
    const w = mount(IncidentsListBody, {
      global: { stubs },
      props: { filter: { resource_id: 'r1' } },
    })
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledWith(
      expect.objectContaining({ monitor_id: 'r1', per_page: 50 }),
    )
    const vm = w.vm as unknown as { filtered: Array<{ id: string }> }
    expect(vm.filtered.length).toBe(1)
    expect(vm.filtered[0]?.id).toBe('i1')
  })

  it('row click emits with the right incident', async () => {
    setActivePinia(createPinia())
    const w = mount(IncidentsListBody, {
      global: { stubs },
      props: { filter: {} },
    })
    await flushPromises()
    await w.find('button').trigger('click')
    const emitted = w.emitted('row-click')
    expect(emitted?.[0]?.[0]).toMatchObject({ id: 'i1' })
  })

  it('filtered is empty when no incidents match the filter', async () => {
    setActivePinia(createPinia())
    const w = mount(IncidentsListBody, {
      global: { stubs },
      props: { filter: { resource_id: 'unknown' } },
    })
    await flushPromises()
    const vm = w.vm as unknown as { filtered: Array<{ id: string }> }
    expect(vm.filtered.length).toBe(0)
  })
})
