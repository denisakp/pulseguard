<script setup lang="ts">
/**
 * Port Scanner tool.
 * Target must be a registered monitor host (backend gates with 403). Presets
 * auto-populate the ports field; results show open/closed/filtered per port.
 */
import { reactive, ref, computed, watch } from 'vue'
import { useToast } from '@nuxt/ui/composables/useToast'
import { portScanSchema, portPresets, portPresetValues } from '@/schemas/toolbox-port.schema'
import { portScan } from '@/services/toolboxService'
import type { PortResult, PortPreset } from '@/types/toolbox'

const toast = useToast()

const state = reactive<{ target: string; preset: PortPreset; portsText: string; timeout_ms: number }>({
  target: '',
  preset: 'common',
  portsText: portPresetValues.common.join(', '),
  timeout_ms: 1000,
})

const presetItems = portPresets.map((p) => ({ label: p.charAt(0).toUpperCase() + p.slice(1), value: p }))

// Selecting a preset (other than custom) repopulates the ports field.
watch(
  () => state.preset,
  (p) => {
    if (p !== 'custom') state.portsText = portPresetValues[p].join(', ')
  },
)

const loading = ref(false)
const results = ref<PortResult[]>([])
const openCount = ref(0)
const scannedCount = ref(0)
const ran = ref(false)
let controller: AbortController | null = null

const summary = computed(() => (ran.value ? `${openCount.value} open / ${scannedCount.value} scanned` : ''))

function parsePorts(text: string): number[] {
  return text
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => Number(s))
}

function statusColor(status: string): 'success' | 'neutral' | 'warning' {
  if (status === 'open') return 'success'
  if (status === 'filtered') return 'warning'
  return 'neutral'
}

async function run() {
  const payload = {
    target: state.target.trim(),
    preset: state.preset,
    ports: parsePorts(state.portsText),
    timeout_ms: state.timeout_ms,
  }
  const parsed = portScanSchema.safeParse(payload)
  if (!parsed.success) {
    toast.add({ title: 'Invalid input', description: parsed.error.issues[0]?.message, color: 'error' })
    return
  }
  loading.value = true
  ran.value = true
  controller = new AbortController()
  try {
    const res = await portScan(
      { target: payload.target, ports: payload.ports, preset: payload.preset, timeout_ms: payload.timeout_ms },
      controller.signal,
    )
    results.value = res.results
    openCount.value = res.open_count
    scannedCount.value = res.scanned_count
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') {
      toast.add({ title: 'Port scan failed', description: (e as Error)?.message, color: 'error' })
    }
  } finally {
    loading.value = false
    controller = null
  }
}

function cancel() {
  controller?.abort()
  loading.value = false
}
</script>

<template>
  <div class="flex flex-col lg:flex-row gap-6">
    <div class="w-full lg:w-[380px] shrink-0 flex flex-col gap-4">
      <UFormField label="Target host" name="target">
        <UInput v-model="state.target" placeholder="db-prod-01.internal" class="w-full font-mono" />
      </UFormField>

      <UFormField label="Preset" name="preset">
        <USelect v-model="state.preset" :items="presetItems" class="w-full" />
      </UFormField>

      <UFormField label="Ports" name="ports" hint="comma or space separated, max 100">
        <UTextarea v-model="state.portsText" :rows="3" class="w-full font-mono" />
      </UFormField>

      <UFormField label="Timeout (ms)" name="timeout_ms">
        <UInput v-model.number="state.timeout_ms" type="number" :min="100" :max="2000" class="w-full" />
      </UFormField>

      <div class="flex gap-2">
        <UButton :loading="loading" icon="i-lucide-play" @click="run">Run</UButton>
        <UButton v-if="loading" color="neutral" variant="subtle" @click="cancel">Cancel</UButton>
      </div>

      <UAlert
        icon="i-lucide-shield"
        color="neutral"
        variant="subtle"
        title="Registered hosts only"
        description="Scans are limited to hosts already monitored, rate-limited, and audited."
      />
    </div>

    <div class="flex-1 min-w-0">
      <div v-if="ran && !loading" class="flex flex-col gap-3">
        <UBadge color="primary" variant="subtle">{{ summary }}</UBadge>
        <div v-if="results.length" class="overflow-x-auto rounded-lg border border-default">
          <table class="w-full text-sm">
            <thead class="bg-elevated text-muted">
              <tr>
                <th class="text-left font-medium px-3 py-2">Port</th>
                <th class="text-left font-medium px-3 py-2">Service</th>
                <th class="text-left font-medium px-3 py-2">Status</th>
                <th class="text-left font-medium px-3 py-2">Banner</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(r, i) in results" :key="i" class="border-t border-default">
                <td class="px-3 py-2 font-mono">{{ r.port }}</td>
                <td class="px-3 py-2 text-muted">{{ r.service || '—' }}</td>
                <td class="px-3 py-2">
                  <UBadge :color="statusColor(r.status)" variant="subtle" size="sm">{{ r.status }}</UBadge>
                </td>
                <td class="px-3 py-2 font-mono text-xs text-muted break-all">{{ r.banner || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div v-else-if="!ran" class="flex items-center justify-center h-40 text-muted text-sm">
        Enter a registered host and run a scan.
      </div>
    </div>
  </div>
</template>
