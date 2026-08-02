<script setup lang="ts">
/**
 * WHOIS tool.
 * Domain registration lookup + "Create monitor" CTA to track expiry.
 */
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@nuxt/ui/composables/useToast'
import { whoisSchema } from '@/schemas/toolbox-whois.schema'
import { whoisLookup } from '@/services/toolboxService'
import type { WhoisResponse } from '@/types/toolbox'

const toast = useToast()
const router = useRouter()

const state = reactive<{ domain: string }>({ domain: '' })

const loading = ref(false)
const result = ref<WhoisResponse | null>(null)
let controller: AbortController | null = null

async function run() {
  loading.value = true
  controller = new AbortController()
  try {
    result.value = await whoisLookup({ domain: state.domain.trim() }, controller.signal)
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') {
      result.value = null
      toast.add({ title: 'WHOIS lookup failed', description: (e as Error)?.message, color: 'error' })
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

function createMonitor() {
  router.push({
    path: '/resources',
    query: { create: '1', type: 'http', target: `https://${state.domain.trim()}` },
  })
}
</script>

<template>
  <div class="flex flex-col lg:flex-row gap-6">
    <div class="w-full lg:w-[380px] shrink-0 flex flex-col gap-4">
      <UForm :schema="whoisSchema" :state="state" class="flex flex-col gap-4" @submit="run">
        <UFormField label="Domain" name="domain">
          <UInput v-model="state.domain" placeholder="example.com" class="w-full" />
        </UFormField>
        <div class="flex gap-2">
          <UButton type="submit" :loading="loading" icon="i-lucide-play">Run</UButton>
          <UButton v-if="loading" color="neutral" variant="subtle" @click="cancel">Cancel</UButton>
        </div>
      </UForm>
    </div>

    <div class="flex-1 min-w-0">
      <div v-if="result" class="flex flex-col gap-4">
        <!-- Domain card -->
        <div class="rounded-lg border border-default p-4 flex flex-col gap-3 text-sm">
          <div class="flex items-center justify-between">
            <span class="font-medium text-highlighted">{{ state.domain }}</span>
            <UBadge v-if="result.days_to_expiry > 0" color="success" variant="subtle">
              Active · {{ result.days_to_expiry }}d remaining
            </UBadge>
          </div>
          <dl class="grid grid-cols-[120px_1fr] gap-y-1">
            <dt class="text-muted">Registrar</dt><dd>{{ result.registrar || '—' }}</dd>
            <dt class="text-muted">Registered</dt><dd>{{ result.registered_at || '—' }}</dd>
            <dt class="text-muted">Updated</dt><dd>{{ result.updated_at || '—' }}</dd>
            <dt class="text-muted">Expires</dt><dd>{{ result.expires_at || '—' }}</dd>
            <dt class="text-muted">Status</dt><dd class="font-mono break-all">{{ result.status.join(', ') || '—' }}</dd>
            <dt class="text-muted">Privacy</dt><dd>{{ result.privacy ? 'Enabled' : 'Disabled' }}</dd>
            <dt class="text-muted">DNSSEC</dt><dd>{{ result.dnssec ? 'Signed' : 'Unsigned' }}</dd>
          </dl>
        </div>

        <!-- Nameservers -->
        <div class="rounded-lg border border-default p-4 flex flex-col gap-2 text-sm">
          <div class="font-medium text-highlighted">Nameservers</div>
          <ul class="font-mono text-muted">
            <li v-for="ns in result.nameservers" :key="ns">{{ ns }}</li>
            <li v-if="!result.nameservers.length">—</li>
          </ul>
        </div>

        <UAlert
          icon="i-lucide-calendar-clock"
          color="primary"
          variant="subtle"
          title="Track this domain's expiry automatically"
          description="Set up a monitor to be alerted before it expires."
        >
          <template #actions>
            <UButton size="xs" color="primary" @click="createMonitor">Create monitor</UButton>
          </template>
        </UAlert>
      </div>
      <div v-else-if="!loading" class="flex items-center justify-center h-40 text-muted text-sm">
        Enter a domain to look up its registration.
      </div>
    </div>
  </div>
</template>
