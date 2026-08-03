<script setup lang="ts">
import { computed } from 'vue'
import type { Incident } from '@/types'

interface Props {
  incident: Incident
}
const props = defineProps<Props>()

const emit = defineEmits<{
  action: [{ kind: 'back' }]
}>()

const isResolved = computed(() => !!props.incident.resolved_at)
const incidentLabel = computed(() => `INC-${props.incident.id.slice(-8).toUpperCase()}`)

const statusStyle = computed(() =>
  isResolved.value
    ? { bg: '#ECFDF5', border: '#6EE7B7', color: '#047857', label: 'Resolved' }
    : { bg: '#FEF2F2', border: '#FCA5A5', color: '#B91C1C', label: 'Active' },
)

const resource = computed(() => props.incident.resource)
const resourceStatusColor = computed(() => {
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

function formatDate(iso?: string | null) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}
</script>

<template>
  <div class="bg-default rounded-lg border border-default p-5 mb-5">
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div class="flex items-center gap-2.5 flex-wrap">
        <span
          class="inline-flex items-center gap-2 px-2.5 py-0.5 rounded-full border"
          :style="{ backgroundColor: statusStyle.bg, borderColor: statusStyle.border }"
        >
          <span class="size-1.5 rounded-full" :style="{ backgroundColor: statusStyle.color }" />
          <span class="text-xs font-semibold" :style="{ color: statusStyle.color }">
            {{ statusStyle.label }}
          </span>
        </span>
        <span class="text-xs font-mono text-muted">{{ incidentLabel }}</span>
      </div>
      <div class="flex items-center gap-2">
        <UButton
          color="neutral"
          variant="outline"
          size="sm"
          icon="i-lucide-arrow-left"
          @click="emit('action', { kind: 'back' })"
        >
          Back
        </UButton>
      </div>
    </div>

    <div class="flex items-center gap-3 mb-3">
      <span class="size-3 rounded-full" :style="{ backgroundColor: resourceStatusColor }" />
      <div class="flex flex-col">
        <h1 class="text-[22px] font-semibold font-mono text-highlighted leading-tight">
          {{ resource?.name ?? incident.resource_id }}
        </h1>
        <p class="text-sm text-muted">
          {{ incident.cause || incident.reason || 'Incident in progress' }}
        </p>
      </div>
    </div>

    <dl class="flex flex-wrap items-center gap-x-6 gap-y-1.5 text-xs">
      <div>
        <dt class="inline text-muted">Started:</dt>
        <dd class="inline ml-1 text-default font-mono">{{ formatDate(incident.started_at) }}</dd>
      </div>
      <div v-if="isResolved">
        <dt class="inline text-muted">Resolved:</dt>
        <dd class="inline ml-1 text-default font-mono">{{ formatDate(incident.resolved_at) }}</dd>
      </div>
    </dl>
  </div>
</template>
