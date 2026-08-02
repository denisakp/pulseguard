import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { hostSchema } from '@/schemas/host'

const registerHostMock = vi.fn()
vi.mock('@/services/hostsService', () => ({
  registerHost: (input: { name: string }) => registerHostMock(input),
}))

import RegisterHostModal from './RegisterHostModal.vue'

// NuxtUI components register under their real names (no `U` prefix). The Form
// stub emits `submit` only when the test triggers it — mirroring how real UForm
// gates submit on schema validity.
const FormStub = {
  name: 'Form',
  template: '<form data-testid="form"><slot /></form>',
  props: ['schema', 'state'],
  emits: ['submit'],
}

const stubs = {
  Modal: { template: '<div><slot name="body" /></div>' },
  Form: FormStub,
  FormField: { template: '<div><slot /></div>' },
  Input: { template: '<input />' },
  Button: { template: '<button><slot /></button>' },
  Alert: { template: '<div />' },
  HostCredentialReveal: {
    name: 'HostCredentialReveal',
    template: '<div data-testid="reveal">{{ credential }}</div>',
    props: ['credential', 'prefix'],
  },
}

function sampleHost() {
  return {
    id: 'h1',
    name: 'web-1',
    os: null,
    agentVersion: null,
    online: false,
    lastSeenAt: null,
    lastCpuPct: null,
    lastMemPct: null,
    lastDiskPct: null,
    lastNetIn: null,
    lastNetOut: null,
    lastDisks: [],
    createdAt: '2026-07-30T00:00:00Z',
    updatedAt: '2026-07-30T00:00:00Z',
  }
}

describe('RegisterHostModal', () => {
  beforeEach(() => {
    registerHostMock.mockReset()
  })

  it('schema blocks an empty name and accepts a valid one (validation gate)', () => {
    expect(hostSchema.safeParse({ name: '' }).success).toBe(false)
    expect(hostSchema.safeParse({ name: '   ' }).success).toBe(false)
    expect(hostSchema.safeParse({ name: 'web-1' }).success).toBe(true)
  })

  it('on submit registers the host and reveals the credential once', async () => {
    registerHostMock.mockResolvedValue({
      host: sampleHost(),
      credential: 'ag_live_abc123',
      prefix: 'ag_live',
    })
    const w = mount(RegisterHostModal, { props: { open: true }, global: { stubs } })

    w.findComponent(FormStub).vm.$emit('submit', { data: { name: 'web-1' } })
    await flushPromises()

    expect(registerHostMock).toHaveBeenCalledWith({ name: 'web-1' })
    expect(w.emitted('registered')).toHaveLength(1)
    const reveal = w.find('[data-testid="reveal"]')
    expect(reveal.exists()).toBe(true)
    expect(reveal.text()).toContain('ag_live_abc123')
    // Form is gone once revealed.
    expect(w.find('[data-testid="form"]').exists()).toBe(false)
  })
})
