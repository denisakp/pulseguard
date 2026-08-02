<script setup lang="ts">
/**
 * Prometheus Metrics documentation page.
 * Static reference for the existing GET /metrics endpoint — no live stats
 * (out of scope, FR-022). Describes endpoint, auth, catalog, scrape config.
 */
import { computed, ref } from 'vue'
import { useToast } from '@nuxt/ui/composables/useToast'
import { GRAFANA_DASHBOARD, ALERT_RULES_YAML } from './integrations'
import integrationsService from '@/services/integrationsService'

const toast = useToast()

// Derive the endpoint from the API base URL (strip a trailing /api segment).
const endpointUrl = computed(() => {
  const base = (import.meta.env.VITE_API_BASE_URL as string) || '/api'
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  const root = base.replace(/\/api\/?$/, '').replace(/\/$/, '')
  const prefix = root.startsWith('http') ? root : `${origin}${root}`
  return `${prefix}/metrics`
})

const tokenRevealed = ref(false)

const catalog = [
  { name: 'ogoune_resource_up', type: 'gauge', description: 'Whether the resource is currently up (1=up, 0=down).' },
  { name: 'ogoune_resource_status', type: 'gauge', description: 'Current status (0=unknown,1=up,2=down,3=paused).' },
  { name: 'ogoune_incidents_total', type: 'counter', description: 'All-time total incidents for the resource.' },
  { name: 'ogoune_incidents_active', type: 'gauge', description: 'Currently open incidents for the resource.' },
  { name: 'ogoune_uptime_ratio', type: 'gauge', description: 'Uptime percentage (0–100) over a time window.' },
]

const scrapeConfig = computed(
  () => `scrape_configs:
  - job_name: ogoune
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials: <METRICS_TOKEN>
    static_configs:
      - targets: ['${endpointUrl.value.replace(/^https?:\/\//, '')}']`,
)

const curlExample = computed(
  () => `$ curl -H "Authorization: Bearer $METRICS_TOKEN" ${endpointUrl.value}

# HELP ogoune_resource_up Whether the resource is currently up (1=up, 0=down).
# TYPE ogoune_resource_up gauge
ogoune_resource_up{resource_id="01H...",name="api"} 1
# TYPE ogoune_uptime_ratio gauge
ogoune_uptime_ratio{resource_id="01H...",window="24h"} 99.8`,
)

function typeColor(type: string): 'primary' | 'warning' | 'success' {
  if (type === 'counter') return 'warning'
  if (type === 'histogram') return 'success'
  return 'primary'
}

async function copy(value: string, label = 'Copied') {
  try {
    await navigator.clipboard.writeText(value)
    toast.add({ title: label, color: 'success' })
  } catch {
    toast.add({ title: 'Copy failed', color: 'error' })
  }
}

