import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const testChannelMock = vi.fn()
vi.mock('@/services/notificationChannelService', () => ({
  testChannel: (...a: unknown[]) => testChannelMock(...a),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), resolve: () => ({ href: '#' }) }),
  useRoute: () => ({ path: '/settings/notifications', params: {}, query: {}, name: 'x' }),
  useLink: () => ({ href: { value: '#' }, navigate: vi.fn(), isActive: { value: false } }),
  RouterLink: { template: '<a><slot /></a>' },
}))

import ChannelModal from '../ChannelModal.vue'

type Vm = {
  type: string
  name: string
  config: Record<string, unknown>
  fieldError: Record<string, string>
  testResult: { delivered: boolean; latency_ms: number; error?: string } | null
  validate: () => unknown
  onSubmit: () => Promise<void>
  onSendTest: () => Promise<void>
}

beforeEach(() => testChannelMock.mockReset())

describe('ChannelModal', () => {
  it('tab swap from smtp → slack clears previous config payload', async () => {
    const w = mount(ChannelModal, { props: { open: true } })
    const vm = w.vm as unknown as Vm
    vm.name = 'mail'
    vm.config = { ...vm.config, host: 'smtp.example.com' }
    vm.type = 'slack'
    await flushPromises()
    expect((vm.config as { host?: string }).host).toBeUndefined()
    expect((vm.config as { webhook_url?: string }).webhook_url).toBe('')
  })

  it('submit emits a discriminated payload (type + config) after validation', async () => {
    const w = mount(ChannelModal, { props: { open: true } })
    const vm = w.vm as unknown as Vm
    vm.type = 'slack'
    await flushPromises()
    vm.name = 'oncall'
    vm.config = {
      webhook_url: 'https://hooks.slack.com/services/T/B/X',
      channel: 'oncall',
    }
    await vm.onSubmit()
    const emitted = w.emitted('submit')?.[0]?.[0] as { type: string; config: { channel: string } }
    expect(emitted?.type).toBe('slack')
    expect(emitted?.config.channel).toBe('oncall')
  })

  it('create mode requires the smtp password (validate fails, field error set)', async () => {
    const w = mount(ChannelModal, { props: { open: true } })
    const vm = w.vm as unknown as Vm
    vm.name = 'Ops'
    vm.config = {
      host: 'smtp.example.com',
      port: 587,
      username: 'ops',
      password: '',
      sender: 'noreply@example.com',
      recipient: 'ops@example.com',
    }
    const r = vm.validate()
    expect(r).toBeNull()
    expect(vm.fieldError['config.password']).toBeTruthy()
  })

  it('edit mode: smtp channel with a masked (blank) password validates and emits', async () => {
    const w = mount(ChannelModal, {
      props: {
        open: true,
        initial: {
          id: 'c1',
          type: 'smtp',
          name: 'Ops',
          // password absent — GET masks it out
          config: {
            host: 'smtp.example.com',
            port: 587,
            username: 'ops',
            sender: 'noreply@example.com',
            recipient: 'ops@example.com',
          },
        },
      },
    })
    const vm = w.vm as unknown as Vm
    await flushPromises()
    await vm.onSubmit()
    const emitted = w.emitted('submit')?.[0]?.[0] as { type: string; config: { host: string } }
    expect(emitted?.type).toBe('smtp')
    expect(emitted?.config.host).toBe('smtp.example.com')
    expect(vm.fieldError['config.password']).toBeUndefined()
  })

  it('Send test surfaces inline result via testResult', async () => {
    testChannelMock.mockResolvedValue({ delivered: true, latency_ms: 42 })
    const w = mount(ChannelModal, {
      props: {
        open: true,
        initial: {
          id: 'c1',
          type: 'webhook',
          name: 'pd',
          config: { url: 'https://example.com', method: 'POST', headers: [] },
        },
      },
    })
    const vm = w.vm as unknown as Vm
    await vm.onSendTest()
    await flushPromises()
    expect(testChannelMock).toHaveBeenCalledWith('c1')
    expect(vm.testResult?.delivered).toBe(true)
    expect(vm.testResult?.latency_ms).toBe(42)
  })
})
