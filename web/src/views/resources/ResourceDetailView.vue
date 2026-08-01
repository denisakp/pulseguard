<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useResourceStore } from '@/stores/resourceStore'
import { useConfirm } from '@/composables/useConfirm'
import { timeAgo } from '@/libs/date-time.helper'
import { fetchActivities } from '@/services/activityService'
import { fetchUptimeStats } from '@/services/resourceService'
import ResourceModal from '@/components/resources/ResourceModal.vue'
import IncidentsListBody from '@/components/incidents/IncidentsListBody.vue'
import LinkHostControl from '@/components/hosts/LinkHostControl.vue'
import MonitorHostPanel from '@/components/hosts/MonitorHostPanel.vue'
import { listMonitors } from '@/services/hostsService'
import type { Resource, MonitoringActivity, HourlyUptimeStat } from '@/types'

const route = useRoute()
const router = useRouter()
const resourceStore = useResourceStore()

const resource = ref<Resource | null>(null)
const showModal = ref(false)
const activeTab = ref<'overview' | 'activity' | 'incidents' | 'settings'>('overview')

const ACTIVITIES_PAGE_SIZE = 10
const STRIP_TARGET_BUCKETS = 20

const activities = ref<MonitoringActivity[]>([])
const activitiesLoading = ref(true)
const activitiesLoadingMore = ref(false)
const activitiesHasMore = ref(true)
const activityView = ref<'timeline' | 'strip'>('timeline')
const selectedActivityId = ref<string | null>(null)
const selectedBucketIndex = ref<number | null>(null)

const hourlyStats = ref<HourlyUptimeStat[]>([])
const chartRange = ref<'24h' | '7d' | '30d'>('24h')

interface MetadataLike {
  ssl_issuer?: string
  ssl_expiration_date?: string
  ssl_days_remaining?: number
  domain_registrar?: string
  domain_expiration_date?: string
  domain_days_remaining?: number
}

const metadata = computed<MetadataLike | null>(
  () => (resource.value as unknown as { metadata?: MetadataLike } | null)?.metadata ?? null,
)
const expiryStatus = computed(
  () => (resource.value as unknown as { expiry_status?: string } | null)?.expiry_status ?? 'ok',
)
const sslColor = computed(() => {
  switch (expiryStatus.value) {
    case 'critical':
    case 'expired':
      return { bg: '#FEF2F2', border: '#FCA5A5', icon: '#B91C1C' }
    case 'warning':
      return { bg: '#FFFBEB', border: '#FCD34D', icon: '#B45309' }
    default:
      return { bg: '#ECFDF5', border: '#6EE7B7', icon: '#047857' }
  }
})

interface ActivityGroup {
  label: string
  items: MonitoringActivity[]
}

function dayLabel(d: Date): string {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today.getTime() - 86_400_000)
  const start = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  if (start.getTime() === today.getTime()) return 'Today'
  if (start.getTime() === yesterday.getTime()) return 'Yesterday'
  const daysAgo = Math.floor((today.getTime() - start.getTime()) / 86_400_000)
  if (daysAgo < 7) return `${daysAgo} days ago`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

function timeOfDay(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

interface StripBucket {
  index: number
  startTs: string
  endTs: string
  totalCount: number
  failedCount: number
  items: MonitoringActivity[]
  tone: 'up' | 'mixed' | 'down'
}

const stripBuckets = computed<StripBucket[]>(() => {
  const sorted = [...activities.value].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  )
  if (sorted.length === 0) return []

  const size = Math.max(1, Math.ceil(sorted.length / STRIP_TARGET_BUCKETS))
  const out: StripBucket[] = []
  for (let i = 0; i < sorted.length; i += size) {
    const slice = sorted.slice(i, i + size)
    const failedCount = slice.filter((s) => !s.success).length
    out.push({
      index: out.length,
      startTs: slice[0]!.created_at,
      endTs: slice[slice.length - 1]!.created_at,
      totalCount: slice.length,
      failedCount,
      items: slice,
      tone: failedCount === 0 ? 'up' : failedCount === slice.length ? 'down' : 'mixed',
    })
  }
  return out
})

const selectedBucket = computed(() =>
  selectedBucketIndex.value != null
    ? (stripBuckets.value[selectedBucketIndex.value] ?? null)
    : null,
)

const selectedActivity = computed(() =>
  selectedActivityId.value
    ? (activities.value.find((a) => a.id === selectedActivityId.value) ?? null)
    : null,
)

function bucketColor(tone: StripBucket['tone']): string {
  if (tone === 'down') return '#EF4444'
  if (tone === 'mixed') return '#F59E0B'
  return '#10B981'
}

const activityGroups = computed<ActivityGroup[]>(() => {
  const map = new Map<string, ActivityGroup>()
  for (const a of activities.value) {
    if (!a.created_at) continue
    const d = new Date(a.created_at)
    const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
    if (!map.has(key)) map.set(key, { label: dayLabel(d), items: [] })
    map.get(key)!.items.push(a)
  }
  return [...map.values()].map((g) => ({
    label: g.label,
    items: g.items.sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    ),
  }))
})

const responseTimes = computed(
  () =>
    (
      resource.value as unknown as {
        response_times?: { timestamp: string; response_time: number }[]
      } | null
    )?.response_times ?? [],
)

