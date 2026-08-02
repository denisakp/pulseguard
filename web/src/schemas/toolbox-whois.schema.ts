import { z } from 'zod'

// WHOIS form schema.

const domain = z
  .string()
  .trim()
  .min(1, 'Required')
  .regex(/^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*$/, 'Enter a valid domain')

export const whoisSchema = z.object({ domain })

export type WhoisInput = z.infer<typeof whoisSchema>
