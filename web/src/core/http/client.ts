import ky, { HTTPError, TimeoutError, type KyInstance } from 'ky'
import {
  ConflictError,
  ForbiddenError,
  NetworkError,
  NotFoundError,
  ServerError,
  UnauthorizedError,
  ValidationError,
} from '@/core/errors'
import { errorInterceptor } from './erreror-interceptor'

const TOKEN_KEY = 'ogoune_auth_token'

async function normalizeError(error: unknown): Promise<never> {
  if (error instanceof HTTPError) {
    const { response } = error
    const traceId = response.headers.get('x-request-id') ?? undefined

    // Ky v2 pre-parses the body into `error.data` before `beforeError` hooks
    // run; the response stream is therefore already consumed. Use `error.data`.
    const body: unknown = (error as HTTPError).data ?? null

    // Prefer the human-readable message. v1 uses RFC7807 problem+json, where the
    // message lives in `detail`; the legacy root API uses `message`. Falling back
    // to statusText produced the generic "Bad Request" that hid real reasons.
    const b = body as { message?: string; detail?: string; code?: string } | null
    const message = b?.detail ?? b?.message ?? response.statusText
    const code = b?.code

    const retryAfterRaw = response.headers.get('retry-after')
    const retryAfterSec = retryAfterRaw
      ? Number.isFinite(Number(retryAfterRaw))
        ? Number(retryAfterRaw)
        : undefined
      : undefined

    const attach = <E extends { code?: string; retryAfterSec?: number }>(err: E): E => {
      if (code) err.code = code
      if (retryAfterSec !== undefined) err.retryAfterSec = retryAfterSec
      return err
    }

    switch (response.status) {
      case 400:
      case 422: {
        const fieldErrors =
          (body as { fieldErrors?: Record<string, string[]> } | null)?.fieldErrors ?? {}
        throw attach(new ValidationError(message, fieldErrors, traceId))
      }
      case 401:
        throw attach(new UnauthorizedError(message, traceId))
      case 403:
        throw attach(new ForbiddenError(message, traceId))
      case 404:
        throw attach(new NotFoundError(message, traceId))
      case 409:
        throw attach(new ConflictError(message, traceId))
      default:
        throw attach(new ServerError(message, response.status, traceId))
    }
  }

  if (error instanceof TimeoutError) throw new NetworkError('Request timed out')
  if (error instanceof TypeError) throw new NetworkError('Network request failed')

  throw error
}

function buildHooks(getToken?: () => string | null) {
  return {
    beforeRequest: [
      ({ request }: { request: Request }) => {
        const token = getToken?.()
        if (token) request.headers.set('Authorization', `Bearer ${token}`)
      },
    ],
    afterResponse: [
      async ({ request, response }: { request: Request; response: Response }) => {
        return await errorInterceptor(request, response)
      },
    ],
    beforeError: [({ error }: { error: Error }) => error],
  }
}

const BASE_CONFIG = {
  timeout: 15_000,
  retry: {
    limit: 2,
    methods: ['GET', 'PUT', 'DELETE'] as string[],
    statusCodes: [408, 429, 500, 502, 503, 504],
    backoffLimit: 5_000,
  },
  headers: {
    'Content-Type': 'application/json',
  },
  prefix: import.meta.env.VITE_API_BASE_URL,
}

/**
 * Unauthenticated client. Use for guest flows (login, signup, public status pages).
 */
export const http: KyInstance = ky.create({
  ...BASE_CONFIG,
  hooks: buildHooks(),
})

/**
 * Factory: returns a Ky instance configured with a Bearer-token callback.
 * The callback is invoked per-request, so token rotation works without
 * re-instantiating the client.
 */
export function createAuthenticatedClient(options: { getToken: () => string | null }): KyInstance {
  return ky.create({
    ...BASE_CONFIG,
    hooks: buildHooks(() => options.getToken()),
  })
}

/**
 * Singleton authenticated client, lazy-initialized on first call.
 * Token is read directly from localStorage on every request (no Pinia
 * coupling — avoids circular imports auth↔core/http and works in tests
 * before Pinia is mounted).
 */
let _authClient: KyInstance | null = null
export function getAuthenticatedClient(): KyInstance {
  if (!_authClient) {
    _authClient = createAuthenticatedClient({
      getToken: () => localStorage.getItem(TOKEN_KEY),
    })
  }
  return _authClient
}

let _publicClient: KyInstance | null = null
/** Public ky instance — same base config, no Authorization header. */
export function getPublicClient(): KyInstance {
  if (!_publicClient) {
    _publicClient = ky.create({ ...BASE_CONFIG, hooks: buildHooks(() => null) })
  }
  return _publicClient
}

/**
 * Orthogonal helper: takes any KyInstance + url + options, returns the
 * parsed JSON body typed as T. Handles 204/205 → undefined and routes
 * every thrown error through `normalizeError` (typed ApiError subclasses).
 */
export async function request<T>(
  client: KyInstance,
  url: string,
  options?: Parameters<KyInstance>[1],
): Promise<T> {
  try {
    const res = await client(url, options)
    if (res.status === 204 || res.status === 205) return undefined as T
    return await res.json<T>()
  } catch (error) {
    return normalizeError(error)
  }
}
