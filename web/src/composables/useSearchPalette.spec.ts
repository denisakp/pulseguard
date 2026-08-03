import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

const loadResourcesMock = vi.fn().mockResolvedValue(undefined)
const resourcesRef = ref<unknown[]>([])
const incidentsRef = ref<unknown[]>([])

// Backend search client — mocked so the composable's debounced fetch and its
// offline fallback can both be exercised deterministically.
const searchPaletteApiMock = vi.fn()
vi.mock('@/services/searchService', () => ({
  searchPalette: (...args: unknown[]) => searchPaletteApiMock(...args),
}))

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

// Advance past the 150ms debounce and flush the awaited fetch microtasks.
async function flushDebounce() {
  await vi.advanceTimersByTimeAsync(200)
}

describe('useSearchPalette', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    __resetSearchPaletteForTests()
    loadResourcesMock.mockClear()
    searchPaletteApiMock.mockReset()
    resourcesRef.value = []
    incidentsRef.value = []
  })

  afterEach(() => {
    vi.useRealTimers()
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

  it('queries the backend (debounced) for queries of 2+ chars and maps deep_link to route', async () => {
    searchPaletteApiMock.mockResolvedValue({
      results: [
        {
          id: 'resource:r1',
          category: 'resource',
          label: 'API gateway',
          meta: 'https://api.example.com',
          route: '/resources/r1',
          score: 0,
        },
      ],
      durationMs: 12,
    })

    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'gateway'
    await flushDebounce()

    expect(searchPaletteApiMock).toHaveBeenCalledWith('gateway', { limit: 30 })
    expect(palette.results.value.map((r) => r.label)).toEqual(['API gateway'])
    expect(palette.results.value[0]?.route).toBe('/resources/r1')
    expect(palette.lastQueryDurationMs.value).toBe(12)
  })

  it('debounces rapid typing into a single backend call', async () => {
    searchPaletteApiMock.mockResolvedValue({ results: [], durationMs: 3 })
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'ga'
    palette.query.value = 'gat'
    palette.query.value = 'gate'
    await flushDebounce()
    expect(searchPaletteApiMock).toHaveBeenCalledTimes(1)
    expect(searchPaletteApiMock).toHaveBeenCalledWith('gate', { limit: 30 })
  })

  it('falls back to local Fuse search when the backend is unreachable', async () => {
    searchPaletteApiMock.mockRejectedValue(new Error('network down'))
    resourcesRef.value = [
      { id: 'r1', name: 'API gateway', target: 'https://api.example.com' } as never,
      { id: 'r2', name: 'Postgres primary', target: 'postgres://db' } as never,
    ]

    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'gateway'
    await flushDebounce()

    const labels = palette.results.value.map((r) => r.label)
    expect(labels[0]).toBe('API gateway')
    expect(palette.lastQueryDurationMs.value).toBeGreaterThan(0)
  })

  it('handles sub-2-char queries locally without hitting the backend', async () => {
    resourcesRef.value = [
      { id: 'r1', name: 'API gateway', target: 'https://api.example.com' } as never,
    ]
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = 'a'
    await flushDebounce()
    // Below the server minimum: served from the local corpus, no network call.
    expect(searchPaletteApiMock).not.toHaveBeenCalled()
  })

  it('triggers store hydration when resources are empty on open', async () => {
    const palette = useSearchPalette()
    palette.setOpen(true)
    await vi.advanceTimersByTimeAsync(0)
    expect(loadResourcesMock).toHaveBeenCalled()
  })

  it('moveHighlight wraps around in both directions (browse mode)', () => {
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

  it('activate pushes the highlighted route (raw path) and closes', () => {
    resourcesRef.value = [{ id: 'r1', name: 'one', target: '/' } as never]
    const palette = useSearchPalette()
    palette.setOpen(true)
    palette.query.value = ''
    // Browse mode: first resource in the corpus is highlighted.
    const push = vi.fn()
    palette.activate(push)
    expect(push).toHaveBeenCalledWith('/resources/r1')
    expect(palette.open.value).toBe(false)
  })
})
