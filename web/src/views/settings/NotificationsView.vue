<script setup lang="ts">
/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — spec 059 polish debt: NuxtUI v4 template-emit types
/**
 * Notification channels — design fidelity v2.
 * Page-level h1 + tagline + info alert + 4 KPI cards + table with Default toggle.
 */
import { computed, h, onMounted, ref, resolveComponent } from 'vue'
import type { TableColumn } from '@nuxt/ui'

import {
  fetchChannels,
  createChannel,
  updateChannel,
  deleteChannel,
  setDefault,
} from '@/services/notificationChannelService'
import { fetchNotificationStats } from '@/services/notificationStatsService'
import type { NotificationChannel, CreateNotificationChannel } from '@/types'
import { stripBlankSecretKeys } from '@/schemas/notification-channel.schema'
import type { NotificationChannelInput } from '@/schemas/notification-channel.schema'
import { useConfirm } from '@/composables/useConfirm'
import ChannelModal from '@/components/settings/notifications/ChannelModal.vue'

const channels = ref<NotificationChannel[]>([])
const loading = ref(true)
const modalOpen = ref(false)
const editing = ref<NotificationChannel | null>(null)

const notifStats = ref<{
  sent_30d: number
  pending: number
  failed_24h: number
} | null>(null)

function lastSentLabel(iso: string | null | undefined): string {
  if (!iso) return '—'
  try {
    const ms = Date.now() - new Date(iso).getTime()
    if (ms < 60_000) return 'just now'
    if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m ago`
    if (ms < 86_400_000) return `${Math.round(ms / 3_600_000)}h ago`
    return `${Math.round(ms / 86_400_000)}d ago`
  } catch {
    return '—'
  }
}

const stats = computed(() => {
  const total = channels.value.length
  const defaultCount = channels.value.filter((c) => c.enabled_by_default).length
  const scopedCount = total - defaultCount
  const fmt = (v: number | undefined | null) => (typeof v === 'number' ? v.toLocaleString() : '—')
  return [
    {
      key: 'channels',
      label: 'CHANNELS',
      value: String(total),
      meta: total > 0 ? `${defaultCount} default, ${scopedCount} scoped` : 'no channels yet',
    },
    {
      key: 'sent',
      label: 'ALERTS SENT (30d)',
      value: fmt(notifStats.value?.sent_30d),
      meta: '',
    },
    {
      key: 'pending',
      label: 'PENDING RETRY',
      value: fmt(notifStats.value?.pending),
      meta: '',
    },
    {
      key: 'failed',
      label: 'FAILED (24h)',
      value: fmt(notifStats.value?.failed_24h),
      meta: '',
    },
  ]
})

const TYPE_META: Record<string, { label: string; icon: string; iconBg: string; badge: string }> = {
  smtp: { label: 'SMTP', icon: 'i-lucide-mail', iconBg: 'bg-info/10 text-info', badge: 'info' },
  slack: {
    label: 'Slack',
    icon: 'i-lucide-message-square',
    iconBg: 'bg-primary/10 text-primary',
    badge: 'primary',
  },
  webhook: {
    label: 'Webhook',
    icon: 'i-lucide-webhook',
    iconBg: 'bg-success/10 text-success',
    badge: 'success',
  },
  discord: {
    label: 'Discord',
    icon: 'i-lucide-message-circle',
    iconBg: 'bg-primary/10 text-primary',
    badge: 'primary',
  },
  teams: { label: 'Teams', icon: 'i-lucide-users', iconBg: 'bg-info/10 text-info', badge: 'info' },
}

function typeMeta(t: string) {
  return TYPE_META[t] ?? TYPE_META.webhook
}

function recipientPreview(c: NotificationChannel): string {
  const cfg = (c.config as unknown as Record<string, unknown>) ?? {}
  if (c.type === 'smtp') {
    if (Array.isArray(cfg.recipients) && cfg.recipients.length > 0) {
      const first = String(cfg.recipients[0])
      const extra = cfg.recipients.length - 1
      return extra > 0 ? `${first} +${extra}` : first
    }
    if (typeof cfg.recipient === 'string') return cfg.recipient
    if (typeof cfg.sender === 'string') return cfg.sender
  }
  if (c.type === 'slack') {
    if (typeof cfg.channel === 'string')
      return cfg.channel.startsWith('#') ? cfg.channel : `#${cfg.channel}`
    if (typeof cfg.webhook_url === 'string') return truncateUrl(cfg.webhook_url)
  }
  if (c.type === 'discord') {
    if (typeof cfg.channel === 'string')
      return cfg.channel.startsWith('#') ? cfg.channel : `#${cfg.channel}`
    if (typeof cfg.webhook_url === 'string') return truncateUrl(cfg.webhook_url)
  }
  if (c.type === 'teams') {
    if (typeof cfg.channel === 'string') return String(cfg.channel)
    if (typeof cfg.webhook_url === 'string') return truncateUrl(cfg.webhook_url)
  }
  if (c.type === 'webhook' && typeof cfg.url === 'string') return truncateUrl(cfg.url)
  return ''
}

