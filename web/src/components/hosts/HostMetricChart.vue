<script setup lang="ts">
/**
 * HostMetricChart — hand-rolled SVG line/area chart for a
 * single host-metric series. No charting library (the codebase has none);
 * mirrors the DIY approach of `ResponseTimeChart.vue`.
 *
 * `%` unit pins the y-axis to 0–100; `bytes` auto-scales to the series max
 * (or an explicit `max`) and humanises the axis/label. Renders an empty state
 * when there are no points.
 */
import { computed } from 'vue'

interface Point {
  t: string
  value: number
}

interface Props {
  points: Point[]
  label: string
  unit: '%' | 'bytes'
  /** Optional upper bound for the y-axis (bytes only; `%` is always 0–100). */
  max?: number
  height?: number
}

const props = withDefaults(defineProps<Props>(), {
  max: undefined,
  height: 160,
})

// SVG is drawn in an abstract 100 x 40 viewBox, stretched to fill via
// preserveAspectRatio="none" so it stays crisp at any width.
const VB_W = 100
const VB_H = 40

const hasData = computed(() => props.points.length > 0)

const yMax = computed(() => {
  if (props.unit === '%') return 100
  const dataMax = props.points.reduce((m, p) => Math.max(m, p.value), 0)
  return Math.max(1, props.max ?? dataMax)
})

interface Coord {
  x: number
  y: number
}

const coords = computed<Coord[]>(() => {
  const n = props.points.length
  if (n === 0) return []
  const denom = Math.max(1, n - 1)
  const top = yMax.value
  return props.points.map((p, i) => {
    const x = (i / denom) * VB_W
    const ratio = Math.min(1, Math.max(0, p.value / top))
    const y = VB_H - ratio * VB_H
    return { x, y }
  })
})

/** Polyline points attribute for the series line. */
const linePoints = computed(() => coords.value.map((c) => `${c.x},${c.y}`).join(' '))

/** Closed path for the filled area under the line. */
const areaPath = computed(() => {
  const c = coords.value
  if (c.length === 0) return ''
  const first = c[0]!
  const last = c[c.length - 1]!
  const line = c.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ')
  return `${line} L${last.x},${VB_H} L${first.x},${VB_H} Z`
})

function humanBytes(v: number): string {
  if (v < 1024) return `${Math.round(v)} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let n = v / 1024
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(1)} ${units[i]}`
}

function format(v: number): string {
  return props.unit === '%' ? `${Math.round(v)}%` : humanBytes(v)
}

const latest = computed(() => {
  const p = props.points[props.points.length - 1]
  return p ? format(p.value) : '—'
})

const axisTop = computed(() => format(yMax.value))
</script>

<template>
  <div class="rounded-lg border border-default bg-default p-4" data-testid="host-metric-chart">
    <div class="flex items-baseline justify-between mb-3">
      <h4 class="text-sm font-medium text-highlighted">{{ label }}</h4>
      <span class="text-sm font-mono text-muted" data-testid="metric-latest">{{ latest }}</span>
    </div>

    <div
      v-if="!hasData"
      class="flex items-center justify-center text-xs text-muted"
      :style="{ height: `${height}px` }"
      data-testid="metric-empty"
    >
      No data in range
    </div>

    <div v-else class="relative w-full" :style="{ height: `${height}px` }">
      <span class="absolute left-0 top-0 text-[10px] font-mono text-dimmed">{{ axisTop }}</span>
      <span class="absolute left-0 bottom-0 text-[10px] font-mono text-dimmed">0</span>
      <svg
        class="w-full h-full overflow-visible"
        :viewBox="`0 0 ${VB_W} ${VB_H}`"
        preserveAspectRatio="none"
        role="img"
        :aria-label="`${label} chart`"
      >
        <path :d="areaPath" fill="currentColor" class="text-primary-500/15" />
        <polyline
          :points="linePoints"
          fill="none"
          stroke="currentColor"
          class="text-primary-500"
          stroke-width="1"
          vector-effect="non-scaling-stroke"
          stroke-linejoin="round"
          stroke-linecap="round"
        />
      </svg>
    </div>
  </div>
</template>
