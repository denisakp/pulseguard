import { http, HttpResponse } from 'msw'

/**
 * Baseline MSW handlers per endpoint family. Each handler returns a minimal
 * valid shape so specs that don't override get a green path. Per-spec
 * overrides go through `server.use(...)` in `beforeEach`; `resetHandlers()`
 * peels them back in `afterEach`.
 *
 * Contract: specs/054-slice-http-migration-axios/contracts/mock-server.md
 */

const API = '*/api/v1'
const ROOT = '*/api'

export const baselineHandlers = [
  // Auth
  http.post(`${API}/auth/login`, () =>
    HttpResponse.json({ token: 'test-token', user: { email: 'test@example.com' } }),
  ),
  http.get(`${API}/auth/me`, () => HttpResponse.json({ email: 'test@example.com', id: '01H' })),
  http.post(`${API}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
  http.post(`${API}/auth/signup`, () =>
    HttpResponse.json({ token: 'test-token', email: 'new@example.com' }),
  ),
  http.post(`${API}/auth/forgot-password`, () => new HttpResponse(null, { status: 204 })),
  http.post(`${API}/auth/reset-password`, () =>
    HttpResponse.json({ token: 'test-token', email: 'reset@example.com' }),
  ),

  // System (unversioned)
  http.get(`${ROOT}/system/has-accounts`, () => HttpResponse.json({ has_accounts: false })),

  // Onboarding state (path under v1/me/...)
  http.get(`${API}/me/onboarding-state`, () => HttpResponse.json({ status: 'pending' })),
  http.patch(`${API}/me/onboarding-state`, () => HttpResponse.json({ status: 'done' })),

  // Resources (v1 monitors) — list is paginated `{ data, meta }`, single is `{ data }`
  http.get(`${API}/monitors`, () =>
    HttpResponse.json({ data: [], meta: { page: 1, per_page: 100, total: 0 } }),
  ),
  http.get(`${API}/monitors/:id`, ({ params }) =>
    HttpResponse.json({ data: { id: params.id, name: 'res' } }),
  ),
  http.post(`${API}/monitors`, () =>
    HttpResponse.json({ data: { id: '01H', name: 'new' } }, { status: 201 }),
  ),
  http.patch(`${API}/monitors/:id`, ({ params }) => HttpResponse.json({ data: { id: params.id } })),
  http.delete(`${API}/monitors/:id`, () => new HttpResponse(null, { status: 204 })),

  // Incidents (v1) — list is paginated `{ data, meta }`, single is `{ data }`
  http.get(`${API}/incidents`, () =>
    HttpResponse.json({ data: [], meta: { page: 1, per_page: 50, total: 0 } }),
  ),
  http.get(`${API}/incidents/:id`, ({ params }) =>
    HttpResponse.json({ data: { id: params.id, status: 'detected' } }),
  ),
  http.get(`${API}/incidents/:id/updates`, () => HttpResponse.json({ data: [] })),
  http.post(`${API}/incidents/:id/updates`, () =>
    HttpResponse.json({ data: { id: 'u-1', status: 'investigating', message: '' } }, { status: 201 }),
  ),
  http.patch(`${API}/incidents/:id/updates/:updateId`, ({ params }) =>
    HttpResponse.json({ data: { id: params.updateId, status: 'investigating', message: '' } }),
  ),
  http.delete(`${API}/incidents/:id/updates/:updateId`, () => new HttpResponse(null, { status: 204 })),

  // Components
  http.get(`${API}/components`, () => HttpResponse.json([])),
  http.delete(`${API}/components/:id`, () => new HttpResponse(null, { status: 204 })),

  // Credentials
  http.get(`${API}/credentials`, () => HttpResponse.json([])),

  // Notification channels (v1) — list is paginated `{ data, meta }`, single is `{ data }`.
  // GET masks config secrets (password/auth_token/token/account_sid/secret absent).
  http.get(`${API}/notification-channels`, () =>
    HttpResponse.json({ data: [], meta: { page: 1, per_page: 100, total: 0 } }),
  ),
  http.get(`${API}/notification-channels/:id`, ({ params }) =>
    HttpResponse.json({ data: { id: params.id, name: 'chan', type: 'slack', config: {} } }),
  ),
  http.post(`${API}/notification-channels`, () =>
    HttpResponse.json({ data: { id: '01H', name: 'new', type: 'slack', config: {} } }, { status: 201 }),
  ),
  http.patch(`${API}/notification-channels/:id`, ({ params }) =>
    HttpResponse.json({ data: { id: params.id, name: 'chan', type: 'slack', config: {} } }),
  ),
  http.delete(`${API}/notification-channels/:id`, () => new HttpResponse(null, { status: 204 })),
  http.post(`${API}/notification-channels/:id/test`, () =>
    HttpResponse.json({ data: { message: 'ok' } }),
  ),
  http.post(`${API}/notification-channels/test-config`, () =>
    HttpResponse.json({ data: { message: 'ok' } }),
  ),

  // Notification stats (v1) — `{ data: { sent_30d, pending, failed_24h } }`
  http.get(`${API}/notifications/stats`, () =>
    HttpResponse.json({ data: { sent_30d: 0, pending: 0, failed_24h: 0 } }),
  ),

  // Status pages
  http.get(`${API}/status-pages`, () => HttpResponse.json([])),

  // System / edition
  http.get(`${API}/system/edition`, () =>
    HttpResponse.json({ edition: 'community', version: '1.0.0' }),
  ),

  // Tags (v1) — list is paginated `{ data, meta }`
  http.get(`${API}/tags`, () =>
    HttpResponse.json({ data: [], meta: { page: 1, per_page: 100, total: 0 } }),
  ),

  // Maintenance
  http.get(`${API}/maintenances`, () => HttpResponse.json([])),

  // Activity
  http.get(`${API}/monitoring-activities`, () =>
    HttpResponse.json({ activities: [], limit: 50, offset: 0 }),
  ),

  // Stats
  http.get(`${API}/stats`, () => HttpResponse.json({})),
]
