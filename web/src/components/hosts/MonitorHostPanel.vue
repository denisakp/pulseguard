<script setup lang="ts">
/**
 * MonitorHostPanel — host context shown on a monitor's page.
 *
 * Self-fetches the host (`getHost`) and the OTHER monitors linked to the same
 * host (from the resources service, filtered by `host_id`, excluding the current
 * monitor when `monitorId` is provided). Renders a compact panel: status badge,
 * current CPU/RAM/Disk %, the sibling-services list, and a "View host" link.
 *
 * The integrator mounts this only when a monitor is linked, so `hostId` is
 * assumed valid. A failed fetch degrades gracefully (host-not-found notice).
 * Frontend-only.
 */
import { computed, onMounted, ref } from 'vue'
import HostStatusBadge from '@/components/hosts/HostStatusBadge.vue'
import { getHost, listMonitors } from '@/services/hostsService'
import type { Host, MonitorSummary } from '@/types'

interface Props {
  hostId: string
  /** Current monitor id — excluded from the "other services" list. */
  monitorId?: string
}
const props = defineProps<Props>()

const host = ref<Host | null>(null)
const loading = ref(true)
const failed = ref(false)
const otherServices = ref<MonitorSummary[]>([])

const metrics = computed(() => [
  { label: 'CPU', icon: 'i-lucide-cpu', value: host.value?.lastCpuPct ?? null },
  { label: 'RAM', icon: 'i-lucide-memory-stick', value: host.value?.lastMemPct ?? null },
  { label: 'Disk', icon: 'i-lucide-hard-drive', value: host.value?.lastDiskPct ?? null },
])

function fmtPct(v: number | null): string {
  return v === null || v === undefined ? '—' : `${Math.round(v)}%`
}

async function load() {
  loading.value = true
  failed.value = false
  try {
    host.value = await getHost(props.hostId)
  } catch {
    host.value = null
    failed.value = true
    loading.value = false
    return
  }

  try {
    const all = await listMonitors()
    otherServices.value = all.filter(
      (m) => m.hostId === props.hostId && m.id !== props.monitorId,
    )
  } catch {
    otherServices.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)

defineExpose({ host, otherServices, failed, loading, load })
</script>

<template>
  <div
    class="rounded-lg border border-default bg-default p-4"
    data-testid="monitor-host-panel"
  >
    <div v-if="loading" class="flex items-center gap-2 text-sm text-muted">
      <UIcon name="i-lucide-loader-circle" class="animate-spin" />
      Loading host…
    </div>

    <div
      v-else-if="failed || !host"
      class="flex items-center gap-2 text-sm text-muted"
      data-testid="host-not-found"
    >
      <UIcon name="i-lucide-server-off" />
      Host information is unavailable.
    </div>

    <div v-else class="space-y-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <UIcon name="i-lucide-server" class="text-muted shrink-0" />
            <span class="font-medium text-default truncate" data-testid="host-name">
              {{ host.name }}
            </span>
          </div>
          <div class="mt-1">
            <HostStatusBadge :online="host.online" :last-seen-at="host.lastSeenAt" />
          </div>
        </div>
        <UButton
          :to="`/hosts/${host.id}`"
          size="xs"
          variant="ghost"
          color="neutral"
          icon="i-lucide-external-link"
          data-testid="view-host-link"
        >
          View host
        </UButton>
      </div>

      <div class="grid grid-cols-3 gap-2">
        <div
          v-for="m in metrics"
          :key="m.label"
          class="rounded-md border border-default bg-elevated/40 px-2 py-1.5 text-center"
        >
          <div class="flex items-center justify-center gap-1 text-xs text-muted">
            <UIcon :name="m.icon" class="size-3.5" />
            {{ m.label }}
          </div>
          <div class="text-sm font-semibold text-default">{{ fmtPct(m.value) }}</div>
        </div>
      </div>

      <div>
        <h4 class="text-xs font-semibold uppercase tracking-wide text-muted mb-1.5">
          Other services on this host
        </h4>
        <p v-if="otherServices.length === 0" class="text-sm text-muted">
          No other monitors linked to this host.
        </p>
        <ul v-else class="space-y-1" data-testid="other-services">
          <li v-for="svc in otherServices" :key="svc.id">
            <RouterLink
              :to="`/resources/${svc.id}`"
              class="flex items-center gap-2 text-sm text-default hover:text-primary"
            >
              <span
                class="size-2 rounded-full shrink-0"
                :class="svc.status === 'up' ? 'bg-success' : 'bg-error'"
              />
              <span class="truncate">{{ svc.name }}</span>
            </RouterLink>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