const filteredResponseTimes = computed(() => {
  const hoursByRange: Record<typeof chartRange.value, number> = {
    '24h': 24,
    '7d': 24 * 7,
    '30d': 24 * 30,
  }
  const cutoff = Date.now() - hoursByRange[chartRange.value] * 3_600_000
  return responseTimes.value
    .filter((p) => new Date(p.timestamp).getTime() >= cutoff)
    .sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
})

const chartAvg = computed(() => {
  const values = filteredResponseTimes.value.map((p) => p.response_time)
  if (values.length === 0) return 0
  return values.reduce((s, v) => s + v, 0) / values.length
})

const TARGET_BARS = 60

const chartBars = computed(() => {
  const data = filteredResponseTimes.value
  if (data.length === 0) return []

  const bucketSize = Math.max(1, Math.ceil(data.length / TARGET_BARS))
  const buckets: { timestamp: string; value: number }[] = []
  for (let i = 0; i < data.length; i += bucketSize) {
    const slice = data.slice(i, i + bucketSize)
    const avg = slice.reduce((s, p) => s + p.response_time, 0) / slice.length
    buckets.push({ timestamp: slice[0]!.timestamp, value: Math.round(avg) })
  }

  const max = Math.max(...buckets.map((b) => b.value), 1)
  return buckets.map((b) => {
    const isAnomaly = chartAvg.value > 0 && b.value >= chartAvg.value * 2
    return {
      timestamp: b.timestamp,
      value: b.value,
      heightPct: Math.max(8, (b.value / max) * 100),
      color: isAnomaly ? '#EF4444' : '#A5B4FC',
    }
  })
})

const incidentCount30d = computed(() => {
  const incidents =
    (resource.value as unknown as { incidents?: { started_at?: string }[] } | null)?.incidents ?? []
  const cutoff = Date.now() - 30 * 24 * 3_600_000
  return incidents.filter((i) => {
    if (!i?.started_at) return false
    const ts = new Date(i.started_at).getTime()
    return !Number.isNaN(ts) && ts >= cutoff
  }).length
})

const uptimeWindows = computed(() => {
  const stats = hourlyStats.value
  const now = Date.now()
  const ranges: Array<{ key: string; hours: number }> = [
    { key: '1h', hours: 1 },
    { key: '24h', hours: 24 },
    { key: '7d', hours: 24 * 7 },
    { key: '30d', hours: 24 * 30 },
    { key: '90d', hours: 24 * 90 },
  ]
  const fmt = (pct: number, key: string) => {
    const tone: 'good' | 'warning' | 'bad' = pct >= 99.9 ? 'good' : pct >= 99 ? 'warning' : 'bad'
    return { key, value: `${pct.toFixed(2)}%`, tone }
  }
  const blank = (key: string) => ({ key, value: '—', tone: 'neutral' as const })

  // Show "—" when the resource is younger than the window. Avoids the
  // "is this 5d or 90d?" ambiguity and keeps list and detail consistent.
  const created = resource.value?.created_at
  const ageHours = created ? (now - new Date(created).getTime()) / 3_600_000 : Infinity

  // 30d comes from the backend (uptime_daily_agg) so detail and list views
  // agree on the same number. hourlyStats only covers the last 24h, so
  // computing 30d/90d locally from it would silently report the 24h value.
  const backendUptime30d = resource.value?.uptime_30d

  return ranges.map((r) => {
    if (ageHours < r.hours) return blank(r.key)
    if (r.key === '30d' && typeof backendUptime30d === 'number') {
      return fmt(backendUptime30d * 100, '30d')
    }
    const cutoff = now - r.hours * 3_600_000
    const inWindow = stats.filter((s) => new Date(s.hour).getTime() >= cutoff)
    const total = inWindow.reduce((s, x) => s + x.total_count, 0)
    const success = inWindow.reduce((s, x) => s + x.successful_count, 0)
    if (total === 0) return blank(r.key)
    return fmt((success / total) * 100, r.key)
  })
})

const uptime30d = computed(() => uptimeWindows.value.find((w) => w.key === '30d') ?? null)

const responseTimeStats = computed(() => {
  const rt = filteredResponseTimes.value
  if (rt.length === 0) return null
  const values = rt.map((r) => r.response_time)
  const sum = values.reduce((s, v) => s + v, 0)
  return {
    avg: Math.round(sum / values.length),
    min: Math.min(...values),
    max: Math.max(...values),
  }
})

const statusColor = computed(() => {
  switch (resource.value?.status) {
    case 'up':
      return '#10B981'
    case 'down':
      return '#EF4444'
    case 'flapping':
      return '#F59E0B'
    case 'paused':
      return '#94A3B8'
    default:
      return '#94A3B8'
  }
})

const isPaused = computed(() => resource.value?.status === 'paused')

const targetSummary = computed(() => {
  const r = resource.value as unknown as { target?: string; type?: string } | null
  if (!r) return ''
  const t = r.target?.trim()
  const kind = (r.type ?? '').toUpperCase()
  return t ? `${kind} monitor for ${t}` : `${kind} monitor`
})

const limitByRange: Record<typeof chartRange.value, number> = {
  '24h': 200,
  '7d': 800,
  '30d': 2000,
}

async function loadDetail() {
  const id = String(route.params.id)
  const r = await resourceStore.loadResourceWithResponseTimes(id, limitByRange[chartRange.value])
  resource.value = (r as Resource | undefined) ?? null
}

