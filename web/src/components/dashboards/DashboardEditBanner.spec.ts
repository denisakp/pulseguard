import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import DashboardEditBanner from './DashboardEditBanner.vue'

const stubs = {
  UIcon: { template: '<span />', props: ['name'] },
  UButton: {
    template:
      '<button :disabled="disabled || loading" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
    props: ['color', 'variant', 'size', 'disabled', 'loading'],
    emits: ['click'],
    inheritAttrs: false,
  },
}

describe('DashboardEditBanner', () => {
  it('renders banner copy + Save/Cancel buttons', () => {
    const wrapper = mount(DashboardEditBanner, { global: { stubs } })
    expect(wrapper.find('[data-testid="dashboard-edit-banner"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Edit Mode')
    expect(wrapper.find('[data-testid="edit-cancel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-save"]').exists()).toBe(true)
  })

  it('emits save event when Save is clicked', async () => {
    const wrapper = mount(DashboardEditBanner, { global: { stubs } })
    await wrapper.find('[data-testid="edit-save"]').trigger('click')
    expect(wrapper.emitted('save')).toBeTruthy()
  })

  it('emits cancel event when Cancel is clicked', async () => {
    const wrapper = mount(DashboardEditBanner, { global: { stubs } })
    await wrapper.find('[data-testid="edit-cancel"]').trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('Save is disabled when saving=true', async () => {
    const wrapper = mount(DashboardEditBanner, {
      global: { stubs },
      props: { saving: true },
    })
    const save = wrapper.find('[data-testid="edit-save"]')
    expect(save.attributes('disabled')).toBeDefined()
  })
})
