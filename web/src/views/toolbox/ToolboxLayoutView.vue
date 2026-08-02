<script setup lang="ts">
/**
 * Toolbox shell — route-synced tabs over <RouterView/>.
 * Four one-shot network tools: DNS / Port / SSL / WHOIS.
 * Pattern mirrors SettingsLayoutView (RouterLink tab bar + useRoute active state).
 */
import { RouterLink, RouterView, useRoute } from 'vue-router'

interface Tab {
  label: string
  to: string
  icon: string
}

const tabs: Tab[] = [
  { label: 'DNS Lookup', to: '/toolbox/dns', icon: 'i-lucide-globe' },
  { label: 'Port Scanner', to: '/toolbox/port', icon: 'i-lucide-radar' },
  { label: 'SSL Checker', to: '/toolbox/ssl', icon: 'i-lucide-lock' },
  { label: 'WHOIS', to: '/toolbox/whois', icon: 'i-lucide-file-search' },
]

const route = useRoute()
const isActive = (to: string) => route.path === to || route.path.startsWith(`${to}/`)
</script>

<template>
  <div class="flex flex-col gap-6 w-full min-h-full bg-default text-default">
    <header class="flex flex-col gap-1">
      <h1 class="text-2xl font-bold text-highlighted">Toolbox</h1>
      <p class="text-sm text-muted">Run one-off network checks</p>
    </header>

    <nav class="flex gap-1 border-b border-default">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.to"
        :to="tab.to"
        class="flex items-center gap-2 px-3 py-2 text-sm border-b-2 -mb-px transition-colors"
        :class="
          isActive(tab.to)
            ? 'border-primary-500 text-highlighted font-medium'
            : 'border-transparent text-muted hover:text-default'
        "
      >
        <UIcon :name="tab.icon" class="size-4" />
        <span>{{ tab.label }}</span>
      </RouterLink>
    </nav>

    <RouterView />
  </div>
</template>
