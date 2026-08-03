import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'

import {
  fetchChannels,
  fetchChannel,
  createChannel,
  updateChannel,
  deleteChannel,
  testChannel,
  testChannelConfig,
  setDefault,
} from '@/services/notificationChannelService'
import { ServerError, ValidationError } from '@/core/errors'
import { server } from '@/test/msw/server'
import type { CreateNotificationChannel } from '@/types'

const makeChannels = (n: number, offset = 0) =>
  Array.from({ length: n }, (_, i) => ({
    id: `c${offset + i}`,
    name: `chan-${offset + i}`,
    type: 'slack',
    config: {},
  }))

describe('notificationChannelService', () => {
  describe('fetchChannels', () => {
    it('walks pages of per_page=100 and returns the flattened list', async () => {
      const page1 = makeChannels(100)
      const page2 = makeChannels(20, 100)
      server.use(
        http.get('*/v1/notification-channels', ({ request }) => {
          const page = new URL(request.url).searchParams.get('page')
          const data = page === '1' ? page1 : page2
          return HttpResponse.json({ data, meta: { page: Number(page), per_page: 100, total: 120 } })
        }),
      )
      const result = await fetchChannels()
      expect(result).toHaveLength(120)
      expect(result[0]?.id).toBe('c0')
      expect(result[119]?.id).toBe('c119')
    })

    it('stops after a single short page (no second request)', async () => {
      let calls = 0
      server.use(
        http.get('*/v1/notification-channels', () => {
          calls += 1
          return HttpResponse.json({
            data: makeChannels(2),
            meta: { page: 1, per_page: 100, total: 2 },
          })
        }),
      )
      const result = await fetchChannels()
      expect(result).toHaveLength(2)
      expect(calls).toBe(1)
    })

    it('propagates server errors as ServerError', async () => {
      server.use(
        http.get('*/v1/notification-channels', () => HttpResponse.json({}, { status: 500 })),
      )
      await expect(fetchChannels()).rejects.toBeInstanceOf(ServerError)
    })
  })

  describe('fetchChannel', () => {
    it('sends GET to /v1/notification-channels/:id and unwraps data', async () => {
      const chan = { id: 'c1', name: 'Ops', type: 'smtp', config: {} }
      server.use(
        http.get('*/v1/notification-channels/c1', () => HttpResponse.json({ data: chan })),
      )
      const result = await fetchChannel('c1')
      expect(result).toEqual(chan)
    })
  })

  describe('createChannel', () => {
    it('sends POST to /v1/notification-channels with payload and unwraps data', async () => {
      const payload: CreateNotificationChannel = {
        name: 'Ops',
        type: 'slack',
        config: { webhook_url: 'https://hooks.slack.com/services/T/B/X' },
        enabled_by_default: false,
      }
      const created = { id: 'c9', ...payload }
      let body: unknown = null
      server.use(
        http.post('*/v1/notification-channels', async ({ request }) => {
          body = await request.json()
          return HttpResponse.json({ data: created }, { status: 201 })
        }),
      )
      const result = await createChannel(payload)
      expect(body).toEqual(payload)
      expect(result).toEqual(created)
    })

    it('surfaces 422 validation errors as ValidationError with fieldErrors', async () => {
      server.use(
        http.post('*/v1/notification-channels', () =>
          HttpResponse.json({ detail: 'Invalid', fieldErrors: { name: ['required'] } }, { status: 422 }),
        ),
      )
      const payload: CreateNotificationChannel = {
        name: '',
        type: 'slack',
        config: { webhook_url: 'https://hooks.slack.com/x' },
        enabled_by_default: false,
      }
      try {
        await createChannel(payload)
        throw new Error('expected rejection')
      } catch (e) {
        expect(e).toBeInstanceOf(ValidationError)
        expect((e as ValidationError).fieldErrors).toEqual({ name: ['required'] })
      }
    })
  })

  describe('updateChannel', () => {
    it('sends PATCH to /v1/notification-channels/:id with payload and unwraps data', async () => {
      const updated = { id: 'c1', name: 'Renamed' }
      let body: unknown = null
      server.use(
        http.patch('*/v1/notification-channels/c1', async ({ request }) => {
          body = await request.json()
          return HttpResponse.json({ data: updated })
        }),
      )
      const result = await updateChannel('c1', { name: 'Renamed' })
      expect(body).toEqual({ name: 'Renamed' })
      expect(result).toEqual(updated)
    })
  })

  describe('deleteChannel', () => {
    it('sends DELETE to /v1/notification-channels/:id and resolves on 204', async () => {
      let called = false
      server.use(
        http.delete('*/v1/notification-channels/c1', () => {
          called = true
          return new HttpResponse(null, { status: 204 })
        }),
      )
      await deleteChannel('c1')
      expect(called).toBe(true)
    })
  })

  describe('setDefault', () => {
    it('PATCHes enabled_by_default: true', async () => {
      let body: unknown = null
      server.use(
        http.patch('*/v1/notification-channels/c1', async ({ request }) => {
          body = await request.json()
          return HttpResponse.json({ data: { id: 'c1' } })
        }),
      )
      await setDefault('c1')
      expect(body).toEqual({ enabled_by_default: true })
    })
  })

  describe('testChannel', () => {
    it('POST /:id/test returns delivered:true with a latency on 200', async () => {
      server.use(
        http.post('*/v1/notification-channels/c1/test', () =>
          HttpResponse.json({ data: { message: 'sent' } }),
        ),
      )
      const result = await testChannel('c1')
      expect(result.delivered).toBe(true)
      expect(typeof result.latency_ms).toBe('number')
    })

    it('POST /:id/test returns delivered:false with the error message on 422', async () => {
      server.use(
        http.post('*/v1/notification-channels/c1/test', () =>
          HttpResponse.json({ detail: 'SMTP refused' }, { status: 422 }),
        ),
      )
      const result = await testChannel('c1')
      expect(result.delivered).toBe(false)
      expect(result.error).toBe('SMTP refused')
    })
  })

  describe('testChannelConfig', () => {
    it('POST /test-config sends { type, config }', async () => {
      let body: unknown = null
      server.use(
        http.post('*/v1/notification-channels/test-config', async ({ request }) => {
          body = await request.json()
          return HttpResponse.json({ data: { message: 'ok' } })
        }),
      )
      await testChannelConfig({
        type: 'slack',
        config: { webhook_url: 'https://hooks.slack.com/x' },
      })
      expect(body).toEqual({ type: 'slack', config: { webhook_url: 'https://hooks.slack.com/x' } })
    })

    it('throws ValidationError on 422', async () => {
      server.use(
        http.post('*/v1/notification-channels/test-config', () =>
          HttpResponse.json({ detail: 'bad config' }, { status: 422 }),
        ),
      )
      await expect(
        testChannelConfig({ type: 'slack', config: { webhook_url: 'https://hooks.slack.com/x' } }),
      ).rejects.toBeInstanceOf(ValidationError)
    })
  })
})
