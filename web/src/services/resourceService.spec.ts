import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'

import {
  fetchResources,
  fetchResource,
  createResource,
  updateResource,
  deleteResource,
  pauseResource,
  resumeResource,
  addTagsToResource,
  removeTagFromResource,
  fetchUptimeStats,
  fetchCapabilities,
} from '@/services/resourceService'
import { ServerError, ValidationError } from '@/core/errors'
import { server } from '@/test/msw/server'

describe('resourceService', () => {
  describe('fetchResources', () => {
    it('sends GET to /v1/monitors and unwraps the paginated envelope', async () => {
      const resources = [{ id: 'r1', name: 'API Server' }]
      server.use(
        http.get('*/v1/monitors', () =>
          HttpResponse.json({ data: resources, meta: { page: 1, per_page: 100, total: 1 } }),
        ),
      )
      const result = await fetchResources()
      expect(result).toEqual(resources)
    })

    it('propagates server errors as ServerError', async () => {
      server.use(http.get('*/v1/monitors', () => HttpResponse.json({}, { status: 500 })))
      await expect(fetchResources()).rejects.toBeInstanceOf(ServerError)
    })
  })

  describe('fetchResource', () => {
    it('sends GET to /v1/monitors/:id and unwraps data', async () => {
      const resource = { id: 'r1', name: 'API Server' }
      server.use(http.get('*/v1/monitors/r1', () => HttpResponse.json({ data: resource })))
      const result = await fetchResource('r1')
      expect(result).toEqual(resource)
    })

    it('passes limit as query param when provided', async () => {
      let limit: string | null = null
      server.use(
        http.get('*/v1/monitors/r1', ({ request }) => {
          limit = new URL(request.url).searchParams.get('limit')
          return HttpResponse.json({ data: { id: 'r1' } })
        }),
      )
      await fetchResource('r1', 50)
      expect(limit).toBe('50')
    })
  })

  describe('createResource', () => {
    it('sends POST to /v1/monitors with payload and success-message header', async () => {
      const newResource = { name: 'New Monitor', url: 'https://example.com' }
      const created = { id: 'r2', ...newResource }
      let body: unknown = null
      let successHeader: string | null = null
      server.use(
        http.post('*/v1/monitors', async ({ request }) => {
          body = await request.json()
          successHeader = request.headers.get('x-success-message')
          return HttpResponse.json({ data: created }, { status: 201 })
        }),
      )

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const result = await createResource(newResource as any)
      expect(body).toEqual(newResource)
      expect(successHeader).toBe('Monitor created successfully')
      expect(result).toEqual(created)
    })

    it('surfaces 422 validation errors as ValidationError with fieldErrors', async () => {
      server.use(
        http.post('*/v1/monitors', () =>
          HttpResponse.json(
            { detail: 'Invalid', fieldErrors: { name: ['required'] } },
            { status: 422 },
          ),
        ),
      )

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const p = createResource({} as any)
      await expect(p).rejects.toBeInstanceOf(ValidationError)
      try {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        await createResource({} as any)
      } catch (e) {
        expect((e as ValidationError).fieldErrors).toEqual({ name: ['required'] })
      }
    })

    it('also normalizes 400 validation errors as ValidationError', async () => {
      server.use(
        http.post('*/v1/monitors', () =>
          HttpResponse.json({ detail: 'Bad', fieldErrors: { url: ['invalid'] } }, { status: 400 }),
        ),
      )
      try {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        await createResource({} as any)
      } catch (e) {
        expect(e).toBeInstanceOf(ValidationError)
        expect((e as ValidationError).fieldErrors).toEqual({ url: ['invalid'] })
      }
    })
  })

  describe('updateResource', () => {
    it('sends PATCH to /v1/monitors/:id with payload and success-message header', async () => {
      const updates = { name: 'Updated Monitor' }
      const updated = { id: 'r1', name: 'Updated Monitor' }
      let body: unknown = null
      let successHeader: string | null = null
      server.use(
        http.patch('*/v1/monitors/r1', async ({ request }) => {
          body = await request.json()
          successHeader = request.headers.get('x-success-message')
          return HttpResponse.json({ data: updated })
        }),
      )

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const result = await updateResource('r1', updates as any)
      expect(body).toEqual(updates)
      expect(successHeader).toBe('Monitor updated successfully')
      expect(result).toEqual(updated)
    })
  })

  describe('deleteResource', () => {
    it('sends DELETE to /v1/monitors/:id with success-message header and reaches success on 204', async () => {
      let successHeader: string | null = null
      server.use(
        http.delete('*/v1/monitors/r1', ({ request }) => {
          successHeader = request.headers.get('x-success-message')
          return new HttpResponse(null, { status: 204 })
        }),
      )
      await deleteResource('r1')
      expect(successHeader).toBe('Monitor deleted successfully')
    })
  })

  describe('pauseResource', () => {
    it('sends POST to /v1/monitors/:id/pause and unwraps data', async () => {
      const paused = { id: 'r1', status: 'paused' }
      server.use(http.post('*/v1/monitors/r1/pause', () => HttpResponse.json({ data: paused })))
      const result = await pauseResource('r1')
      expect(result).toEqual(paused)
    })
  })

  describe('resumeResource', () => {
    it('sends POST to /v1/monitors/:id/resume and unwraps data', async () => {
      const resumed = { id: 'r1', status: 'active' }
      server.use(http.post('*/v1/monitors/r1/resume', () => HttpResponse.json({ data: resumed })))
      const result = await resumeResource('r1')
      expect(result).toEqual(resumed)
    })
  })

  describe('addTagsToResource', () => {
    it('sends POST to /v1/monitors/:id/tags with tag_ids', async () => {
      let body: { tag_ids: string[] } | null = null
      server.use(
        http.post('*/v1/monitors/r1/tags', async ({ request }) => {
          body = (await request.json()) as typeof body
          return new HttpResponse(null, { status: 204 })
        }),
      )
      await addTagsToResource('r1', ['t1', 't2'])
      expect(body).toEqual({ tag_ids: ['t1', 't2'] })
    })
  })

  describe('removeTagFromResource', () => {
    it('sends DELETE to /v1/monitors/:id/tags/:tagId', async () => {
      let called = false
      server.use(
        http.delete('*/v1/monitors/r1/tags/t1', () => {
          called = true
          return new HttpResponse(null, { status: 204 })
        }),
      )
      await removeTagFromResource('r1', 't1')
      expect(called).toBe(true)
    })
  })

  describe('fetchUptimeStats', () => {
    it('sends GET to /v1/monitors/:id/uptime-stats and unwraps data', async () => {
      const stats = { resource_id: 'r1', stats: [] }
      server.use(
        http.get('*/v1/monitors/r1/uptime-stats', () => HttpResponse.json({ data: stats })),
      )
      const result = await fetchUptimeStats('r1')
      expect(result).toEqual(stats)
    })
  })

  describe('fetchCapabilities', () => {
    it('sends GET to /system/capabilities', async () => {
      const caps = { icmp_available: true }
      server.use(http.get('*/system/capabilities', () => HttpResponse.json(caps)))
      const result = await fetchCapabilities()
      expect(result).toEqual(caps)
    })
  })
})
