<script setup lang="ts">
/**
 * Hosts list page (spec 081, P1). Trouble-first table of the host fleet with a
 * register/onboarding flow and per-row rotate-credential / delete actions.
 * Read/write affordances are always shown; a backend 403 surfaces via the
 * shared error toast (there is no session read-only gate in this SPA).
 */
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import { useHosts, type EnrichedHost } from '@/composables/useHosts'
import { useConfirm } from '@/composables/useConfirm'
import { rotateCredential, deleteHost } from '@/services/hostsService'
import { timeAgo } from '@/libs/date-time.helper'
import HostStatusBadge from '@/components/hosts/HostStatusBadge.vue'
import HostCredentialReveal from '@/components/hosts/HostCredentialReveal.vue'
import RegisterHostModal from '@/components/hosts/RegisterHostModal.vue'
import type { Host } from '@/types'

interface RowAction {
  label: string
  icon: string
}

const { hosts, loading, error, count, fetch, startPolling, stopPolling } = useHosts()

const showRegister = ref(false)

// Reveal credential after a rotate — held locally only, cleared on dismiss.
const rotated = ref<{ credential: string; prefix: string } | null>(null)

function pct(v: number | null, online: boolean): ReturnType<typeof h> {
  if (v === null || !online) return h('span', { class: 'text-muted' }, '—')
  return h('span', { class: 'text-sm tabular-nums' }, `${Math.round(v)}%`)
}

const rotateAction: RowAction = { label: 'Rotate credential', icon: 'i-lucide-key-round' }
const deleteAction: RowAction = { label: 'Delete', icon: 'i-lucide-trash-2' }
const rowActions: RowAction[] = [rotateAction, deleteAction]

const columns: TableColumn<EnrichedHost>[] = [
  {
    id: 'name',
    accessorKey: 'name',
    header: 'Host',
    cell: ({ row }) =>
      h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'font-medium' }, row.original.name),
        h(HostStatusBadge, {
          online: row.original.online,
          lastSeenAt: row.original.lastSeenAt,
        }),
      ]),
  },
  {
    id: 'cpu',
    header: 'CPU %',
    cell: ({ row }) => pct(row.original.lastCpuPct, row.original.online),
  },
  {
    id: 'mem',
    header: 'RAM %',
    cell: ({ row }) => pct(row.original.lastMemPct, row.original.online),
  },
  {
    id: 'disk',
    header: 'Disk %',
    cell: ({ row }) => pct(row.original.lastDiskPct, row.original.online),
  },
  {
    id: 'services',
    header: 'Services',
    cell: ({ row }) => h('span', { class: 'text-sm tabular-nums' }, String(row.original.serviceCount)),
  },
  {
    id: 'last_seen',
    header: 'Last seen',
    cell: ({ row }) => {
      const v = row.original.lastSeenAt
      return v
        ? h('span', { class: 'text-xs text-muted' }, timeAgo(v))
        : h('span', { class: 'text-muted' }, '—')
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) =>
      h(
        'div',
        { class: 'flex gap-1 justify-end' },
        rowActions.map((a) =>
          h(
            resolveButton(),
            {
              icon: a.icon,
              color: 'neutral',
              variant: 'ghost',
              size: 'xs',
              'aria-label': a.label,
              onClick: () => onAction({ action: a, row: row.original }),
            },
            () => a.label,
          ),
        ),
      ),
  },
]

// Resolve UButton lazily so the render functions don't crash in vitest stubs.
function resolveButton() {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (globalThis as any).UButton ?? 'UButton'
}

async function onAction(p: { action: RowAction; row: Host }) {
  const row = p.row
  if (p.action.label === rotateAction.label) {
    const ok = await useConfirm({
      kind: 'default',
      title: 'Rotate credential?',
      body: `${row.name}'s current credential stops working immediately. You'll get a new one to reinstall.`,
      ctaLabel: 'Rotate',
    })
    if (!ok) return
    const result = await rotateCredential(row.id)
    rotated.value = { credential: result.credential, prefix: result.prefix }
  } else if (p.action.label === deleteAction.label) {
    const ok = await useConfirm({
      kind: 'destructive',
      title: 'Delete host?',
      body: `${row.name} and its metric history will be removed. Linked monitors are unlinked.`,
      ctaLabel: 'Delete',
    })
    if (!ok) return
    await deleteHost(row.id)
    await fetch()
  }
}

const isEmpty = computed(() => !loading.value && count.value === 0)

function openRegister() {
  showRegister.value = true
}

async function onRegistered() {
  await fetch()
}

function onRotatedClose() {
  rotated.value = null
}

onMounted(() => {
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})

defineExpose({ columns, onAction, isEmpty, showRegister, rotated })
</script>

<template>
  <div class="p-6 bg-default text-default min-h-screen">
    <div class="flex items-start justify-between mb-6">
      <div>
        <h1 class="text-2xl font-semibold">Hosts</h1>
        <p class="text-sm text-muted mt-1">
          Monitored servers reporting metrics through the Ogoune agent
        </p>
      </div>
      <UButton color="primary" icon="i-lucide-plus" @click="openRegister">Register host</UButton>
    </div>

    <UAlert v-if="error" color="error" :title="error" class="mb-4" />

    <div v-if="isEmpty" class="empty-state">
      <UEmpty
        icon="i-lucide-server"
        title="No hosts yet"
        description="Register a host to install the Ogoune agent and start collecting CPU, memory, disk, and network metrics."
      >
        <template #actions>
          <UButton color="primary" icon="i-lucide-plus" @click="openRegister">Register host</UButton>
        </template>
      </UEmpty>
    </div>

    <UTable
      v-else
      :columns="columns"
      :data="hosts"
      :loading="loading"
      empty="No hosts yet"
    />

    <RegisterHostModal
      :open="showRegister"
      @registered="onRegistered"
      @close="showRegister = false"
    />

    <UModal
      v-if="rotated"
      :open="true"
      title="New credential"
      :ui="{ content: 'sm:max-w-lg' }"
      @update:open="onRotatedClose"
    >
      <template #body>
        <HostCredentialReveal
          :credential="rotated.credential"
          :prefix="rotated.prefix"
          @close="onRotatedClose"
        />
      </template>
    </UModal>
  </div>
</template>
