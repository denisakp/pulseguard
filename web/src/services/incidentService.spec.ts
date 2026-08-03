import { describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'

import { fetchIncidents, fetchIncidentById, fetchUnresolvedIncidents } from '@/services/incidentService'
import { ServerError } from '@/core/errors'
import { server } from '@/test/msw/server'

const API = '*/api/v1'

const listEnvelope = (
  data: unknown[],
  meta: { page: number; per_page: number; total: number } = {
    page: 1,
    per_page: 50,
    total: data.length,
  },
) => HttpResponse.json({ data, meta })

function captureSearchParamsOn(pattern: string) {
  const captured: { value: URLSearchParams | null } = { value: null }
  server.use(
    http.get(pattern, ({ request }) => {
      captured.value = new URL(request.url).searchParams
      return listEnvelope([])
    }),
  )
  return captured
}

describe('incidentService', () => {
  describe('fetchIncidents', () => {
    it('sends GET to v1/incidents and unwraps the { data, meta } envelope', async () => {
      const incidents = [{ id: 'inc-1' }]
      server.use(
        http.get(`${API}/incidents`, () =>
          listEnvelope(incidents, { page: 2, per_page: 10, total: 25 }),
        ),
      )

      const result = await fetchIncidents({ page: 2, per_page: 10 })
      expect(result.data).toEqual(incidents)
      expect(result.total).toBe(25)
      expect(result.limit).toBe(10)
      expect(result.offset).toBe(10) // (page 2 - 1) * per_page 10
    })

    it('maps unresolved=true onto status=open', async () => {
      const captured = captureSearchParamsOn(`${API}/incidents`)
      await fetchIncidents({ unresolved: true })
      expect(captured.value?.get('status')).toBe('open')
      expect(captured.value?.get('unresolved')).toBeNull()
    })

    it('passes status, monitor_id, page and per_page through', async () => {
      const captured = captureSearchParamsOn(`${API}/incidents`)
      await fetchIncidents({ status: 'resolved', monitor_id: 'm1', page: 3, per_page: 25 })
      expect(captured.value?.get('status')).toBe('resolved')
      expect(captured.value?.get('monitor_id')).toBe('m1')
      expect(captured.value?.get('page')).toBe('3')
      expect(captured.value?.get('per_page')).toBe('25')
    })

    it('does not send legacy resource_id / limit / offset params', async () => {
      const captured = captureSearchParamsOn(`${API}/incidents`)
      await fetchIncidents({ monitor_id: 'm1', per_page: 5 })
      expect(captured.value?.get('resource_id')).toBeNull()
      expect(captured.value?.get('limit')).toBeNull()
      expect(captured.value?.get('offset')).toBeNull()
    })

    it('propagates errors as typed ApiError', async () => {
      server.use(http.get(`${API}/incidents`, () => HttpResponse.json({}, { status: 500 })))
      await expect(fetchIncidents()).rejects.toBeInstanceOf(ServerError)
    })
  })

  describe('fetchIncidentById', () => {
    it('sends GET to v1/incidents/:id and unwraps { data }', async () => {
      const incident = { id: 'inc-1', cause: 'timeout' }
      server.use(http.get(`${API}/incidents/inc-1`, () => HttpResponse.json({ data: incident })))

      const result = await fetchIncidentById('inc-1')
      expect(result).toEqual(incident)
    })
  })

  describe('fetchUnresolvedIncidents', () => {
    it('requests status=open and returns the unwrapped data array', async () => {
      const incidents = [{ id: 'inc-1' }, { id: 'inc-2' }]
      server.use(
        http.get(`${API}/incidents`, ({ request }) => {
          const url = new URL(request.url)
          if (url.searchParams.get('status') === 'open') {
            return listEnvelope(incidents)
          }
          return listEnvelope([])
        }),
      )

      const result = await fetchUnresolvedIncidents()
      expect(result).toEqual(incidents)
    })

    it('propagates errors', async () => {
      server.use(http.get(`${API}/incidents`, () => HttpResponse.json({}, { status: 503 })))
      await expect(fetchUnresolvedIncidents()).rejects.toBeInstanceOf(ServerError)
    })
  })
})
