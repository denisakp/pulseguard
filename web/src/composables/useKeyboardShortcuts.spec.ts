import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

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

import {
  installKeyboardShortcuts,
  useKeyboardShortcuts,
  __resetKeyboardShortcutsForTests,
} from './useKeyboardShortcuts'
import { useSearchPalette, __resetSearchPaletteForTests } from './useSearchPalette'

describe('useKeyboardShortcuts', () => {
  let uninstall: () => void = () => undefined

  beforeEach(() => {
    setActivePinia(createPinia())
    __resetSearchPaletteForTests()
    __resetKeyboardShortcutsForTests()
    pushMock.mockClear()
    uninstall = installKeyboardShortcuts(fakeRouter)
  })

  afterEach(() => {
    uninstall()
    __resetKeyboardShortcutsForTests()
  })

  it('registers the documented shortcuts in the registry', () => {
    const ks = useKeyboardShortcuts()
    const ids = ks.shortcuts().map((s) => s.id)
    expect(ids).toContain('palette.open')
    expect(ids).toContain('shortcuts.open')
    expect(ids).toContain('nav.overview')
    expect(ids).toContain('nav.resources')
    expect(ids).toContain('nav.incidents')
    expect(ids).toContain('nav.status')
  })

  it('Ctrl+K toggles the search palette open', () => {
    const palette = useSearchPalette()
    expect(palette.open.value).toBe(false)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    expect(palette.open.value).toBe(true)
  })

  it('? opens the shortcuts modal', () => {
    const ks = useKeyboardShortcuts()
    expect(ks.modalOpen.value).toBe(false)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: '?' }))
    expect(ks.modalOpen.value).toBe(true)
  })

  it('? is ignored when an input is focused', () => {
    const ks = useKeyboardShortcuts()
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: '?' }))
    expect(ks.modalOpen.value).toBe(false)
    input.remove()
  })

  it('Ctrl+K still fires when an input is focused (FR-011)', () => {
    const palette = useSearchPalette()
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    expect(palette.open.value).toBe(true)
    input.remove()
  })

  it('G then O navigates to Overview within the chord window', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'o' }))
    expect(pushMock).toHaveBeenCalledWith({ name: 'Overview' })
  })

  it('G then R navigates to Resources, G I to Incidents, G S to Status', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'r' }))
    expect(pushMock).toHaveBeenLastCalledWith({ name: 'Resources' })

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'i' }))
    expect(pushMock).toHaveBeenLastCalledWith({ name: 'Incidents' })

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 's' }))
    expect(pushMock).toHaveBeenLastCalledWith({ name: 'SettingsStatusPage' })
  })

  it('chord buffer expires after 1200 ms', () => {
    vi.useFakeTimers()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    vi.advanceTimersByTime(1300)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'o' }))
    expect(pushMock).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('chord is cancelled by an unexpected key', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'x' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'o' }))
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('chord shortcuts do not fire when an input is focused', () => {
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'o' }))
    expect(pushMock).not.toHaveBeenCalled()
    input.remove()
  })

  it('Esc closes the modal when open', () => {
    const ks = useKeyboardShortcuts()
    ks.open()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(ks.modalOpen.value).toBe(false)
  })

  it('open/close imperative API works', () => {
    const ks = useKeyboardShortcuts()
    ks.open()
    expect(ks.modalOpen.value).toBe(true)
    ks.close()
    expect(ks.modalOpen.value).toBe(false)
  })
})