function download(filename: string, content: string, mime: string) {
  const url = URL.createObjectURL(new Blob([content], { type: mime }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
  toast.add({ title: `${filename} downloaded`, color: 'success' })
}

// Both downloads try the config-derived endpoint first; on any error they fall
// back to the bundled static template so the button is never dead.
async function importGrafanaDashboard() {
  try {
    const dash = await integrationsService.fetchGrafanaDashboard()
    download('ogoune-grafana-dashboard.json', JSON.stringify(dash, null, 2), 'application/json')
  } catch {
    download('ogoune-grafana-dashboard.json', JSON.stringify(GRAFANA_DASHBOARD, null, 2), 'application/json')
    toast.add({ title: 'Used the generic template (config-derived unavailable)', color: 'warning' })
  }
}

async function downloadAlertRules() {
  try {
    const rules = await integrationsService.fetchAlertRules()
    download('ogoune-alerts.rules.yml', rules, 'text/yaml')
  } catch {
    download('ogoune-alerts.rules.yml', ALERT_RULES_YAML, 'text/yaml')
    toast.add({ title: 'Used the generic template (config-derived unavailable)', color: 'warning' })
  }
}
</script>

<template>
  <div class="flex flex-col gap-6 w-full min-h-full bg-default text-default">
    <header class="flex flex-col gap-1">
      <h1 class="text-2xl font-bold text-highlighted">Prometheus Metrics</h1>
      <p class="text-sm text-muted">Scrape Ogoune metrics into your monitoring stack.</p>
    </header>

    <div class="flex flex-col lg:flex-row gap-6">
      <div class="flex-1 min-w-0 flex flex-col gap-6">
        <!-- Hero -->
        <div class="rounded-lg border border-default p-4 flex flex-col gap-3">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-highlighted">Endpoint</span>
            <UBadge color="success" variant="subtle" size="sm">Enabled via ENABLE_METRICS</UBadge>
          </div>
          <div class="flex items-center gap-2">
            <code class="flex-1 px-3 py-2 rounded-md bg-elevated font-mono text-sm break-all">{{ endpointUrl }}</code>
            <UButton icon="i-lucide-copy" color="neutral" variant="subtle" size="sm" @click="copy(endpointUrl)" />
          </div>
          <div class="flex items-center gap-2 text-sm">
            <span class="text-muted">Bearer token</span>
            <code class="font-mono">{{ tokenRevealed ? '$METRICS_TOKEN' : '••••••••' }}</code>
            <UButton
              :icon="tokenRevealed ? 'i-lucide-eye-off' : 'i-lucide-eye'"
              color="neutral"
              variant="ghost"
              size="xs"
              @click="tokenRevealed = !tokenRevealed"
            />
            <span class="text-xs text-muted">(set server-side via METRICS_TOKEN)</span>
          </div>
        </div>

        <!-- Catalog -->
        <div class="rounded-lg border border-default overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-elevated text-muted">
              <tr>
                <th class="text-left font-medium px-3 py-2">Metric</th>
                <th class="text-left font-medium px-3 py-2">Type</th>
                <th class="text-left font-medium px-3 py-2">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in catalog" :key="m.name" class="border-t border-default">
                <td class="px-3 py-2 font-mono">{{ m.name }}</td>
                <td class="px-3 py-2">
                  <UBadge :color="typeColor(m.type)" variant="subtle" size="sm">{{ m.type }}</UBadge>
                </td>
                <td class="px-3 py-2 text-muted">{{ m.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Curl sandbox -->
        <div class="rounded-lg border border-default overflow-hidden">
          <div class="flex items-center justify-between px-3 py-2 bg-elevated">
            <span class="text-xs font-medium text-muted uppercase tracking-wide">Example</span>
            <UButton icon="i-lucide-copy" color="neutral" variant="ghost" size="xs" @click="copy(curlExample)" />
          </div>
          <pre class="px-3 py-3 bg-gray-950 text-green-400 text-xs font-mono overflow-x-auto">{{ curlExample }}</pre>
        </div>
      </div>

      <!-- Right sidebar -->
      <div class="w-full lg:w-80 shrink-0 flex flex-col gap-6">
        <div class="rounded-lg border border-default overflow-hidden">
          <div class="flex items-center justify-between px-3 py-2 bg-elevated">
            <span class="text-xs font-medium text-muted uppercase tracking-wide">Scrape config</span>
            <UButton icon="i-lucide-copy" color="neutral" variant="ghost" size="xs" @click="copy(scrapeConfig, 'Config copied')" />
          </div>
          <pre class="px-3 py-3 text-xs font-mono overflow-x-auto">{{ scrapeConfig }}</pre>
        </div>

        <div class="rounded-lg border border-default p-4 flex flex-col gap-2 text-sm">
          <div class="font-medium text-highlighted">Integrations</div>
          <p class="text-xs text-muted" data-testid="scrape-hint">
            Downloads are generated from your Ogoune config. They only show data once a
            Prometheus scrape of <code>/metrics</code> is active.
          </p>
          <div class="flex items-center justify-between">
            <span>Grafana</span>
            <UButton
              size="xs"
              variant="subtle"
              color="neutral"
              icon="i-lucide-download"
              data-testid="grafana-download"
              @click="importGrafanaDashboard"
            >
              Download dashboard
            </UButton>
          </div>
          <div class="flex items-center justify-between">
            <span>Alertmanager</span>
            <UButton
              size="xs"
              variant="subtle"
              color="neutral"
              icon="i-lucide-download"
              data-testid="alertmanager-examples"
              @click="downloadAlertRules"
            >
              Download rules
            </UButton>
          </div>
          <div class="flex items-center justify-between text-muted">
            <span>OpenTelemetry</span><UBadge color="neutral" variant="subtle" size="sm">Soon</UBadge>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
