import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/maintenance-mode', name: 'MaintenanceMode' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template: '<button @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'block', 'to', 'external'],
    emits: ['click'],
  },
}

describe('MaintenanceModeView', () => {
  beforeEach(() => {
    document.documentElement.classList.remove('dark')
    vi.resetModules()
  })

  afterEach(() => {
    document.documentElement.classList.remove('dark')
    vi.unstubAllEnvs()
  })

  it('forces dark mode on the root element', async () => {
    const { default: MaintenanceModeView } = await import('./MaintenanceModeView.vue')
    mount(MaintenanceModeView, { global: { stubs } })
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('renders the ETA pill copy when VITE_MAINTENANCE_ETA is set', async () => {
    vi.stubEnv('VITE_MAINTENANCE_ETA', 'est. 30 min')
    const { default: MaintenanceModeView } = await import('./MaintenanceModeView.vue')
    const wrapper = mount(MaintenanceModeView, { global: { stubs } })
    expect(wrapper.text()).toContain('Scheduled maintenance · est. 30 min')
  })

  it('renders the MAINTENANCE label and default message', async () => {
    const { default: MaintenanceModeView } = await import('./MaintenanceModeView.vue')
    const wrapper = mount(MaintenanceModeView, { global: { stubs } })
    expect(wrapper.text()).toContain('MAINTENANCE')
    expect(wrapper.text()).toContain("On bricole sous le capot")
  })

  it('exposes the status page link on the primary CTA', async () => {
    const { default: MaintenanceModeView } = await import('./MaintenanceModeView.vue')
    const wrapper = mount(MaintenanceModeView, { global: { stubs } })
    expect(wrapper.html()).toContain('/status.html')
  })
})
