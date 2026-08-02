import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref, nextTick } from 'vue'

const pushMock = vi.fn().mockResolvedValue(undefined)
const fakeRouter = { push: pushMock } as never

const resourcesRef = ref<unknown[]>([])
const incidentsRef = ref<unknown[]>([])

vi.mock('@/services/resourceService', () => ({
  fetchResources: vi.fn().mockResolvedValue([]),
}))

vi.mock('@/stores/resourceStore', () => ({
  useResourceStore: () => ({
    resources: resourcesRef,
    loadResources: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/stores/incidentStore', () => ({
  useIncidentStore: () => ({
    incidents: incidentsRef,
  }),
}))

vi.mock('pinia', async () => {
  const actual = await vi.importActual<typeof import('pinia')>('pinia')
  return {
    ...actual,
    storeToRefs: (store: { resources?: unknown; incidents?: unknown }) => store,
  }
})

import UKeyboardShortcutsModal from './UKeyboardShortcutsModal.vue'
import {
  installKeyboardShortcuts,
  useKeyboardShortcuts,
  __resetKeyboardShortcutsForTests,
} from '@/composables/useKeyboardShortcuts'
import { __resetSearchPaletteForTests } from '@/composables/useSearchPalette'

const stubs = {
  UModal: {
    template:
      '<div v-if="open" data-testid="modal-stub" :data-open="open"><slot name="content" /><slot /></div>',
    props: ['open', 'ui'],
    emits: ['update:open'],
  },
  UIcon: { template: '<span />', props: ['name'] },
}

describe('UKeyboardShortcutsModal', () => {
  let uninstall: () => void = () => undefined
  let wrapper: ReturnType<typeof mount> | null = null

  beforeEach(() => {
    setActivePinia(createPinia())
    __resetSearchPaletteForTests()
    __resetKeyboardShortcutsForTests()
    pushMock.mockClear()
    uninstall = installKeyboardShortcuts(fakeRouter)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    uninstall()
    __resetKeyboardShortcutsForTests()
  })

  it('does not render content when modalOpen is false', () => {
    wrapper = mount(UKeyboardShortcutsModal, { global: { stubs } })
    expect(wrapper.find('[data-testid="modal-stub"]').exists()).toBe(false)
  })

  it('renders the three sections when open (via document.body teleport)', async () => {
    const ks = useKeyboardShortcuts()
    ks.open()
    wrapper = mount(UKeyboardShortcutsModal, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await nextTick()
    const body = document.body.innerHTML
    expect(body).toContain('NAVIGATION')
    expect(body).toContain('VIEW')
  })

  it('lists the documented navigation chord shortcuts', async () => {
    const ks = useKeyboardShortcuts()
    ks.open()
    wrapper = mount(UKeyboardShortcutsModal, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await nextTick()
    const body = document.body.innerHTML
    expect(body).toContain('Go to Overview')
    expect(body).toContain('Go to Resources')
    expect(body).toContain('Go to Incidents')
    expect(body).toContain('Go to Status pages')
    expect(body).toContain('then')
  })

  it('close button calls ks.close()', async () => {
    const ks = useKeyboardShortcuts()
    ks.open()
    wrapper = mount(UKeyboardShortcutsModal, { global: { stubs }, attachTo: document.body })
    await nextTick()
    await nextTick()
    const closeBtn = document.body.querySelector('button[aria-label="Close"]') as HTMLButtonElement | null
    expect(closeBtn).not.toBeNull()
    closeBtn?.click()
    await nextTick()
    expect(ks.modalOpen.value).toBe(false)
  })
})
