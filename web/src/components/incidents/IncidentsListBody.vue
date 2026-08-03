<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useIncidentStore } from '@/stores/incidentStore'
import { timeAgo } from '@/libs/date-time.helper'
import type { Incident } from '@/types'

interface Props {
  filter?: { resource_id?: string }
  limit?: number
}
const props = withDefaults(defineProps<Props>(), { filter: () => ({}), limit: 50 })

const emit = defineEmits<{ 'row-click': [Incident] }>()

const incidentStore = useIncidentStore()
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  try {
    await incidentStore.fetchIncidents({
      monitor_id: props.filter.resource_id,
      per_page: props.limit,
    })
  } finally {
    loading.value = false
  }
})

const filtered = computed<Incident[]>(() => {
  let out = incidentStore.incidents
  if (props.filter.resource_id) {
    out = out.filter((i) => i.resource_id === props.filter.resource_id)
  }
  return out
})

function statusOf(i: Incident) {
  return i.resolved_at
    ? { label: 'Resolved', color: '#047857', bg: '#ECFDF5' }
    : { label: 'Active', color: '#B91C1C', bg: '#FEF2F2' }
}

defineExpose({ filtered, loading })
</script>

<template>
  <div>
    <div v-if="loading" class="px-6 py-8 text-center text-sm text-muted">Loading…</div>
    <UEmpty
      v-else-if="filtered.length === 0"
      variant="naked"
      icon="i-lucide-shield-check"
      title="No incidents for this resource"
      description="When checks fail, incidents will appear here."
    />
    <div v-else class="divide-y divide-slate-100">
      <button
        v-for="i in filtered"
        :key="i.id"
        type="button"
        class="w-full grid grid-cols-[90px_1fr_120px_140px] gap-3 items-center px-4 py-3 text-left text-sm hover:bg-muted"
        @click="emit('row-click', i)"
      >
        <span
          class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-semibold w-fit"
          :style="{ backgroundColor: statusOf(i).bg, color: statusOf(i).color }"
        >
          {{ statusOf(i).label }}
        </span>
        <span class="text-default truncate">{{ i.cause || i.reason || '—' }}</span>
        <span class="text-xs font-mono text-muted">{{ timeAgo(i.started_at) }}</span>
        <UIcon name="i-lucide-chevron-right" class="size-4 text-dimmed justify-self-end" />
      </button>
    </div>
  </div>
</template>
