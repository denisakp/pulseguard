/**
 * After-response interceptor for the shared Ky client.
 *
 * Concerns owned here (side-effects + toasts), kept out of `normalizeError`
 * (which stays a pure unknown→typed transform):
 *  - Success toast on 2xx mutating methods (POST/PUT/PATCH/DELETE)
 *  - Error toast on 4xx/5xx with status-specific wording
 *  - 401 single-flight: clear stored credentials + redirect to /login,
 *    exactly once per concurrent burst
 *
 * Per-call opt-outs via request headers:
 *  - `x-skip-success-toast: '1'`
 *  - `x-skip-error-toast: '1'`
 *  - `x-success-message: '<custom message>'`
 */

const TOKEN_KEY = 'ogoune_auth_token'
const USER_EMAIL_KEY = 'ogoune_user_email'
const USER_ID_KEY = 'ogoune_user_id'

const SUCCESS_TOAST_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

type ToastApi = {
  add: (input: {
    title: string
    description?: string
    color?: 'success' | 'error' | 'info' | 'warning' | string
  }) => unknown
}

async function resolveToast(): Promise<ToastApi | null> {
  try {
    const mod = await import('@nuxt/ui/composables/useToast')
    return mod.useToast() as ToastApi
  } catch {
    return null
  }
}

function defaultSuccessMessage(method: string): string {
  switch (method.toUpperCase()) {
    case 'POST':
      return 'Created successfully'
    case 'PUT':
    case 'PATCH':
      return 'Updated successfully'
    case 'DELETE':
      return 'Deleted successfully'
    default:
      return 'Operation successful'
  }
}

function errorTitleFor(status: number, message: string | null): string {
  const msg = message ?? 'An error occurred'
  if (status >= 400 && status < 500) {
    switch (status) {
      case 400:
        return `Bad request: ${msg}`
      case 401:
        return 'Unauthorized. Please log in again.'
      case 403:
        return 'Access forbidden.'
      case 404:
        return 'Resource not found.'
      case 409:
        return `Conflict: ${msg}`
      case 422:
        return `Validation failed: ${msg}`
      default:
        return `Client error: ${msg}`
    }
  }
  if (status >= 500) {
    switch (status) {
      case 502:
        return 'Service temporarily unavailable.'
      case 503:
        return 'Service under maintenance.'
      case 504:
        return 'Request timeout.'
      default:
        return `Server error: ${msg}`
    }
  }
  return `Error: ${msg}`
}

// ---- 401 single-flight (clarification Q1) ----
let redirecting = false

function clearStoredCredentials(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_EMAIL_KEY)
  localStorage.removeItem(USER_ID_KEY)
}

async function handle401SingleFlight(): Promise<void> {
  if (redirecting) return
  redirecting = true

  // Guard against an infinite full-reload loop. `redirecting` is module state
  // and resets on every full page load, so a 401 raised *while already on*
  // /login would otherwise do `window.location.href = '/login'` → full reload →
  // same request → same 401 → reload again, forever. When we're already on the
  // login page, just clear stale credentials and stop — the app is right where
  // the user needs to be, and the route guard handles the rest.
  if (typeof window !== 'undefined' && window.location.pathname === '/login') {
    clearStoredCredentials()
    redirecting = false
    return
  }

  const toast = await resolveToast()
  toast?.add({ title: 'Unauthorized. Please log in again.', color: 'error' })
  clearStoredCredentials()
  if (typeof window !== 'undefined') {
    window.location.href = '/login'
  }
}

async function maybeSuccessToast(request: Request): Promise<void> {
  const method = request.method.toUpperCase()
  if (!SUCCESS_TOAST_METHODS.has(method)) return
  if (request.headers.get('x-skip-success-toast') === '1') return
  const custom = request.headers.get('x-success-message')
  const toast = await resolveToast()
  toast?.add({
    title: custom ?? defaultSuccessMessage(method),
    color: 'success',
  })
}

async function maybeErrorToast(request: Request, response: Response): Promise<void> {
  if (request.headers.get('x-skip-error-toast') === '1') return
  // Note: we intentionally DO NOT read the response body here. The body stream
  // is consumed by `normalizeError` in `client.ts` to extract typed error
  // details (fieldErrors on ValidationError, etc.). Reading + cloning here
  // can race the downstream consumer. The toast title uses the HTTP status
  // alone — server messages flow through the typed ApiError if callers want
  // to surface them via a follow-up `useToast` call.
  console.error('API Error:', request.method, request.url, response.status)
  const toast = await resolveToast()
  toast?.add({
    title: errorTitleFor(response.status, null),
    color: 'error',
  })
}

export async function errorInterceptor(request: Request, response: Response): Promise<Response> {
  if (response.ok) {
    await maybeSuccessToast(request)
    return response
  }

  if (response.status === 401) {
    await handle401SingleFlight()
  } else {
    await maybeErrorToast(request, response)
  }

  // Let Ky throw HTTPError with `data` pre-populated. `normalizeError`
  // (in client.ts) reads `error.data` to extract typed-error details.
  return response
}
