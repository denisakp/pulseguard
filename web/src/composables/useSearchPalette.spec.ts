import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

const loadResourcesMock = vi.fn().mockResolvedValue(undefined)
const resourcesRef = ref<unknown[]>([])
const incidentsRef = ref<unknown[]>([])

vi.mock('@/services/resourceService', () => ({
  fetchResources: vi.fn().mockResolvedValue([]),
}))

vi.mock('@/stores/resourceStore', () => ({
  useResourceStore: () => ({
    resources: resourcesRef,
    loadResources: loadResourcesMock,
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

import { useSearchPalette, __resetSearchPaletteForTests } from './useSearchPalette'

describe('useSearchPalette', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    __resetSearchPaletteForTests()
    loadResourcesMock.mockClear()
    resourcesRef.value = []
    incidentsRef.value = []
  })

  it('opens and closes via setOpen / toggle, resetting query on close', () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'foo'
    expect(palette.open.value).toBe(true)
    palette.setOpen(false)
    expect(palette.open.value).toBe(false)
    expect(palette.query.value).toBe('')
    palette.toggle()
    expect(palette.open.value).toBe(true)
  })

  it('seeds static pages in the corpus so the palette is useful before stores hydrate', () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    expect(palette.results.value.some((r) => r.label === 'Overview' && r.category === 'page')).toBe(
      true,
    )
  })

  it('filters by query using fuzzy matching across the corpus', () => {
    resourcesRef.value = [
      { id: 'r1', name: 'API gateway', target: 'https://api.example.com' } as never,
      { id: 'r2', name: 'Postgres primary', target: 'postgres://db' } as never,
    ]
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'gateway'
    const labels = palette.results.value.map((r) => r.label)
    expect(labels[0]).toBe('API gateway')
  })

  it('triggers store hydration when resources are empty on open', async () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    // Microtask drain
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(loadResourcesMock).toHaveBeenCalled()
  })

  it('moveHighlight wraps around in both directions', () => {
    resourcesRef.value = [
      { id: 'r1', name: 'one', target: '/' } as never,
      { id: 'r2', name: 'two', target: '/' } as never,
    ]
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = ''
    const total = palette.results.value.length
    expect(palette.highlightIndex.value).toBe(0)
    palette.moveHighlight(-1)
    expect(palette.highlightIndex.value).toBe(total - 1)
    palette.moveHighlight(1)
    expect(palette.highlightIndex.value).toBe(0)
  })

  it('activate calls router.push with the highlighted route and closes', () => {
    resourcesRef.value = [
      { id: 'r1', name: 'one', target: '/' } as never,
    ]
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'one'
    const push = vi.fn()
    palette.activate(push)
    expect(push).toHaveBeenCalledWith({ name: 'ResourceDetail', params: { id: 'r1' } })
    expect(palette.open.value).toBe(false)
  })

  it('exposes a query duration after a search', () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'over'
    void palette.results.value
    expect(palette.lastQueryDurationMs.value).toBeGreaterThan(0)
  })
})
