import { z } from 'zod'

/**
 * Spec 059 US3 — discriminated channel schema.
 * Backend dispatch (internal/monitoring/incident_service.go) currently
 * handles `smtp` + `webhook`; `slack` is recognised by the domain enum but
 * not yet routed. Discord and Telegram are spec-idealised — deferred.
 *
 * Spec 086 US3 — the v1 `GET /notification-channels` endpoint MASKS config
 * secrets (they come back absent). On edit the operator therefore sees blank
 * secret fields and may leave them blank to keep the stored value, so the
 * `edit` schema variant makes secrets optional (create stays strict). Blank
 * secrets are stripped from the update payload by `stripBlankSecretKeys` so the
 * backend preserves the stored credential.
 */

/**
 * Config keys the backend masks in GET responses and preserves on update when
 * omitted or blank. Only `password` (SMTP) appears in the current form union;
 * the rest are listed so the strip/optional logic stays correct if SMS or other
 * secret-bearing channels are added.
 */
export const SECRET_CONFIG_KEYS = [
  'password',
  'auth_token',
  'token',
  'account_sid',
  'secret',
] as const

export type SchemaMode = 'create' | 'edit'

const baseFields = {
  name: z.string().trim().min(1, 'Required').max(80, 'At most 80 characters'),
  is_default: z.boolean().default(false),
  is_active: z.boolean().default(true),
}

// SMTP config — password is required on create, optional on edit (leave blank
// to keep the stored secret, which GET masks out).
const smtpChannelSchema = (mode: SchemaMode) =>
  z.object({
    type: z.literal('smtp'),
    ...baseFields,
    config: z.object({
      host: z.string().trim().min(1, 'SMTP host required'),
      port: z.coerce.number().int().min(1).max(65535),
      username: z.string().trim().min(1, 'Username required'),
      password: mode === 'edit' ? z.string().optional() : z.string().min(1, 'Password required'),
      sender: z.string().trim().email('Sender must be an email'),
      recipient: z.string().trim().email('Recipient must be an email'),
    }),
  })

const slackChannelSchema = z.object({
  type: z.literal('slack'),
  ...baseFields,
  config: z.object({
    webhook_url: z
      .string()
      .url('Must be a valid URL')
      .startsWith('https://hooks.slack.com/', 'Must be a Slack incoming-webhook URL'),
    channel: z
      .string()
      .trim()
      .regex(/^#?[a-z0-9-_]+$/, 'Lowercase letters, digits, dashes, underscores'),
    display_name: z.string().max(80).optional(),
  }),
})

const webhookChannelSchema = z.object({
  type: z.literal('webhook'),
  ...baseFields,
  config: z.object({
    url: z.string().url('Must be a valid URL'),
    method: z.enum(['POST', 'PUT']).default('POST'),
    headers: z
      .array(
        z.object({
          name: z.string().trim().min(1).max(80),
          value: z.string().min(1).max(1024),
        }),
      )
      .max(20, 'At most 20 headers')
      .default([]),
  }),
})

/**
 * Build the discriminated channel schema for the given mode. `create` keeps
 * secret fields required; `edit` relaxes them (masked secrets come back blank).
 */
export const channelSchemaFor = (mode: SchemaMode = 'create') =>
  z.discriminatedUnion('type', [smtpChannelSchema(mode), slackChannelSchema, webhookChannelSchema])

// Default (create) schema — kept as a named export for the strict-validation path.
export const notificationChannelSchema = channelSchemaFor('create')

export type NotificationChannelInput = z.infer<typeof notificationChannelSchema>
export type ChannelType = NotificationChannelInput['type']

/**
 * Remove blank/absent secret keys from a channel config before sending an
 * update. A key left blank means "keep the stored secret" — the backend
 * preserves omitted secrets. A newly typed secret is a non-empty string and is
 * kept as-is (overwrites the stored value).
 */
export function stripBlankSecretKeys<T extends Record<string, unknown>>(config: T): T {
  const out: Record<string, unknown> = { ...config }
  for (const key of SECRET_CONFIG_KEYS) {
    const v = out[key]
    if (v === undefined || v === null || v === '') delete out[key]
  }
  return out as T
}

export const CHANNEL_TYPES: { value: ChannelType; label: string; icon: string }[] = [
  { value: 'smtp', label: 'Email (SMTP)', icon: 'i-lucide-mail' },
  { value: 'slack', label: 'Slack', icon: 'i-lucide-message-square' },
  { value: 'webhook', label: 'Webhook', icon: 'i-lucide-webhook' },
]

export function emptyConfigForType(type: ChannelType): NotificationChannelInput['config'] {
  switch (type) {
    case 'smtp':
      return { host: '', port: 587, username: '', password: '', sender: '', recipient: '' }
    case 'slack':
      return { webhook_url: '', channel: '', display_name: '' }
    case 'webhook':
      return { url: '', method: 'POST', headers: [] }
  }
}
