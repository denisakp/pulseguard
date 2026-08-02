import { z } from 'zod'

// DNS lookup form schema.

export const dnsRecordTypes = ['A', 'AAAA', 'MX', 'NS', 'TXT', 'CNAME'] as const
export const dnsResolvers = ['cloudflare', 'google', 'quad9', 'custom'] as const

// Hostname (RFC-1123-ish): labels of letters/digits/hyphens separated by dots.
const hostname = z
  .string()
  .trim()
  .min(1, 'Required')
  .regex(/^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*$/, 'Enter a valid domain')

export const dnsLookupSchema = z
  .object({
    domain: hostname,
    record_types: z.array(z.enum(dnsRecordTypes)).min(1, 'Pick at least one record type'),
    resolver: z.enum(dnsResolvers),
    custom_resolver: z.string().trim().optional(),
  })
  .refine(
    (v) => v.resolver !== 'custom' || (v.custom_resolver?.length ?? 0) > 0,
    { message: 'Custom resolver IP is required', path: ['custom_resolver'] },
  )

export type DnsLookupInput = z.infer<typeof dnsLookupSchema>
