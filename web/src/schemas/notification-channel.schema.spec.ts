import { describe, expect, it } from 'vitest'
import {
  notificationChannelSchema,
  channelSchemaFor,
  emptyConfigForType,
  stripBlankSecretKeys,
} from './notification-channel.schema'

describe('notificationChannelSchema', () => {
  it('accepts a valid smtp channel', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'smtp',
      name: 'Ops mailbox',
      is_default: false,
      is_active: true,
      config: {
        host: 'smtp.example.com',
        port: 587,
        username: 'ops',
        password: 'p4ss',
        sender: 'noreply@example.com',
        recipient: 'ops@example.com',
      },
    })
    expect(r.success).toBe(true)
  })

  it('rejects smtp without sender email', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'smtp',
      name: 'x',
      config: {
        host: 'smtp.example.com',
        port: 587,
        username: 'ops',
        password: 'p',
        sender: 'not-an-email',
        recipient: 'ops@example.com',
      },
    })
    expect(r.success).toBe(false)
  })

  it('accepts a valid slack channel', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'slack',
      name: 'oncall',
      config: {
        webhook_url: 'https://hooks.slack.com/services/T/B/X',
        channel: 'oncall',
      },
    })
    expect(r.success).toBe(true)
  })

  it('rejects slack with non-slack webhook host', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'slack',
      name: 'oncall',
      config: {
        webhook_url: 'https://example.com/hook',
        channel: 'oncall',
      },
    })
    expect(r.success).toBe(false)
  })

  it('accepts a valid webhook channel with custom headers', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'webhook',
      name: 'PagerDuty',
      config: {
        url: 'https://events.pagerduty.com/v2/enqueue',
        method: 'POST',
        headers: [{ name: 'X-Routing-Key', value: 'abc' }],
      },
    })
    expect(r.success).toBe(true)
  })

  it('rejects webhook with invalid URL', () => {
    const r = notificationChannelSchema.safeParse({
      type: 'webhook',
      name: 'bad',
      config: { url: 'not-a-url', method: 'POST', headers: [] },
    })
    expect(r.success).toBe(false)
  })

  it('create mode requires the smtp password', () => {
    const r = channelSchemaFor('create').safeParse({
      type: 'smtp',
      name: 'Ops mailbox',
      config: {
        host: 'smtp.example.com',
        port: 587,
        username: 'ops',
        password: '',
        sender: 'noreply@example.com',
        recipient: 'ops@example.com',
      },
    })
    expect(r.success).toBe(false)
  })

  it('edit mode accepts an smtp channel with a blank (masked) password', () => {
    const r = channelSchemaFor('edit').safeParse({
      type: 'smtp',
      name: 'Ops mailbox',
      config: {
        host: 'smtp.example.com',
        port: 587,
        username: 'ops',
        // password omitted — masked by GET, left blank on edit
        sender: 'noreply@example.com',
        recipient: 'ops@example.com',
      },
    })
    expect(r.success).toBe(true)
  })

  it('emptyConfigForType returns sane defaults for each type', () => {
    const s = emptyConfigForType('smtp') as { port: number }
    expect(s.port).toBe(587)
    const sl = emptyConfigForType('slack') as { channel: string }
    expect(sl.channel).toBe('')
    const w = emptyConfigForType('webhook') as { method: string; headers: unknown[] }
    expect(w.method).toBe('POST')
    expect(w.headers).toEqual([])
  })
})

describe('stripBlankSecretKeys', () => {
  it('drops blank/absent secret keys so the backend preserves the stored value', () => {
    const out = stripBlankSecretKeys({
      host: 'smtp.example.com',
      password: '',
      auth_token: undefined,
      username: 'ops',
    })
    expect(out).not.toHaveProperty('password')
    expect(out).not.toHaveProperty('auth_token')
    expect(out).toEqual({ host: 'smtp.example.com', username: 'ops' })
  })

  it('keeps a newly typed non-empty secret', () => {
    const out = stripBlankSecretKeys({ username: 'ops', password: 'n3w' })
    expect(out.password).toBe('n3w')
  })

  it('leaves non-secret keys untouched', () => {
    const out = stripBlankSecretKeys({ webhook_url: '', channel: 'oncall' })
    // webhook_url is NOT a masked secret, so an empty value is preserved
    expect(out).toEqual({ webhook_url: '', channel: 'oncall' })
  })
})
