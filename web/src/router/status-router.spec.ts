import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { statusRoutes } from './status-router'

describe('status-router', () => {
  const buildRouter = () => createRouter({ history: createMemoryHistory(), routes: statusRoutes })

  it('resolves / to the current snapshot view', () => {
    const router = buildRouter()
    const resolved = router.resolve('/')
    expect(resolved.name).toBe('PublicStatusCurrent')
    expect(typeof resolved.matched[0]?.components?.default).toBe('function')
  })

  it('resolves /history to the incident archive view', () => {
    const router = buildRouter()
    const resolved = router.resolve('/history')
    expect(resolved.name).toBe('PublicStatusHistory')
  })

  it('resolves /uptime to the calendar view', () => {
    const router = buildRouter()
    const resolved = router.resolve('/uptime')
    expect(resolved.name).toBe('PublicStatusUptime')
  })

  it('resolves /resource/:id preserving the param', () => {
    const router = buildRouter()
    const resolved = router.resolve('/resource/res-123')
    expect(resolved.name).toBe('PublicStatusResource')
    expect(resolved.params.id).toBe('res-123')
  })

  it('redirects unknown paths to /', () => {
    const catchAll = statusRoutes.find((r) => r.path === '/:pathMatch(.*)*')
    expect(catchAll?.redirect).toBe('/')
  })
})