watch(chartRange, () => {
  void loadDetail()
})

async function loadActivity() {
  if (!resource.value) return
  activitiesLoading.value = true
  activitiesHasMore.value = true
  selectedActivityId.value = null
  selectedBucketIndex.value = null
  try {
    const page = await fetchActivities(resource.value.id, ACTIVITIES_PAGE_SIZE, 0)
    activities.value = page
    activitiesHasMore.value = page.length === ACTIVITIES_PAGE_SIZE
  } catch {
    activities.value = []
    activitiesHasMore.value = false
  } finally {
    activitiesLoading.value = false
  }
}

async function loadMoreActivity() {
  if (!resource.value || activitiesLoadingMore.value || !activitiesHasMore.value) return
  activitiesLoadingMore.value = true
  try {
    const page = await fetchActivities(
      resource.value.id,
      ACTIVITIES_PAGE_SIZE,
      activities.value.length,
    )
    activities.value = [...activities.value, ...page]
    activitiesHasMore.value = page.length === ACTIVITIES_PAGE_SIZE
  } catch {
    activitiesHasMore.value = false
  } finally {
    activitiesLoadingMore.value = false
  }
}

async function loadUptimeWindows() {
  if (!resource.value) return
  try {
    const r = await fetchUptimeStats(resource.value.id)
    hourlyStats.value = r.stats ?? []
  } catch {
    hourlyStats.value = []
  }
}

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function formatInterval(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return '—'
  if (seconds < 60) return `${seconds} s`
  if (seconds < 3600) {
    const m = Math.round(seconds / 60)
    return `${m} min`
  }
  if (seconds < 86400) {
    const h = Math.round((seconds / 3600) * 10) / 10
    return `${h} h`
  }
  const d = Math.round((seconds / 86400) * 10) / 10
  return `${d} d`
}

function displayTarget(r: Resource | null): string {
  if (!r) return '—'
  switch (r.type) {
    case 'http':
    case 'keyword':
      return r.target || '—'
    case 'tcp':
    case 'protocol': {
      const t = r.target ?? ''
      if (t.includes(':')) return t
      return r.protocol_port ? `${t}:${r.protocol_port}` : t
    }
    case 'dns':
    case 'icmp':
      return r.target || '—'
    case 'heartbeat':
      return r.heartbeat_slug || '—'
    default:
      return r.target || '—'
  }
}

async function onModalSubmit() {
  showModal.value = false
  await loadDetail()
}

async function togglePause() {
  if (!resource.value) return
  if (isPaused.value) {
    await resourceStore.resumeMonitoring(resource.value.id)
  } else {
    await resourceStore.pauseMonitoring(resource.value.id)
  }
  await loadDetail()
}

async function onDelete() {
  if (!resource.value) return
  const ok = await useConfirm({
    kind: 'destructive',
    title: 'Delete monitor?',
    body: `${resource.value.name} will stop being checked immediately.`,
    ctaLabel: 'Delete',
  })
  if (ok) {
    await resourceStore.removeResource(resource.value.id)
    router.push('/resources')
  }
}

// Host link (spec 081). host_id lives on the v1 monitor, not the root resource,
// so derive the current monitor's host from the v1 monitors list.
const monitorId = computed(() => String(route.params.id))
const currentHostId = ref<string | null>(null)
async function loadHostLink() {
  try {
    const mons = await listMonitors()
    currentHostId.value = mons.find((m) => m.id === monitorId.value)?.hostId ?? null
  } catch {
    currentHostId.value = null
  }
}
function onHostChanged(hostId: string | null) {
  currentHostId.value = hostId
}

onMounted(async () => {
  await loadDetail()
  void loadActivity()
  void loadUptimeWindows()
  void loadHostLink()
})

defineExpose({ resource, activeTab, loadDetail, loadActivity, togglePause, onDelete })
</script>

