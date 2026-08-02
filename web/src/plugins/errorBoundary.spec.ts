import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, defineComponent, h } from 'vue'

const isNavigationFailureMock = vi.fn((_e?: unknown) => false)
vi.mock('vue-router', () => ({
  isNavigationFailure: (e: unknown) => isNavigationFailureMock(e),
}))

import { installErrorBoundary, __resetErrorBoundaryForTests } from './errorBoundary'

describe('installErrorBoundary', () => {
  const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

  beforeEach(() => {
    __resetErrorBoundaryForTests()
    consoleSpy.mockClear()
    isNavigationFailureMock.mockReturnValue(false)
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('pushes to Error500 with a synthetic incident on the first error', () => {
    const pushSpy = vi.fn().mockResolvedValue(undefined)
    const fakeRouter = { push: pushSpy } as never
    const app = createApp(defineComponent({ render: () => h('div') }))
    installErrorBoundary(app, fakeRouter)

    const boom = new Error('boom')
    app.config.errorHandler?.(boom, null as never, 'render')

    expect(consoleSpy).toHaveBeenCalled()
    expect(pushSpy).toHaveBeenCalledTimes(1)
    const arg = pushSpy.mock.calls[0]![0]
    expect(arg.name).toBe('Error500')
    expect(arg.state.incidentId).toMatch(/INC-\d{4}-[A-Z2-7]{6}/)
    expect(arg.state.originalMessage).toBe('boom')
    expect(typeof arg.state.occurredAt).toBe('string')
  })

  it('renders a static HTML fallback on a re-entrant failure', () => {
    const pushSpy = vi.fn().mockResolvedValue(undefined)
    const fakeRouter = { push: pushSpy } as never
    const app = createApp(defineComponent({ render: () => h('div') }))
    installErrorBoundary(app, fakeRouter)

    app.config.errorHandler?.(new Error('first'), null as never, 'render')
    // Second error before the first navigation resolves → fallback HTML.
    app.config.errorHandler?.(new Error('second'), null as never, 'render')

    expect(document.body.innerHTML).toContain('Something went very wrong')
    expect(document.body.innerHTML).toContain('Reload')
    expect(pushSpy).toHaveBeenCalledTimes(1)
  })

  it('logs the original error message to the console', () => {
    const fakeRouter = { push: vi.fn().mockResolvedValue(undefined) } as never
    const app = createApp(defineComponent({ render: () => h('div') }))
    installErrorBoundary(app, fakeRouter)

    const original = new Error('kaboom')
    app.config.errorHandler?.(original, null as never, 'lifecycle')

    expect(consoleSpy).toHaveBeenCalledWith('[errorBoundary]', original, 'lifecycle')
  })

  it('IGNORES Vue Router NavigationFailure errors (no Error500 push, no log)', () => {
    isNavigationFailureMock.mockReturnValue(true)
    const pushSpy = vi.fn().mockResolvedValue(undefined)
    const fakeRouter = { push: pushSpy } as never
    const app = createApp(defineComponent({ render: () => h('div') }))
    installErrorBoundary(app, fakeRouter)

    const navFailure = new Error('Navigation cancelled')
    app.config.errorHandler?.(navFailure, null as never, 'router')

    expect(pushSpy).not.toHaveBeenCalled()
    expect(consoleSpy).not.toHaveBeenCalled()
  })
})
