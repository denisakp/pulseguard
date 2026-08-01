<script setup lang="ts">
/**
 * HostServicesList — the monitors ("hosted services") linked
 * to a host. Each row shows the monitor status + last-check time and links to
 * the monitor detail route (`ResourceDetail` → /resources/:id). Empty state
 * when the host has no linked monitors.
 */
import { timeAgo } from '@/libs/date-time.helper'

export interface HostLinkedMonitor {
  id: string
  name: string
  type: string
  status: string
  lastCheckedAt: string | null
}

interface Props {
  monitors: HostLinkedMonitor[]
}

defineProps<Props>()

type Dot = 'up' | 'down' | 'warning' | 'unknown'

function statusDot(status: string): Dot {
  switch (status) {
    case 'up':
      return 'up'
    case 'down':
    case 'error':
      return 'down'
    case 'flapping':
      return 'warning'
    default:
      return 'unknown'
  }
}

const DOT_CLASS: Record<Dot, string> = {
  up: 'bg-success',
  down: 'bg-error',
  warning: 'bg-warning',
  unknown: 'bg-dimmed',
}
</script>

<template>
  <div class="rounded-lg border border-default bg-default p-5" data-testid="host-services-list">
    <h3 class="text-base font-semibold text-highlighted mb-4">Hosted services</h3>

    <div
      v-if="monitors.length === 0"
      class="text-sm text-muted text-center py-8"
      data-testid="services-empty"
    >
      No monitors are linked to this host yet.
    </div>

    <ul v-else class="divide-y divide-default">
      <li v-for="m in monitors" :key="m.id">
        <RouterLink
          :to="{ name: 'ResourceDetail', params: { id: m.id } }"
          class="flex items-center gap-3 py-2.5 px-1 rounded-md hover:bg-elevated transition-colors"
          data-testid="service-row"
        >
          <span class="size-2 rounded-full shrink-0" :class="DOT_CLASS[statusDot(m.status)]" />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-default truncate">{{ m.name }}</p>
            <p class="text-xs text-muted uppercase tracking-wide">{{ m.type }}</p>
          </div>
          <span class="text-xs text-muted shrink-0">
            {{ m.lastCheckedAt ? timeAgo(m.lastCheckedAt) : 'never' }}
          </span>
        </RouterLink>
      </li>
    </ul>
  </div>
</template>