<template>
  <div class="bg-default text-default min-h-full">
    <div v-if="!resource" class="px-6 py-12 text-center text-sm text-muted">Loading…</div>
    <template v-else>
      <div class="flex items-center justify-between mb-5">
        <div class="flex items-center gap-2 text-sm text-muted">
          <RouterLink to="/resources" class="hover:text-default">Resources</RouterLink>
          <UIcon name="i-lucide-chevron-right" class="size-3.5 text-dimmed" />
          <span class="text-highlighted font-medium">{{ resource.name }}</span>
        </div>
        <div class="flex items-center gap-2">
          <UButton
            color="neutral"
            variant="outline"
            size="sm"
            icon="i-lucide-zap"
            @click="loadDetail"
          >
            Test now
          </UButton>
          <UButton
            color="neutral"
            variant="outline"
            size="sm"
            :icon="isPaused ? 'i-lucide-play' : 'i-lucide-pause'"
            @click="togglePause"
          >
            {{ isPaused ? 'Resume' : 'Pause' }}
          </UButton>
          <UButton
            color="neutral"
            variant="outline"
            size="sm"
            icon="i-lucide-pencil"
            @click="showModal = true"
          >
            Edit
          </UButton>
          <UButton
            color="error"
            variant="outline"
            size="sm"
            icon="i-lucide-trash-2"
            @click="onDelete"
          >
            Delete
          </UButton>
        </div>
      </div>

      <div class="flex items-center gap-4 mb-5">
        <span class="size-3 rounded-full" :style="{ backgroundColor: statusColor }" />
        <div class="flex flex-col gap-0.5">
          <h1 class="text-[22px] font-semibold font-mono text-highlighted leading-tight">
            {{ resource.name }}
          </h1>
          <p class="text-sm text-muted">{{ targetSummary }}</p>
        </div>
      </div>

      <div class="flex items-center gap-1 border-b border-default mb-6">
        <button
          v-for="t in ['overview', 'activity', 'incidents', 'settings'] as const"
          :key="t"
          type="button"
          class="px-4 py-2 text-sm capitalize border-b-2 transition-colors"
          :class="
            activeTab === t
              ? 'border-primary-600 text-primary-600 font-medium'
              : 'border-transparent text-muted hover:text-highlighted'
          "
          @click="activeTab = t"
        >
          {{ t }}
        </button>
      </div>

      <div v-if="activeTab === 'overview'" class="grid grid-cols-[1fr_320px] gap-6 items-start">
        <div class="flex flex-col gap-5 min-w-0">
          <!-- Host link (spec 081). The context panel shows only when linked. -->
          <div class="bg-default rounded-lg border border-default p-5 flex flex-col gap-4">
            <div class="flex items-center justify-between gap-3">
              <h3 class="text-base font-semibold text-highlighted">Host</h3>
              <LinkHostControl
                :monitor-id="monitorId"
                :current-host-id="currentHostId"
                @changed="onHostChanged"
              />
            </div>
            <MonitorHostPanel
              v-if="currentHostId"
              :host-id="currentHostId"
              :monitor-id="monitorId"
            />
            <p v-else class="text-sm text-muted">
              This monitor is not linked to a host. Link it to see the machine's live
              CPU, memory and disk alongside its checks.
            </p>
          </div>

          <div class="grid grid-cols-4 gap-3">
            <div class="bg-default rounded-lg border border-default p-4">
              <div class="text-xs text-muted mb-1">Status</div>
              <div class="text-xl font-semibold capitalize" :style="{ color: statusColor }">
                {{ resource.status }}
              </div>
            </div>
            <div class="bg-default rounded-lg border border-default p-4">
              <div class="text-xs text-muted mb-1">Uptime (30d)</div>
              <div class="text-xl font-semibold text-highlighted">
                {{ uptime30d?.value ?? '—' }}
              </div>
            </div>
            <div class="bg-default rounded-lg border border-default p-4">
              <div class="text-xs text-muted mb-1">Incidents (30d)</div>
              <div class="text-xl font-semibold text-highlighted">{{ incidentCount30d }}</div>
            </div>
            <div class="bg-default rounded-lg border border-default p-4">
              <div class="text-xs text-muted mb-1">Last Checked</div>
              <div class="text-xl font-semibold text-highlighted">
                {{ resource.last_checked ? timeAgo(resource.last_checked) : '—' }}
              </div>
            </div>
          </div>

          <div class="bg-default rounded-lg border border-default p-5">
            <h3 class="text-base font-semibold text-highlighted mb-3">Uptime by Time Window</h3>
            <div class="grid grid-cols-5 gap-3">
              <div
                v-for="w in uptimeWindows"
                :key="w.key"
                class="bg-muted rounded-md p-3 text-center"
              >
                <div class="text-[11px] text-muted mb-1">{{ w.key }}</div>
                <div
                  class="text-base font-semibold"
                  :class="{
                    'text-emerald-600': w.tone === 'good',
                    'text-amber-600': w.tone === 'warning',
                    'text-red-600': w.tone === 'bad',
                    'text-dimmed': w.tone === 'neutral',
                  }"
                >
                  {{ w.value }}
                </div>
              </div>
            </div>
          </div>

          <div class="bg-default rounded-lg border border-default p-5">
            <div class="flex items-center justify-between mb-3">
              <h3 class="text-base font-semibold text-highlighted">Response Time</h3>
              <div class="flex p-0.5 rounded-md bg-muted">
                <button
                  v-for="r in ['24h', '7d', '30d'] as const"
                  :key="r"
                  type="button"
                  class="px-3 py-1 rounded text-xs font-medium transition-colors"
                  :class="
                    chartRange === r
                      ? 'bg-default text-highlighted shadow-sm'
                      : 'text-muted hover:text-default'
                  "
                  @click="chartRange = r"
                >
                  {{ r }}
                </button>
              </div>
            </div>
            <div
              v-if="responseTimeStats"
              class="flex items-center gap-5 text-xs text-muted mb-3"
            >
              <span class="inline-flex items-center gap-1.5">
                <span class="size-2 rounded-full" style="background-color: #4f46e5" />
                Average
                <strong class="text-highlighted font-semibold">{{ responseTimeStats.avg }}ms</strong>
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="size-2 rounded-full" style="background-color: #10b981" />
                Min
                <strong class="text-highlighted font-semibold">{{ responseTimeStats.min }}ms</strong>
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="size-2 rounded-full" style="background-color: #ef4444" />
                Max
                <strong class="text-highlighted font-semibold">{{ responseTimeStats.max }}ms</strong>
              </span>
            </div>
            <div v-if="chartBars.length === 0" class="text-sm text-muted text-center py-12">
              No response time data for this window.
            </div>
            <div v-else class="flex items-end gap-1.5 h-44">
              <div
                v-for="(b, i) in chartBars"
                :key="i"
                class="flex-1 rounded-sm group relative"
                :style="{ height: `${b.heightPct}%`, backgroundColor: b.color }"
              >
                <div
                  class="absolute bottom-full left-1/2 -translate-x-1/2 mb-1 hidden group-hover:flex flex-col items-center px-2 py-1 rounded bg-inverted text-white text-[10px] whitespace-nowrap"
                >
                  <span class="font-mono">{{ b.value }}ms</span>
                  <span class="text-dimmed">{{ new Date(b.timestamp).toLocaleString() }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-5">
          <div class="bg-default rounded-lg border border-default p-5">
            <h3 class="text-base font-semibold text-highlighted mb-4">Monitor Details</h3>
            <dl class="space-y-3.5 text-sm">
              <div>
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Type</dt>
                <dd class="text-highlighted font-medium">
                  {{ String(resource.type).toUpperCase() }}
                </dd>
              </div>
              <div>
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Target</dt>
                <dd class="text-highlighted font-mono text-xs break-all">
                  {{ (resource as unknown as { target?: string }).target ?? '—' }}
                </dd>
              </div>
              <div v-if="resource.type === 'http'">
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Method</dt>
                <dd class="text-highlighted font-mono text-xs">
                  {{ (resource as unknown as { method?: string }).method ?? 'GET' }}
                </dd>
              </div>
              <div>
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Interval</dt>
                <dd class="text-highlighted">
                  Every {{ (resource as unknown as { interval?: number }).interval ?? 60 }} seconds
                </dd>
              </div>
              <div v-if="(resource as unknown as { timeout?: number }).timeout != null">
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Timeout</dt>
                <dd class="text-highlighted">
                  {{ (resource as unknown as { timeout: number }).timeout }} seconds
                </dd>
              </div>
              <div v-if="(resource as unknown as { created_at?: string }).created_at">
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">Created</dt>
                <dd class="text-highlighted">
                  {{ formatDate((resource as unknown as { created_at: string }).created_at) }}
                </dd>
              </div>
              <div v-if="(resource as unknown as { updated_at?: string }).updated_at">
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-0.5">
                  Last Updated
                </dt>
                <dd class="text-highlighted">
                  {{ formatDate((resource as unknown as { updated_at: string }).updated_at) }}
                </dd>
              </div>
              <div v-if="resource.tags && resource.tags.length > 0">
                <dt class="text-[11px] text-muted uppercase tracking-wide mb-1.5">Tags</dt>
                <dd class="flex flex-wrap gap-1.5">
                  <span
                    v-for="t in resource.tags"
                    :key="(t as unknown as { id?: string }).id ?? String(t)"
                    class="inline-block px-2 py-0.5 rounded-md text-xs bg-elevated text-default"
                  >
                    {{ (t as unknown as { name?: string }).name ?? String(t) }}
                  </span>
                </dd>
              </div>
            </dl>
          </div>

          <div
            v-if="metadata?.ssl_issuer || metadata?.ssl_expiration_date"
            class="rounded-lg border p-5"
            :style="{ backgroundColor: sslColor.bg, borderColor: sslColor.border }"
          >
            <div class="flex items-center gap-2 mb-3">
              <UIcon
                name="i-lucide-shield-check"
                class="size-4"
                :style="{ color: sslColor.icon }"
              />
              <h3 class="text-sm font-semibold" :style="{ color: sslColor.icon }">
                SSL Certificate
              </h3>
            </div>
            <div class="space-y-1.5 text-xs">
              <div v-if="metadata?.ssl_issuer" class="text-default">
                Issuer: <span class="font-medium">{{ metadata.ssl_issuer }}</span>
              </div>
              <div v-if="metadata?.ssl_expiration_date" class="text-default">
                Expires:
                <span class="font-medium">{{ formatDate(metadata.ssl_expiration_date) }}</span>
                <span v-if="metadata.ssl_days_remaining != null" class="text-muted">
                  ({{ metadata.ssl_days_remaining }} days)
                </span>
              </div>
            </div>
          </div>

          <div
            v-if="metadata?.domain_registrar || metadata?.domain_expiration_date"
            class="rounded-lg border border-default bg-default p-5"
          >
            <div class="flex items-center gap-2 mb-3">
              <UIcon name="i-lucide-globe" class="size-4 text-muted" />
              <h3 class="text-sm font-semibold text-highlighted">Domain</h3>
            </div>
            <div class="space-y-1.5 text-xs">
              <div v-if="metadata?.domain_registrar" class="text-default">
                Registrar: <span class="font-medium">{{ metadata.domain_registrar }}</span>
              </div>
              <div v-if="metadata?.domain_expiration_date" class="text-default">
                Expires:
                <span class="font-medium">{{ formatDate(metadata.domain_expiration_date) }}</span>
                <span v-if="metadata.domain_days_remaining != null" class="text-muted">
                  ({{ metadata.domain_days_remaining }} days)
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else-if="activeTab === 'activity'"
        class="bg-default rounded-lg border border-default overflow-hidden"
      >
        <div class="flex items-center justify-between gap-3 px-6 py-4 border-b border-default">
          <h3 class="text-base font-semibold text-highlighted">Activity log</h3>
          <div class="flex items-center gap-3">
            <span v-if="activities.length > 0" class="text-xs text-muted">
              {{ activities.length }} check{{ activities.length > 1 ? 's' : '' }}
            </span>
            <div class="flex p-0.5 rounded-md bg-muted">
              <button
                type="button"
                class="px-2.5 py-1 rounded text-[11px] font-medium inline-flex items-center gap-1.5"
                :class="
                  activityView === 'timeline'
                    ? 'bg-default text-highlighted shadow-sm'
                    : 'text-muted hover:text-default'
                "
                @click="activityView = 'timeline'"
              >
                <UIcon name="i-lucide-list" class="size-3" />
                Timeline
              </button>
              <button
                type="button"
                class="px-2.5 py-1 rounded text-[11px] font-medium inline-flex items-center gap-1.5"
                :class="
                  activityView === 'strip'
                    ? 'bg-default text-highlighted shadow-sm'
                    : 'text-muted hover:text-default'
                "
                @click="activityView = 'strip'"
              >
                <UIcon name="i-lucide-bar-chart-3" class="size-3" />
                Strip
              </button>
            </div>
          </div>
        </div>

        <div v-if="activitiesLoading" class="px-6 py-4 space-y-2">
          <USkeleton v-for="i in 6" :key="i" class="h-6 w-full" />
        </div>
        <UEmpty
          v-else-if="activityGroups.length === 0"
          variant="naked"
          icon="i-lucide-inbox"
          title="No activity yet"
          description="Once this monitor starts running checks, the log will populate here."
        />
        <div v-else-if="activityView === 'timeline'" class="px-6 py-4">
          <div v-for="g in activityGroups" :key="g.label" class="mb-6 last:mb-0">
            <div
              class="text-[10px] font-semibold tracking-wider text-dimmed uppercase mb-2 pl-6"
            >
              {{ g.label }}
            </div>
            <div class="relative pl-6 border-l border-default">
              <div
                v-for="a in g.items"
                :key="a.id"
                class="relative -ml-6.75 flex items-center gap-3 py-2 pl-6 pr-2 rounded-md hover:bg-muted"
              >
                <span
                  class="absolute left-0 size-2 rounded-full ring-2 ring-white shrink-0"
                  :style="{ backgroundColor: a.success ? '#10B981' : '#EF4444' }"
                />
                <span class="text-xs font-mono text-muted w-20 shrink-0">
                  {{ timeOfDay(a.created_at) }}
                </span>
                <span
                  class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium shrink-0"
                  :style="
                    a.success
                      ? { backgroundColor: '#ECFDF5', color: '#047857' }
                      : { backgroundColor: '#FEF2F2', color: '#B91C1C' }
                  "
                >
                  {{ a.success ? 'Up' : 'Down' }}
                </span>
                <span class="text-sm text-default flex-1 truncate min-w-0">
                  {{ a.message || (a.success ? 'Check passed' : 'Check failed') }}
                </span>
                <span class="text-xs font-mono text-muted shrink-0">
                  {{ a.response_time != null ? `${a.response_time}ms` : '—' }}
                </span>
              </div>
            </div>
          </div>
          <div v-if="activitiesHasMore" class="flex justify-center pt-2 pb-1">
            <UButton
              color="neutral"
              variant="outline"
              size="sm"
              :loading="activitiesLoadingMore"
              icon="i-lucide-chevron-down"
              @click="loadMoreActivity"
            >
              Load {{ ACTIVITIES_PAGE_SIZE }} more
            </UButton>
          </div>
          <div
            v-else-if="activities.length > 0"
            class="text-center text-[11px] text-dimmed pt-2 pb-1"
          >
            End of log
          </div>
        </div>

        <div v-else class="px-6 py-5">
          <div class="flex items-end gap-px h-12 mb-2">
            <button
              v-for="b in stripBuckets"
              :key="b.index"
              type="button"
              class="flex-1 min-w-0.75 rounded-sm transition-opacity"
              :class="{
                'opacity-100': selectedBucketIndex === b.index,
                'opacity-90 hover:opacity-100': selectedBucketIndex !== b.index,
                'ring-2 ring-primary-600 ring-offset-1': selectedBucketIndex === b.index,
              }"
              :style="{ backgroundColor: bucketColor(b.tone), height: '100%' }"
              :title="`${new Date(b.startTs).toLocaleString()} → ${new Date(b.endTs).toLocaleString()} · ${b.totalCount} checks (${b.failedCount} failed)`"
              @click="selectedBucketIndex = selectedBucketIndex === b.index ? null : b.index"
            />
          </div>
          <div class="flex items-center justify-between text-[10px] text-dimmed mb-4">
            <span v-if="stripBuckets.length > 0">
              {{ new Date(stripBuckets[0]!.startTs).toLocaleString() }}
            </span>
            <span class="text-muted">
              {{ activities.length }} checks
              <template v-if="stripBuckets.length < activities.length">
                · bucketed into {{ stripBuckets.length }} groups
              </template>
            </span>
            <span v-if="stripBuckets.length > 0">
              {{ new Date(stripBuckets[stripBuckets.length - 1]!.endTs).toLocaleString() }}
            </span>
          </div>

          <div
            v-if="selectedBucket"
            class="rounded-md border border-default bg-muted p-4 space-y-3"
          >
            <div class="flex items-center justify-between gap-3">
              <div class="flex items-center gap-2 flex-wrap">
                <span
                  class="size-2 rounded-full"
                  :style="{ backgroundColor: bucketColor(selectedBucket.tone) }"
                />
                <span
                  class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium"
                  :style="
                    selectedBucket.tone === 'up'
                      ? { backgroundColor: '#ECFDF5', color: '#047857' }
                      : selectedBucket.tone === 'down'
                        ? { backgroundColor: '#FEF2F2', color: '#B91C1C' }
                        : { backgroundColor: '#FFFBEB', color: '#92400E' }
                  "
                >
                  {{
                    selectedBucket.tone === 'up'
                      ? 'All Up'
                      : selectedBucket.tone === 'down'
                        ? 'All Down'
                        : 'Mixed'
                  }}
                </span>
                <span class="text-xs text-muted">
                  {{ selectedBucket.totalCount }} check{{
                    selectedBucket.totalCount > 1 ? 's' : ''
                  }}
                  <template v-if="selectedBucket.failedCount > 0">
                    · <strong>{{ selectedBucket.failedCount }} failed</strong>
                  </template>
                </span>
              </div>
              <button
                type="button"
                class="text-dimmed hover:text-default"
                @click="selectedBucketIndex = null"
              >
                <UIcon name="i-lucide-x" class="size-4" />
              </button>
            </div>
            <div class="text-xs text-muted">
              From
              <span class="font-mono">{{ new Date(selectedBucket.startTs).toLocaleString() }}</span>
              to
              <span class="font-mono">{{ new Date(selectedBucket.endTs).toLocaleString() }}</span>
            </div>
            <div v-if="selectedBucket.items.length > 0" class="space-y-1 max-h-60 overflow-auto">
              <button
                v-for="a in selectedBucket.items"
                :key="a.id"
                type="button"
                class="w-full text-left flex items-center gap-3 px-2 py-1.5 rounded hover:bg-default text-xs"
                :class="{ 'bg-default ring-1 ring-primary-200': selectedActivityId === a.id }"
                @click="selectedActivityId = selectedActivityId === a.id ? null : a.id"
              >
                <span
                  class="size-1.5 rounded-full shrink-0"
                  :style="{ backgroundColor: a.success ? '#10B981' : '#EF4444' }"
                />
                <span class="font-mono text-muted w-20 shrink-0">
                  {{ timeOfDay(a.created_at) }}
                </span>
                <span class="text-default flex-1 truncate min-w-0">
                  {{ a.message || (a.success ? 'Check passed' : 'Check failed') }}
                </span>
                <span class="text-muted font-mono shrink-0">
                  {{ a.response_time != null ? `${a.response_time}ms` : '—' }}
                </span>
              </button>
            </div>
            <div
              v-if="
                selectedActivity &&
                (selectedActivity as unknown as { response_data?: string }).response_data
              "
              class="text-xs"
            >
              <div class="text-[10px] text-muted uppercase tracking-wider mb-1">
                Response data
              </div>
              <pre
                class="bg-default border border-default rounded p-2 font-mono text-[11px] text-default max-h-32 overflow-auto"
                >{{ (selectedActivity as unknown as { response_data: string }).response_data }}</pre
              >
            </div>
          </div>
          <div
            v-else
            class="text-xs text-muted text-center py-3 border border-dashed border-default rounded-md"
          >
            Hover a bar for a quick preview · Click for the bucket's check list.
          </div>

          <div v-if="activitiesHasMore" class="flex justify-center pt-4">
            <UButton
              color="neutral"
              variant="outline"
              size="sm"
              :loading="activitiesLoadingMore"
              icon="i-lucide-chevron-down"
              @click="loadMoreActivity"
            >
              Load {{ ACTIVITIES_PAGE_SIZE }} more
            </UButton>
          </div>
        </div>
      </div>

      <div
        v-else-if="activeTab === 'incidents'"
        class="bg-default rounded-lg border border-default overflow-hidden"
      >
        <div class="px-6 py-4 border-b border-default flex items-center justify-between">
          <h3 class="text-base font-semibold text-highlighted">Per-resource incidents</h3>
          <RouterLink to="/incidents" class="text-xs font-medium text-primary-600 hover:underline">
            See all incidents →
          </RouterLink>
        </div>
        <IncidentsListBody
          :filter="{ resource_id: resource.id }"
          @row-click="(i) => router.push({ name: 'IncidentDetail', params: { id: i.id } })"
        />
      </div>

      <div v-else class="space-y-4">
        <!-- TARGET -->
        <section class="bg-default rounded-lg border border-default p-6">
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-target" class="size-4 text-muted" />
              <h3 class="text-base font-semibold text-highlighted">Target</h3>
            </div>
          </div>
          <dl class="grid grid-cols-1 md:grid-cols-3 gap-y-3 text-sm">
            <dt class="text-muted">Type</dt>
            <dd class="md:col-span-2">
              <UBadge variant="subtle" color="primary" size="sm">
                {{ resource.type.toUpperCase() }}
              </UBadge>
            </dd>

            <dt class="text-muted">
              {{
                resource.type === 'http' || resource.type === 'keyword'
                  ? 'URL'
                  : resource.type === 'heartbeat'
                    ? 'Heartbeat slug'
                    : 'Target'
              }}
            </dt>
            <dd class="md:col-span-2 font-mono text-xs text-highlighted break-all">
              {{ displayTarget(resource) }}
            </dd>

            <template v-if="resource.type === 'keyword'">
              <dt class="text-muted">Keyword</dt>
              <dd class="md:col-span-2 font-mono text-xs text-highlighted">
                {{ resource.keyword || '—' }}
                <span v-if="resource.keyword_mode" class="ml-2 text-muted">
                  ({{ resource.keyword_mode }})
                </span>
              </dd>
            </template>

            <template v-if="resource.type === 'protocol'">
              <dt class="text-muted">Protocol</dt>
              <dd class="md:col-span-2 text-highlighted">
                {{ resource.protocol_type ?? '—' }}
              </dd>
            </template>
          </dl>
        </section>

        <!-- SCHEDULE -->
        <section class="bg-default rounded-lg border border-default p-6">
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-clock" class="size-4 text-muted" />
              <h3 class="text-base font-semibold text-highlighted">Schedule</h3>
            </div>
          </div>
          <dl class="grid grid-cols-1 md:grid-cols-3 gap-y-3 text-sm">
            <dt class="text-muted">Check interval</dt>
            <dd class="md:col-span-2 text-highlighted">{{ formatInterval(resource.interval) }}</dd>

            <dt class="text-muted">Timeout</dt>
            <dd class="md:col-span-2 text-highlighted">{{ formatInterval(resource.timeout) }}</dd>

            <dt class="text-muted">Confirmation</dt>
            <dd class="md:col-span-2 text-highlighted">
              {{ resource.confirmation_checks }} consecutive failures
              <span class="text-muted">
                · every {{ formatInterval(resource.confirmation_interval) }}
              </span>
            </dd>

            <dt class="text-muted">Active</dt>
            <dd class="md:col-span-2">
              <UBadge
                v-if="resource.is_active"
                variant="subtle"
                color="success"
                size="sm"
                icon="i-lucide-check"
              >
                Enabled
              </UBadge>
              <UBadge
                v-else
                variant="subtle"
                color="neutral"
                size="sm"
                icon="i-lucide-pause"
              >
                Paused
              </UBadge>
            </dd>
          </dl>
        </section>

        <!-- FLAP DETECTION -->
        <section
          v-if="resource.flap_detection_enabled"
          class="bg-default rounded-lg border border-default p-6"
        >
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-activity" class="size-4 text-muted" />
              <h3 class="text-base font-semibold text-highlighted">Flap detection</h3>
            </div>
          </div>
          <dl class="grid grid-cols-1 md:grid-cols-3 gap-y-3 text-sm">
            <dt class="text-muted">Threshold</dt>
            <dd class="md:col-span-2 text-highlighted">
              {{ resource.flap_threshold ?? '—' }} transitions
            </dd>
            <dt class="text-muted">Window</dt>
            <dd class="md:col-span-2 text-highlighted">
              {{ formatInterval(resource.flap_window_seconds) }}
            </dd>
            <dt class="text-muted">Max duration</dt>
            <dd class="md:col-span-2 text-highlighted">
              {{ resource.flap_max_duration_minutes ?? '—' }} min
            </dd>
          </dl>
        </section>

        <!-- TAGS -->
        <section class="bg-default rounded-lg border border-default p-6">
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-tag" class="size-4 text-muted" />
              <h3 class="text-base font-semibold text-highlighted">Tags</h3>
            </div>
          </div>
          <div
            v-if="(resource.tags?.length ?? 0) > 0"
            class="flex flex-wrap gap-2"
          >
            <UBadge
              v-for="tag in resource.tags ?? []"
              :key="tag.id"
              variant="subtle"
              color="neutral"
              size="md"
            >
              {{ tag.name }}
            </UBadge>
          </div>
          <p v-else class="text-sm text-muted">
            No tags. Add tags to group monitors and target alerts.
          </p>
        </section>

        <!-- META (footer, low-vis) -->
        <section class="bg-muted rounded-lg border border-muted p-4">
          <dl class="grid grid-cols-1 md:grid-cols-3 gap-y-2 text-xs text-muted">
            <dt>Resource ID</dt>
            <dd class="md:col-span-2 font-mono break-all">{{ resource.id }}</dd>
            <dt>Created</dt>
            <dd class="md:col-span-2">{{ formatDate(resource.created_at) }}</dd>
            <dt>Last updated</dt>
            <dd class="md:col-span-2">{{ formatDate(resource.updated_at) }}</dd>
          </dl>
        </section>

        <!-- DANGER ZONE -->
        <section class="bg-red-50 rounded-lg border border-red-200 p-6">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h3 class="text-base font-semibold text-red-700">Danger Zone</h3>
              <p class="text-xs text-red-600/80 mt-1">
                Pause monitoring to stop checks without losing history. Delete removes the monitor
                and all its data.
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <UButton
                variant="outline"
                color="neutral"
                size="sm"
                :icon="isPaused ? 'i-lucide-play' : 'i-lucide-pause'"
                @click="togglePause"
              >
                {{ isPaused ? 'Resume' : 'Pause' }}
              </UButton>
              <UButton
                color="error"
                size="sm"
                icon="i-lucide-trash-2"
                @click="onDelete"
              >
                Delete
              </UButton>
            </div>
          </div>
        </section>
      </div>

      <ResourceModal v-model:open="showModal" :resource="resource" @submit="onModalSubmit" />
    </template>
  </div>
</template>