function truncateUrl(u: string): string {
  if (u.length <= 32) return u
  return u.slice(0, 30) + '…'
}

function lastSentForChannel(c: NotificationChannel): string {
  return lastSentLabel(c.last_sent_at ?? null)
}

function failuresClass(n: number | undefined): string {
  return n && n > 0 ? 'text-red-600 font-semibold' : 'text-muted'
}

async function reload() {
  loading.value = true
  try {
    const [list, s] = await Promise.all([
      fetchChannels(),
      fetchNotificationStats().catch(() => null),
    ])
    channels.value = list
    notifStats.value = s
  } catch {
    channels.value = []
    notifStats.value = null
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  modalOpen.value = true
}

function openEdit(c: NotificationChannel) {
  editing.value = c
  modalOpen.value = true
}

async function onSubmit(payload: NotificationChannelInput) {
  const type = payload.type as CreateNotificationChannel['type']
  if (editing.value) {
    // GET masks config secrets, so on edit they come back blank. Drop blank
    // secret keys from the update payload — the backend preserves omitted
    // secrets; a newly typed secret is non-empty and overwrites the stored one.
    const config = stripBlankSecretKeys(
      payload.config as unknown as Record<string, unknown>,
    ) as unknown as CreateNotificationChannel['config']
    await updateChannel(editing.value.id, {
      name: payload.name,
      type,
      config,
      enabled_by_default: payload.is_default,
    })
  } else {
    await createChannel({
      name: payload.name,
      type,
      config: payload.config as unknown as CreateNotificationChannel['config'],
      enabled_by_default: payload.is_default,
    })
  }
  modalOpen.value = false
  await reload()
}

async function onDelete(c: NotificationChannel) {
  const ok = await useConfirm({
    kind: 'destructive',
    title: `Delete ${c.name}?`,
    body: 'Alerts previously routed here will no longer fire.',
    ctaLabel: 'Delete channel',
  })
  if (!ok) return
  await deleteChannel(c.id)
  channels.value = channels.value.filter((x) => x.id !== c.id)
}

async function onToggleDefault(c: NotificationChannel) {
  if (c.enabled_by_default) return
  const previous = channels.value.map((x) => ({ ...x }))
  channels.value = channels.value.map((x) => ({
    ...x,
    enabled_by_default: x.id === c.id,
  }))
  try {
    await setDefault(c.id)
  } catch {
    channels.value = previous
  }
}

const initialForModal = computed(() => {
  if (!editing.value) return undefined
  return {
    id: editing.value.id,
    name: editing.value.name,
    type: editing.value.type as NotificationChannelInput['type'],
    is_default: editing.value.enabled_by_default,
    is_active: true,
    config: editing.value.config as unknown as NotificationChannelInput['config'],
  }
})

onMounted(reload)

const columns: TableColumn<NotificationChannel>[] = [
  {
    id: 'channel',
    header: 'Channel',
    cell: ({ row }) => {
      const c = row.original
      const tm = typeMeta(c.type)
      const recipient = recipientPreview(c)
      return h('div', { class: 'flex items-center gap-3' }, [
        h(
          'div',
          {
            class: ['size-9 shrink-0 rounded-md flex items-center justify-center', tm.iconBg],
          },
          [h(resolveComponent('UIcon'), { name: tm.icon, class: 'size-4' })],
        ),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-default' }, c.name),
          recipient ? h('p', { class: 'text-xs text-muted font-mono truncate' }, recipient) : null,
        ]),
      ])
    },
  },
  {
    id: 'type',
    header: 'Type',
    cell: ({ row }) => {
      const tm = typeMeta(row.original.type)
      return h(
        resolveComponent('UBadge'),
        { color: tm.badge, variant: 'subtle', size: 'sm' },
        () => tm.label,
      )
    },
  },
  {
    id: 'status',
    header: 'Status',
    cell: () =>
      h(resolveComponent('UBadge'), { color: 'success', variant: 'subtle', size: 'sm' }, () => [
        h('span', { class: 'inline-block size-1.5 rounded-full mr-1 bg-success' }),
        'Verified',
      ]),
  },
  {
    id: 'default',
    header: 'Default',
    cell: ({ row }) =>
      h(resolveComponent('USwitch'), {
        modelValue: row.original.enabled_by_default,
        onClick: () => onToggleDefault(row.original),
      }),
  },
  {
    id: 'last_sent',
    header: 'Last sent',
    cell: ({ row }) => h('span', { class: 'text-muted' }, lastSentForChannel(row.original)),
  },
  {
    id: 'failures',
    header: 'Failures (24h)',
    cell: ({ row }) =>
      h(
        'span',
        { class: failuresClass(row.original.failures_24h) },
        String(row.original.failures_24h ?? 0),
      ),
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const c = row.original
      return h(
        resolveComponent('UDropdownMenu'),
        {
          items: [
            { label: 'Edit', icon: 'i-lucide-pencil', onSelect: () => openEdit(c) },
            { label: 'Delete', icon: 'i-lucide-trash-2', onSelect: () => onDelete(c) },
          ],
        },
        () =>
          h(resolveComponent('UButton'), {
            variant: 'ghost',
            size: 'xs',
            icon: 'i-lucide-more-horizontal',
          }),
      )
    },
  },
]

