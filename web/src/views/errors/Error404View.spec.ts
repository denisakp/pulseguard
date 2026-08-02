import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/404', name: 'Error404' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import Error404View from './Error404View.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template: '<button @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'block', 'to', 'external'],
    emits: ['click'],
  },
}

describe('Error404View', () => {
  it('renders the 404 headline and brand', () => {
    const wrapper = mount(Error404View, { global: { stubs } })
    expect(wrapper.text()).toContain('404')
    expect(wrapper.text()).toContain('Page introuvable')
    expect(wrapper.text()).toContain('Ogoune')
  })

  it('navigates to Overview when the primary CTA is clicked', async () => {
    pushMock.mockClear()
    const wrapper = mount(Error404View, { global: { stubs } })
    const buttons = wrapper.findAll('button')
    await buttons[0]?.trigger('click')
    expect(pushMock).toHaveBeenCalledWith({ name: 'Overview' })
  })

  it('navigates to Resources when the secondary CTA is clicked', async () => {
    pushMock.mockClear()
    const wrapper = mount(Error404View, { global: { stubs } })
    const buttons = wrapper.findAll('button')
    await buttons[1]?.trigger('click')
    expect(pushMock).toHaveBeenCalledWith({ name: 'Resources' })
  })
})
