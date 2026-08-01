import { z } from 'zod'

/**
 * Register-host form schema (spec 081). The only user-supplied field at
 * registration time is the display name; everything else (OS, agent version,
 * metrics) is reported by the agent after it connects.
 */
export const hostSchema = z.object({
  name: z.string().trim().min(1, 'Required').max(200, 'At most 200 characters'),
})

export type HostInput = z.infer<typeof hostSchema>
