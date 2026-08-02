import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/overview', name: 'Overview' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import { ref } from 'vue'

const resourcesRef = ref<unknown[]>([
  { id: 'r1', name: 'API gateway', target: 'https://api.example.com' },
])
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

import USearchPalette from './USearchPalette.vue'
import { useSearchPalette, __resetSearchPaletteForTests } from '@/composables/useSearchPalette'

const stubs = {
  UModal: {
    template: '<div data-testid="modal-stub" :data-open="open"><slot name="content" /><slot /></div>',
    props: ['open', 'ui'],
    emits: ['update:open'],
  },
  UIcon: { template: '<span />', props: ['name'] },
}

describe('USearchPalette', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    __resetSearchPaletteForTests()
    pushMock.mockClear()
  })


  let wrapper: ReturnType<typeof mount> | null = null

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  it('closes on Escape when open (local listener)', async () => {
    wrapper = mount(USearchPalette, { global: { stubs }, attachTo: document.body })
    await nextTick()
    const palette = useSearchPalette()
    palette.setOpen(true)
    expect(palette.open.value).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(palette.open.value).toBe(false)
  })

  it('exposes corpus grouped into resource / incident / page sections', async () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    wrapper = mount(USearchPalette, { global: { stubs }, attachTo: document.body })
    await nextTick()
    const grouped = palette.groupedResults.value
    expect(grouped.resource.length).toBeGreaterThan(0)
    expect(grouped.page.length).toBeGreaterThan(0)
  })

  it('navigates with arrow keys and activates with Enter', async () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    wrapper = mount(USearchPalette, { global: { stubs }, attachTo: document.body })
    await nextTick()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }))
    expect(palette.highlightIndex.value).toBe(1)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }))
    expect(pushMock).toHaveBeenCalled()
  })
})
