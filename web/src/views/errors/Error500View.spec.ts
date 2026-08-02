import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

const replaceMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
  useRoute: () => ({ query: {}, params: {}, path: '/error-500', name: 'Error500' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import Error500View from './Error500View.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template: '<button @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'block', 'to', 'external'],
    emits: ['click'],
  },
}

describe('Error500View', () => {
  beforeEach(() => {
    replaceMock.mockClear()
    window.history.replaceState(
      {
        incidentId: 'INC-2026-ABC234',
        occurredAt: new Date().toISOString(),
        originalMessage: 'boom',
      },
      '',
    )
  })

  it('renders the synthetic incident pill in the canonical format', () => {
    const wrapper = mount(Error500View, { global: { stubs } })
    expect(wrapper.text()).toMatch(/INC-\d{4}-[A-Z2-7]{6}/)
    expect(wrapper.text()).toContain('500')
  })

  it('replaces history with Overview when Try again is clicked', async () => {
    const wrapper = mount(Error500View, { global: { stubs } })
    const buttons = wrapper.findAll('button')
    await buttons[0]?.trigger('click')
    expect(replaceMock).toHaveBeenCalledWith({ name: 'Overview' })
  })

  it('exposes the status page link on the secondary CTA', () => {
    const wrapper = mount(Error500View, { global: { stubs } })
    expect(wrapper.html()).toContain('/status.html')
  })

  it('falls back to a freshly generated incident when no state is present', () => {
    window.history.replaceState(null, '')
    const wrapper = mount(Error500View, { global: { stubs } })
    expect(wrapper.text()).toMatch(/INC-\d{4}-[A-Z2-7]{6}/)
  })
})
