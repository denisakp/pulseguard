import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { flushPromises } from '@vue/test-utils'
import { ForbiddenError } from '@/core/errors'

const listHostsMock = vi.fn()
const linkMock = vi.fn()
const unlinkMock = vi.fn()

vi.mock('@/services/hostsService', () => ({
  listHosts: (...a: unknown[]) => listHostsMock(...a),
  linkMonitorToHost: (...a: unknown[]) => linkMock(...a),
  unlinkMonitorFromHost: (...a: unknown[]) => unlinkMock(...a),
}))

const toastAddMock = vi.fn()
vi.mock('@nuxt/ui/composables/useToast', () => ({
  useToast: () => ({ add: toastAddMock }),
}))

import LinkHostControl from './LinkHostControl.vue'

const HOSTS = [
  { id: 'h1', name: 'web-01' },
  { id: 'h2', name: 'web-02' },
]

const stubs = {
  USelect: {
    template: '<div data-testid="select" />',
    props: ['modelValue', 'items', 'loading', 'placeholder', 'icon'],
    emits: ['update:modelValue'],
  },
  UButton: {
    template: '<button :disabled="disabled || loading" v-bind="$attrs"><slot /></button>',
    props: ['color', 'variant', 'size', 'icon', 'disabled', 'loading'],
    inheritAttrs: false,
    emits: ['click'],
  },
}

function mountControl(currentHostId: string | null = null) {
  return mount(LinkHostControl, {
    global: { stubs },
    props: { monitorId: 'm1', currentHostId },
  })
}

describe('LinkHostControl', () => {
  beforeEach(() => {
    listHostsMock.mockReset().mockResolvedValue(HOSTS)
    linkMock.mockReset().mockResolvedValue(undefined)
    unlinkMock.mockReset().mockResolvedValue(undefined)
    toastAddMock.mockReset()
  })

  afterEach(() => vi.clearAllMocks())

  it('loads hosts on mount', async () => {
    const w = mountControl()
    await flushPromises()
    expect(listHostsMock).toHaveBeenCalledOnce()
    expect((w.vm as unknown as { hosts: unknown[] }).hosts).toHaveLength(2)
  })

  it('linking calls the service with the chosen host and emits changed', async () => {
    const w = mountControl(null)
    await flushPromises()
    ;(w.vm as unknown as { selectedHostId: string }).selectedHostId = 'h2'
    await (w.vm as unknown as { link: () => Promise<void> }).link()
    expect(linkMock).toHaveBeenCalledWith('m1', 'h2')
    expect(w.emitted('changed')?.[0]).toEqual(['h2'])
  })

  it('unlinking calls unlink and emits null', async () => {
    const w = mountControl('h1')
    await flushPromises()
    await (w.vm as unknown as { unlink: () => Promise<void> }).unlink()
    expect(unlinkMock).toHaveBeenCalledWith('m1')
    expect(w.emitted('changed')?.[0]).toEqual([null])
  })

  it('catches a 403 on link without crashing and surfaces a toast', async () => {
    linkMock.mockRejectedValueOnce(new ForbiddenError())
    const w = mountControl(null)
    await flushPromises()
    ;(w.vm as unknown as { selectedHostId: string }).selectedHostId = 'h1'
    await (w.vm as unknown as { link: () => Promise<void> }).link()
    expect(w.emitted('changed')).toBeUndefined()
    expect(toastAddMock).toHaveBeenCalledOnce()
    expect(toastAddMock.mock.calls[0]?.[0]?.color).toBe('error')
  })

  it('catches a 403 on unlink without crashing', async () => {
    unlinkMock.mockRejectedValueOnce(new ForbiddenError())
    const w = mountControl('h1')
    await flushPromises()
    await (w.vm as unknown as { unlink: () => Promise<void> }).unlink()
    expect(w.emitted('changed')).toBeUndefined()
    expect(toastAddMock).toHaveBeenCalledOnce()
  })
})
