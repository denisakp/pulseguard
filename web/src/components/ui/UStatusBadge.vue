<script setup lang="ts">
/**
 * Presentational status badge — STRICT isolation.
 * No contextual composables (auth, router, licence, color-mode).
 */
import { computed } from 'vue'

type Status = 'up' | 'down' | 'warning' | 'maintenance' | 'unknown'
type Size = 'sm' | 'md' | 'lg'

interface Props {
  status: Status
  size?: Size
  dot?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  dot: false,
})

const palette: Record<Status, { dot: string; bg: string; text: string; label: string }> = {
  up: {
    dot: 'bg-emerald-500',
    bg: 'bg-emerald-50 dark:bg-emerald-950/30',
    text: 'text-emerald-700 dark:text-emerald-300',
    label: 'Operational',
  },
  down: {
    dot: 'bg-red-500',
    bg: 'bg-red-50 dark:bg-red-950/30',
    text: 'text-red-700 dark:text-red-300',
    label: 'Down',
  },
  warning: {
    dot: 'bg-amber-500',
    bg: 'bg-amber-50 dark:bg-amber-950/30',
    text: 'text-amber-700 dark:text-amber-300',
    label: 'Warning',
  },
  maintenance: {
    dot: 'bg-sky-500',
    bg: 'bg-sky-50 dark:bg-sky-950/30',
    text: 'text-sky-700 dark:text-sky-300',
    label: 'Maintenance',
  },
  unknown: {
    dot: 'bg-slate-400',
    bg: 'bg-slate-100 dark:bg-slate-800',
    text: 'text-slate-600 dark:text-slate-400',
    label: 'Unknown',
  },
}

const sizeClass = computed(
  () =>
    ({
      sm: 'text-xs px-1.5 py-0.5 gap-1',
      md: 'text-sm px-2 py-1 gap-1.5',
      lg: 'text-base px-2.5 py-1 gap-2',
    })[props.size],
)

const config = computed(() => palette[props.status])
</script>

<template>
  <span
    :class="['inline-flex items-center rounded-md font-medium', sizeClass, config.bg, config.text]"
    :data-status="status"
  >
    <span v-if="dot" :class="['size-2 rounded-full', config.dot]" />
    {{ config.label }}
  </span>
</template>
