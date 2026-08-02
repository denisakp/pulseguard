import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

/**
 * Isolation contract — spec 055 clarification Q2.
 *
 * `UStatusBadge`, `UUptimeBar`, `UUptimeCalendar` ship into the public status
 * bundle (Slice 4) where Pinia/router/auth context does not exist. They MUST
 * be strictly presentational: no `useAuthStore`, no `useRouter`, no
 * `useLicence`, no `useColorMode`, no `useToast`, no `useConfirm`.
 *
 * This spec greps each component's source and fails if any forbidden import
 * appears. Cheaper than tracking down a "white screen" symptom in Slice 4.
 */
const FORBIDDEN = /from ['"]@\/(stores|composables|router)\b/
const ISOLATED = ['UStatusBadge.vue', 'UUptimeBar.vue', 'UUptimeCalendar.vue']

describe('Shared component isolation', () => {
  it.each(ISOLATED)('%s does not import from @/stores, @/composables, or @/router', (file) => {
    const path = resolve(__dirname, '..', file)
    const src = readFileSync(path, 'utf8')
    const match = src.match(FORBIDDEN)
    expect(match, `${file} imports a forbidden contextual composable: ${match?.[0]}`).toBeNull()
  })
})
