import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

const pushMock = vi.fn()
const createMock = vi.fn(async () => ({ id: 'new-dash' }))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/dashboards', name: 'Dashboards' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({ userId: 'user-default', email: 'me@example.com' }),
}))

vi.mock('@/composables/useDashboards', () => ({
  useDashboards: () => ({ create: createMock }),
}))

const useConfirmMock = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: (...args: unknown[]) => useConfirmMock(...args),
}))

const toastAddMock = vi.fn()
;(globalThis as Record<string, unknown>).useToast = () => ({ add: toastAddMock })

vi.mock('@/widgets/widgetCatalog', () => ({
  listWidgets: () => [
    {
      id: 'uptime-stat',
      name: 'Uptime',
      icon: 'i-lucide-trending-up',
      archetype: 'stat',
      defaultConfig: {},
      component: () => Promise.resolve({ default: {} }),
    },
    {
      id: 'incidents-list',
      name: 'Recent incidents',
      icon: 'i-lucide-circle-alert',
      archetype: 'list',
      defaultConfig: {},
      component: () => Promise.resolve({ default: {} }),
    },
  ],
}))

import DashboardWizardModal from './DashboardWizardModal.vue'

const stubs = {
  UModal: {
    template:
      '<div v-if="open" data-testid="modal-stub"><slot name="content" /><slot /></div>',
    props: ['open', 'ui'],
    emits: ['update:open'],
  },
  UButton: {
    template: '<button :disabled="disabled || loading" v-bind="$attrs"><slot /></button>',
    props: ['color', 'variant', 'size', 'icon', 'disabled', 'loading'],
    inheritAttrs: false,
    emits: ['click'],
  },
  UIcon: { template: '<span />', props: ['name'] },
  UTooltip: { template: '<div><slot /></div>', props: ['text'] },
  UEditionBadge: {
    template: '<span data-testid="edition-badge" :data-edition="edition">{{ edition }}</span>',
    props: ['edition', 'size'],
  },
  DashboardScopeResolver: {
    template:
      '<button data-testid="scope-resolver-trigger" type="button" @click="emitMatch">Trigger match</button>',
    props: ['modelValue'],
    emits: ['update:modelValue', 'update:matchCount'],
    methods: {
      emitMatch(this: { $emit: (e: string, v: unknown) => void }) {
        this.$emit('update:modelValue', { mode: 'tag', payload: { tagIds: ['t1'] } })
        this.$emit('update:matchCount', 3)
      },
    },
  },
}

function $(sel: string): HTMLElement | null {
  return document.body.querySelector(sel) as HTMLElement | null
}

async function settle() {
  await nextTick()
  await nextTick()
  await new Promise((r) => setTimeout(r, 0))
}

describe('DashboardWizardModal', () => {
  let wrapper: ReturnType<typeof mount> | null = null

  beforeEach(() => {
    document.body.innerHTML = ''
    pushMock.mockClear()
    createMock.mockClear()
    useConfirmMock.mockReset().mockResolvedValue(true)
    toastAddMock.mockClear()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  it('renders Step 1 with Continue disabled when no name + no matches', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    expect($('[data-testid="wizard-step-1"]')).not.toBeNull()
    const cont = $('[data-testid="wizard-continue"]') as HTMLButtonElement | null
    expect(cont).not.toBeNull()
    expect(cont!.disabled).toBe(true)
  })

  it('Continue advances to Step 2 once name + match count are valid', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const input = $('[data-testid="wizard-name-input"]') as HTMLInputElement
    expect(input).not.toBeNull()
    input.value = 'Test dash'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    ;($('[data-testid="scope-resolver-trigger"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    expect($('[data-testid="wizard-step-2"]')).not.toBeNull()
  })

  it('Step 2 widget counter updates on toggle', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const input = $('[data-testid="wizard-name-input"]') as HTMLInputElement
    input.value = 'x'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    ;($('[data-testid="scope-resolver-trigger"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    expect(($('[data-testid="wizard-continue"]') as HTMLButtonElement).disabled).toBe(true)
    ;($('[data-testid="wizard-widget-uptime-stat"]') as HTMLButtonElement).click()
    await settle()
    expect($('[data-testid="wizard-widget-counter"]')!.textContent).toContain('1 of 2')
    expect(($('[data-testid="wizard-continue"]') as HTMLButtonElement).disabled).toBe(false)
  })

  it('Step 3 disables Team and Public visibility cards (EE)', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const input = $('[data-testid="wizard-name-input"]') as HTMLInputElement
    input.value = 'x'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    ;($('[data-testid="scope-resolver-trigger"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-widget-uptime-stat"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    expect(($('[data-testid="wizard-visibility-team"]') as HTMLButtonElement).disabled).toBe(true)
    expect(($('[data-testid="wizard-visibility-public"]') as HTMLButtonElement).disabled).toBe(true)
  })

  it('clicking close after a modification triggers the discard confirm', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const input = $('[data-testid="wizard-name-input"]') as HTMLInputElement
    input.value = 'dirty'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await settle()
    ;($('[data-testid="wizard-close"]') as HTMLButtonElement).click()
    await settle()
    expect(useConfirmMock).toHaveBeenCalled()
  })

  it('submit creates the dashboard, fires success toast, navigates to detail', async () => {
    wrapper = mount(DashboardWizardModal, {
      global: { stubs },
      props: { open: true },
      attachTo: document.body,
    })
    await settle()
    const input = $('[data-testid="wizard-name-input"]') as HTMLInputElement
    input.value = 'Final'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    ;($('[data-testid="scope-resolver-trigger"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-widget-uptime-stat"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-continue"]') as HTMLButtonElement).click()
    await settle()
    ;($('[data-testid="wizard-submit"]') as HTMLButtonElement).click()
    await new Promise((r) => setTimeout(r, 20))
    expect(createMock).toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith({ name: 'DashboardDetail', params: { id: 'new-dash' } })
  })
})
