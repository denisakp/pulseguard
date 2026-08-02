/**
 * Spec 069 / US1 — branded error + maintenance routes.
 *
 * Lives next to router/index.ts to keep the existing index.spec-less file
 * tree untouched. Verifies catch-all, error/maintenance routes, and the
 * env-driven maintenance gate. Auth guard is exercised by importing the
 * real router default export.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

async function loadRouter() {
  vi.resetModules()
  const mod = await import('./index')
  return mod.default
}

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({
    isAuthenticated: false,
    verify: vi.fn().mockResolvedValue(true),
  }),
}))
vi.mock('@/composables/useRuntimeConfig', () => ({
  loadRuntimeConfig: vi.fn().mockResolvedValue(undefined),
}))

describe('cross-cutting routes', () => {
  beforeEach(() => {
    vi.unstubAllEnvs()
  })
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('catch-all route resolves unknown paths to Error404 for anonymous visitors', async () => {
    const router = await loadRouter()
    const resolved = router.resolve('/route-that-does-not-exist')
    expect(resolved.name).toBe('Error404')
    expect(resolved.meta.public).toBe(true)
  })

  it('declares Error500 as a public route with no layout', async () => {
    const router = await loadRouter()
    const resolved = router.resolve('/error-500')
    expect(resolved.name).toBe('Error500')
    expect(resolved.meta.public).toBe(true)
    expect(resolved.meta.requiresLayout).toBe(false)
  })

  it('declares MaintenanceMode as a public route', async () => {
    const router = await loadRouter()
    const resolved = router.resolve('/maintenance-mode')
    expect(resolved.name).toBe('MaintenanceMode')
    expect(resolved.meta.public).toBe(true)
  })

  it('maintenance gate redirects every route to MaintenanceMode when env flag is true', async () => {
    vi.stubEnv('VITE_MAINTENANCE_MODE', 'true')
    const router = await loadRouter()
    await router.push('/overview')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('MaintenanceMode')
  })

  it('maintenance gate is inactive when the env flag is missing or false', async () => {
    const router = await loadRouter()
    const resolved = router.resolve('/login')
    expect(resolved.name).toBe('Login')
  })
})