defineExpose({ channels, stats, openCreate, openEdit, onSubmit, onToggleDefault, onDelete, columns })
</script>

<template>
  <div class="space-y-6">
    <header class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold text-default">Notification Channels</h1>
        <p class="text-sm text-muted">Where Ogoune sends alerts when incidents happen</p>
      </div>
      <UButton color="primary" icon="i-lucide-plus" @click="openCreate">Add Channel</UButton>
    </header>

    <div class="rounded-xl border border-info/30 bg-info/5 px-4 py-3 flex items-start gap-3">
      <UIcon name="i-lucide-info" class="size-5 text-info shrink-0 mt-0.5" />
      <div class="text-sm">
        <p class="font-semibold text-default">One default channel can cover all monitors</p>
        <p class="text-muted">
          Toggle « Default » on any channel — it'll receive alerts for every monitor unless
          overridden at the resource level.
        </p>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div
        v-for="s in stats"
        :key="s.key"
        class="rounded-xl border border-default bg-default p-4 space-y-1"
      >
        <p class="text-[11px] font-medium text-muted uppercase tracking-wide">{{ s.label }}</p>
        <p class="text-2xl font-bold text-default leading-tight">{{ s.value }}</p>
        <p class="text-xs text-muted">{{ s.meta }}</p>
      </div>
    </div>

    <USkeleton v-if="loading" class="h-64 w-full" />

    <UEmpty
      v-else-if="channels.length === 0"
      icon="i-lucide-bell-off"
      title="No notification channels yet"
      description="Add one to start receiving incident alerts."
    >
      <template #actions>
        <UButton color="primary" @click="openCreate">Add your first channel</UButton>
      </template>
    </UEmpty>

    <div v-else class="overflow-hidden rounded-xl border border-default bg-default">
      <UTable :data="channels" :columns="columns" />
    </div>

    <ChannelModal
      v-model:open="modalOpen"
      :initial="initialForModal as unknown as never"
      @submit="onSubmit"
    />
  </div>
</template>
