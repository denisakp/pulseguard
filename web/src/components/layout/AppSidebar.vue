<script setup lang="ts">
/**
 * App sidebar — Pencil `wHAmm`.
 * Four sections (MONITOR / REPORT / TOOLS / SETTINGS) + footer status pill.
 * Contract: specs/055-slice-shared-components/contracts/app-layout.md
 */
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'

import { useResourceStore } from '@/stores/resourceStore'

interface NavItem {
  label: string
  to: string
  icon: string
}

// Hosts is always visible so operators can discover it and reach the
// onboarding/register flow even before any host exists (spec 081 follow-up).
const monitor: NavItem[] = [
  { label: 'Overview', to: '/overview', icon: 'i-lucide-gauge' },
  { label: 'Resources', to: '/resources', icon: 'i-lucide-radar' },
  { label: 'Hosts', to: '/hosts', icon: 'i-lucide-server' },
  { label: 'Incidents', to: '/incidents', icon: 'i-lucide-zap' },
  { label: 'Maintenance', to: '/maintenance', icon: 'i-lucide-wrench' },
]

const report: NavItem[] = [
  { label: 'Reports', to: '/reports', icon: 'i-lucide-file-text' },
  { label: 'Dashboards', to: '/dashboards', icon: 'i-lucide-layout-grid' },
]

const tools: NavItem[] = [
  { label: 'Toolbox', to: '/toolbox', icon: 'i-lucide-wrench' },
  { label: 'Metrics', to: '/metrics', icon: 'i-lucide-line-chart' },
]

const settings: NavItem[] = [
  { label: 'Notifications', to: '/notifications', icon: 'i-lucide-bell' },
  { label: 'Escalation', to: '/escalation', icon: 'i-lucide-siren' },
  { label: 'API Keys', to: '/api-keys', icon: 'i-lucide-key-round' },
  { label: 'Preferences', to: '/settings', icon: 'i-lucide-settings' },
]

const route = useRoute()

// Footer status pill — reflect the real fleet state instead of a hardcoded
// "operational". Fetch resources once if the store is still empty so the pill
// is accurate on any page (the sidebar is mounted app-wide).
const resourceStore = useResourceStore()
onMounted(() => {
  if (resourceStore.resources.length === 0 && !resourceStore.fetchLoading) {
    void resourceStore.loadResources()
  }
})
const hasResources = computed(() => resourceStore.resources.length > 0)
const downCount = computed(() => resourceStore.resources.filter((r) => r.status === 'down').length)
const footerLabel = computed(() => {
  if (!hasResources.value) return 'No resources yet'
  if (downCount.value > 0) return `${downCount.value} resource${downCount.value > 1 ? 's' : ''} down`
  return 'All systems operational'
})
const footerTextClass = computed(() => {
  if (!hasResources.value) return 'text-muted'
  return downCount.value > 0 ? 'text-warning' : 'text-success'
})
const footerDotClass = computed(() => {
  if (!hasResources.value) return 'bg-dimmed'
  return downCount.value > 0 ? 'bg-warning' : 'bg-success'
})

function linkClass(to: string): string {
  const active = route.path === to || route.path.startsWith(`${to}/`)
  return active
    ? 'bg-elevated text-default font-medium'
    : 'text-muted hover:bg-elevated hover:text-default'
}
</script>

<template>
  <aside
    class="hidden lg:flex w-60 shrink-0 flex-col bg-default border-r border-default h-screen sticky top-0"
  >
    <div class="p-4">
      <RouterLink to="/" class="flex items-center gap-2">
        <span class="inline-flex size-7 items-center justify-center rounded-lg bg-primary-500">
          <UIcon name="i-lucide-activity" class="size-4 text-white" />
        </span>
        <span class="text-lg font-bold text-default">Ogoune</span>
      </RouterLink>
    </div>

    <nav class="flex-1 overflow-y-auto px-3 pb-3 space-y-6">
      <section>
        <div class="px-2 py-1 text-xs font-medium text-muted uppercase tracking-wide">Monitor</div>
        <div class="space-y-1">
          <RouterLink
            v-for="item in monitor"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors"
            :class="linkClass(item.to)"
          >
            <UIcon :name="item.icon" class="size-4" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </div>
      </section>
      <section>
        <div class="px-2 py-1 text-xs font-medium text-muted uppercase tracking-wide">Report</div>
        <div class="space-y-1">
          <RouterLink
            v-for="item in report"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors"
            :class="linkClass(item.to)"
          >
            <UIcon :name="item.icon" class="size-4" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </div>
      </section>
      <section>
        <div class="px-2 py-1 text-xs font-medium text-muted uppercase tracking-wide">Tools</div>
        <div class="space-y-1">
          <RouterLink
            v-for="item in tools"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors"
            :class="linkClass(item.to)"
          >
            <UIcon :name="item.icon" class="size-4" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </div>
      </section>
      <section>
        <div class="px-2 py-1 text-xs font-medium text-muted uppercase tracking-wide">Settings</div>
        <div class="space-y-1">
          <RouterLink
            v-for="item in settings"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors"
            :class="linkClass(item.to)"
          >
            <UIcon :name="item.icon" class="size-4" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </div>
      </section>
    </nav>

    <div class="p-3 border-t border-default">
      <ULink
        to="/status.html"
        target="_blank"
        rel="noopener"
        class="flex items-center gap-2 px-2 py-1.5 rounded-md text-xs hover:bg-elevated transition-colors"
        :class="footerTextClass"
      >
        <span class="size-2 rounded-full" :class="footerDotClass" />
        {{ footerLabel }}
      </ULink>
    </div>
  </aside>
</template>
