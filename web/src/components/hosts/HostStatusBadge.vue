<script setup lang="ts">
/**
 * Host online/offline badge . Green dot + "online" when the agent is
 * connected; muted dot + relative last-seen when offline.
 */
import { computed } from 'vue'
import { timeAgo } from '@/libs/date-time.helper'

interface Props {
  online: boolean
  lastSeenAt: string | null
}

const props = defineProps<Props>()

const label = computed(() => {
  if (props.online) return 'Online'
  if (props.lastSeenAt) return `Offline · ${timeAgo(props.lastSeenAt)}`
  return 'Never seen'
})

const dotClass = computed(() => (props.online ? 'bg-success' : 'bg-dimmed'))
const textClass = computed(() => (props.online ? 'text-success' : 'text-muted'))
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-xs" :class="textClass">
    <span class="size-2 rounded-full" :class="dotClass" />
    {{ label }}
  </span>
</template>
