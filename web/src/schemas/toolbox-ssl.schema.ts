import { z } from 'zod'

// SSL checker form schema.

const domain = z
  .string()
  .trim()
  .min(1, 'Required')
  .regex(/^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*$/, 'Enter a valid domain')

export const sslCheckSchema = z.object({
  domain,
  port: z.number().int().min(1, '1–65535').max(65535, '1–65535'),
})

export type SslCheckInput = z.infer<typeof sslCheckSchema>
