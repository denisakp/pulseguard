<script setup lang="ts">
/**
 * HostDetailView — a single host's detail page: header
 * (name / OS / agent version / last-seen + online state), an "Install agent"
 * helper, a range selector driving `useHostMetrics`, four metric charts
 * (CPU %, RAM %, per-mount Disk %, Network in/out), and the list of monitors
 * linked to the host. Frontend-only; polls the host snapshot every ~12s.
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getHost,
  listMonitors,
  rotateCredential,
  revokeCredential,
  deleteHost,
} from '@/services/hostsService'
import { NotFoundError } from '@/core/errors'
import { timeAgo } from '@/libs/date-time.helper'
import { useConfirm } from '@/composables/useConfirm'
import { useHostMetrics } from '@/composables/useHostMetrics'
import HostMetricChart from '@/components/hosts/HostMetricChart.vue'
import HostCredentialReveal from '@/components/hosts/HostCredentialReveal.vue'
import HostServicesList, {
  type HostLinkedMonitor,
} from '@/components/hosts/HostServicesList.vue'
import type { Host, HostMetricRange } from '@/types'

const POLL_MS = 12_000
const RANGES: HostMetricRange[] = ['1h', '6h', '24h', '7d']

const route = useRoute()
const hostId = computed(() => String(route.params.id ?? ''))

const host = ref<Host | null>(null)
const monitors = ref<HostLinkedMonitor[]>([])
const loading = ref(true)
const notFound = ref(false)
const error = ref<string | null>(null)

const range = ref<HostMetricRange>('24h')
function setRange(r: HostMetricRange) {
  range.value = r
}

const metrics = useHostMetrics(hostId, range)

const cpuSeries = computed(() => metrics.points.value.map((p) => ({ t: p.t, value: p.cpuPct })))
const memSeries = computed(() => metrics.points.value.map((p) => ({ t: p.t, value: p.memPct })))
const netInSeries = computed(() => metrics.points.value.map((p) => ({ t: p.t, value: p.netIn })))
const netOutSeries = computed(() => metrics.points.value.map((p) => ({ t: p.t, value: p.netOut })))
const diskMounts = computed(() => Object.entries(metrics.disksByMount.value))

async function loadHost(): Promise<void> {
  try {
    host.value = await getHost(hostId.value)
    notFound.value = false
  } catch (e) {
    if (e instanceof NotFoundError) {
      notFound.value = true
      host.value = null
    } else {
      error.value = e instanceof Error ? e.message : 'Failed to load host'
    }
  }
}

async function loadMonitors(): Promise<void> {
  try {
    const all = await listMonitors()
    monitors.value = all
      .filter((m) => m.hostId === hostId.value)
      .map((m) => ({
        id: m.id,
        name: m.name,
        type: m.type,
        status: m.status,
        lastCheckedAt: m.lastCheckedAt,
      }))
  } catch {
    // Monitors are supplementary — a failure here should not blank the page.
    monitors.value = []
  }
}

let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  loading.value = true
  await Promise.all([loadHost(), loadMonitors()])
  loading.value = false
  // Poll the host snapshot while the page is open (metrics refetch on range change).
  pollTimer = setInterval(() => {
    void loadHost()
  }, POLL_MS)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

const showInstall = ref(false)

// Credential lifecycle actions.
const router = useRouter()
const rotated = ref<{ credential: string; prefix: string } | null>(null)

async function onRotate() {
  const ok = await useConfirm({
    kind: 'default',
    title: 'Rotate credential?',
    body: "The current credential stops working immediately. You'll get a new one to reinstall on the agent.",
    ctaLabel: 'Rotate',
  })
  if (!ok) return
  rotated.value = await rotateCredential(hostId.value)
}
async function onRevoke() {
  const ok = await useConfirm({
    kind: 'destructive',
    title: 'Revoke credential?',
    body: 'The agent stops authenticating until a new credential is issued. This cannot be undone.',
    ctaLabel: 'Revoke',
  })
  if (!ok) return
  await revokeCredential(hostId.value)
}
async function onDelete() {
  const ok = await useConfirm({
    kind: 'destructive',
    title: 'Delete host?',
    body: 'The host and its metric history are removed and its monitors are unlinked (the monitors themselves are kept). This cannot be undone.',
    ctaLabel: 'Delete',
  })
  if (!ok) return
  await deleteHost(hostId.value)
  await router.push('/hosts')
}
function onRotatedClose() {
  rotated.value = null
}

const onlineLabel = computed(() => (host.value?.online ? 'Online' : 'Offline'))
const onlineDotClass = computed(() => (host.value?.online ? 'bg-success' : 'bg-dimmed'))
const lastSeenLabel = computed(() =>
  host.value?.lastSeenAt ? timeAgo(host.value.lastSeenAt) : 'never',
)

defineExpose({ host, monitors, loading, notFound, range, setRange, metrics })
</script>

<template>
  <div class="p-6 bg-default text-default min-h-screen">
    <div v-if="loading" class="flex justify-center py-20" data-testid="host-loading">
      <UIcon name="i-lucide-loader-circle" class="size-8 animate-spin text-primary-500" />
    </div>

    <div v-else-if="notFound" class="py-20" data-testid="host-not-found">
      <UEmpty
        icon="i-lucide-server-off"
        title="Host not found"
        description="This host does not exist or was removed."
      />
    </div>

    <template v-else-if="host">
      <!-- Header -->
      <div class="flex items-start justify-between gap-4 mb-6">
        <div class="min-w-0">
          <div class="flex items-center gap-3">
            <h1 class="text-2xl font-semibold truncate">{{ host.name }}</h1>
            <span class="inline-flex items-center gap-1.5 text-sm text-muted">
              <span class="size-2 rounded-full" :class="onlineDotClass" />
              {{ onlineLabel }}
            </span>
          </div>
          <div class="flex flex-wrap gap-x-4 gap-y-1 mt-2 text-sm text-muted">
            <span data-testid="host-os">OS: {{ host.os ?? 'unknown' }}</span>
            <span data-testid="host-agent">Agent: {{ host.agentVersion ?? '—' }}</span>
            <span data-testid="host-last-seen">Last seen: {{ lastSeenLabel }}</span>
          </div>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <UButton
            color="neutral"
            variant="subtle"
            icon="i-lucide-download"
            @click="showInstall = !showInstall"
          >
            Install agent
          </UButton>
          <UButton
            color="neutral"
            variant="subtle"
            icon="i-lucide-refresh-cw"
            data-testid="rotate-credential"
            @click="onRotate"
          >
            Rotate
          </UButton>
          <UButton
            color="warning"
            variant="subtle"
            icon="i-lucide-ban"
            data-testid="revoke-credential"
            @click="onRevoke"
          >
            Revoke
          </UButton>
          <UButton
            color="error"
            variant="subtle"
            icon="i-lucide-trash-2"
            data-testid="delete-host"
            @click="onDelete"
          >
            Delete
          </UButton>
        </div>
      </div>

      <UAlert v-if="error" color="error" :title="error" class="mb-4" />

      <!-- Install agent helper (copy mirrors nebula/self-host/agent.md) -->
      <div
        v-if="showInstall"
        class="mb-6 rounded-lg border border-default bg-elevated p-5 text-sm"
        data-testid="install-agent"
      >
        <p class="text-muted mb-3">
          The <code>ogoune-agent</code> runs on this Linux host and streams CPU, memory,
          per-mount disk, and network metrics to Ogoune every ~10 seconds. Use the host's
          credential below — press <strong>Rotate</strong> above to issue a fresh one if you
          don't have it.
        </p>
        <p class="text-muted mb-2 font-medium text-default">Docker (quickest):</p>
        <pre
          class="mb-4 overflow-x-auto rounded-md bg-default p-3 text-xs font-mono text-default"
        >docker run -d --name ogoune-agent --restart unless-stopped \
  --pid=host --network=host \
  -e OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream \
  -e OGOUNE_CREDENTIAL=&lt;this host's credential&gt; \
  ghcr.io/denisakp/ogoune-agent:latest</pre>
        <p class="text-muted mb-2 font-medium text-default">Binary + systemd:</p>
        <pre
          class="overflow-x-auto rounded-md bg-default p-3 text-xs font-mono text-default"
        ># download the release binary for your arch (amd64 / arm64) + verify
sudo install -m755 ogoune-agent-linux-arm64 /usr/local/bin/ogoune-agent
sudo mkdir -p /etc/ogoune
sudo tee /etc/ogoune/agent.cfg &gt;/dev/null &lt;&lt;'EOF'
OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream
OGOUNE_CREDENTIAL=&lt;this host's credential&gt;
EOF
sudo chmod 600 /etc/ogoune/agent.cfg
sudo systemctl enable --now ogoune-agent</pre>
        <p class="text-muted mt-3">
          Within ~15 seconds the host reports live metrics and shows as <strong>online</strong>.
        </p>
      </div>

      <!-- Range selector -->
      <div class="flex items-center gap-2 mb-4" data-testid="range-selector">
        <UButton
          v-for="r in RANGES"
          :key="r"
          :color="range === r ? 'primary' : 'neutral'"
          :variant="range === r ? 'solid' : 'subtle'"
          size="xs"
          :data-range="r"
          @click="setRange(r)"
        >
          {{ r }}
        </UButton>
      </div>

      <UAlert v-if="metrics.error.value" color="error" :title="metrics.error.value" class="mb-4" />

      <div class="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-6">
        <!-- Charts -->
        <div class="space-y-4">
          <HostMetricChart :points="cpuSeries" label="CPU" unit="%" />
          <HostMetricChart :points="memSeries" label="Memory" unit="%" />

          <div class="space-y-4" data-testid="disk-charts">
            <HostMetricChart
              v-for="[mount, series] in diskMounts"
              :key="mount"
              :points="series.map((d) => ({ t: d.t, value: d.usedPct }))"
              :label="`Disk ${mount}`"
              unit="%"
            />
            <div
              v-if="diskMounts.length === 0"
              class="rounded-lg border border-default bg-default p-4 text-xs text-muted text-center"
            >
              No disk metrics in range
            </div>
          </div>

          <HostMetricChart :points="netInSeries" label="Network In" unit="bytes" />
          <HostMetricChart :points="netOutSeries" label="Network Out" unit="bytes" />
        </div>

        <!-- Hosted services -->
        <div>
          <HostServicesList :monitors="monitors" />
        </div>
      </div>
    </template>

    <!-- New credential after a rotate (shown once). -->
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
