import { z } from 'zod'

// Port scanner form schema. Mirrors backend caps:
// max 100 ports, timeout 100–2000ms (default 1000).

export const portPresets = ['common', 'web', 'db', 'custom'] as const

export const portPresetValues: Record<(typeof portPresets)[number], number[]> = {
  common: [21, 22, 25, 53, 80, 110, 143, 443, 3306, 5432],
  web: [80, 443, 8080, 8443],
  db: [3306, 5432, 6379, 27017, 1521, 1433],
  custom: [],
}

const target = z
  .string()
  .trim()
  .min(1, 'Required')
  .regex(/^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*$/, 'Enter a valid host')

export const portScanSchema = z.object({
  target,
  preset: z.enum(portPresets),
  ports: z
    .array(z.number().int().min(1, '1–65535').max(65535, '1–65535'))
    .min(1, 'At least one port')
    .max(100, 'At most 100 ports per scan'),
  timeout_ms: z.number().int().min(100, 'Min 100ms').max(2000, 'Max 2000ms'),
})

export type PortScanInput = z.infer<typeof portScanSchema>
