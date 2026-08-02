import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import HostCredentialReveal from './HostCredentialReveal.vue'

const stubs = {
  UAlert: { template: '<div />' },
  UButton: { template: '<button><slot /></button>' },
}

describe('HostCredentialReveal (SC-004)', () => {
  beforeEach(() => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    })
  })

  it('renders the raw secret once with a copy button', () => {
    const w = mount(HostCredentialReveal, {
      props: { credential: 'ag_live_secret123', prefix: 'ag_live' },
      global: { stubs },
    })
    expect(w.find('[data-testid="host-credential"]').text()).toContain('ag_live_secret123')
    expect(w.get('[aria-label="Copy credential"]')).toBeTruthy()
  })

  it('copies the credential to the clipboard', async () => {
    const w = mount(HostCredentialReveal, {
      props: { credential: 'ag_live_secret123', prefix: 'ag_live' },
      global: { stubs },
    })
    await w.get('[aria-label="Copy credential"]').trigger('click')
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('ag_live_secret123')
  })

  it('clears the secret and emits close when dismissed (SC-004)', async () => {
    const w = mount(HostCredentialReveal, {
      props: { credential: 'ag_live_secret123', prefix: 'ag_live' },
      global: { stubs },
    })
    // The "Done" button is the last stubbed UButton.
    const buttons = w.findAll('button')
    await buttons[buttons.length - 1]!.trigger('click')
    expect(w.emitted('close')).toBeTruthy()
    expect(w.find('[data-testid="host-credential"]').text()).not.toContain('ag_live_secret123')
  })
})
